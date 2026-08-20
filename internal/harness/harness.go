package harness

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

// PollInterval 是就绪轮询的时间间隔。
const PollInterval = 400 * time.Millisecond

// ReadyTimeout 是就绪等待的上限。
const ReadyTimeout = 180 * time.Second

// ShutdownGrace 是优雅关闭后强杀进程组的等待时间。
const ShutdownGrace = 5 * time.Second

// State 描述 harness 子进程当前所处生命周期阶段。
type State string

const (
	StateIdle     State = "idle"     // 尚未启动
	StateStarting State = "starting" // 已拉起, 等待就绪
	StateReady    State = "ready"    // 就绪, 可访问
	StateStopping State = "stopping" // 正在关闭
	StateStopped  State = "stopped"  // 已停止(含失败态由 Error 区分)
	StateFailed   State = "failed"   // 启动失败
)

// Status 是暴露给前端的运行时快照。
type Status struct {
	State   State  `json:"state"`
	URL     string `json:"url"`
	Port    int    `json:"port"`
	LogPath string `json:"logPath"`
	Error   string `json:"error,omitempty"`
}

// Harness 管理 dsh web 子进程的完整生命周期。
type Harness struct {
	mu           sync.Mutex
	dshHome      string // 与系统 DSH 共用的数据根目录
	logFile      string // harness 日志文件路径
	port         int    // 监听端口
	state        State
	lastError    string // 最近一次失败原因
	cmd          *exec.Cmd
	done         <-chan struct{} // cmd 的唯一 Wait 完成信号；可被多个生命周期步骤观察
	processGroup int             // 进程组 id(= leader pid), 用于退出时兜底回收整棵进程树
}

// New 创建一个 Harness 实例。
func New(dshHome, logFile string, port int) *Harness {
	return &Harness{
		dshHome: dshHome,
		logFile: logFile,
		port:    port,
		state:   StateIdle,
	}
}

// URL 返回就绪后可访问的本地地址。
func (h *Harness) URL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", h.port)
}

// DshHome 返回与系统 DSH 共用的数据根目录。
func (h *Harness) DshHome() string {
	return h.dshHome
}

// CurrentStatus 返回暴露给前端的运行时快照。
func (h *Harness) CurrentStatus() Status {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.currentStatus()
}

func (h *Harness) currentStatus() Status {
	return Status{
		State:   h.state,
		URL:     h.URL(),
		Port:    h.port,
		LogPath: h.logFile,
		Error:   h.lastError,
	}
}

// Start 拉起 dsh web 子进程并等待其就绪, 期间子进程提前退出或超时则失败。
func (h *Harness) Start() error {
	h.mu.Lock()
	if h.state == StateStarting || h.state == StateReady {
		h.mu.Unlock()
		return nil
	}
	h.state = StateStarting
	h.lastError = ""
	h.mu.Unlock()

	// 复用探测: 端口上已有服务则直接进入就绪态, 不重复拉起。
	if servicePresent(h.port) {
		h.mu.Lock()
		h.state = StateReady
		h.mu.Unlock()
		return nil
	}

	logFile, err := os.OpenFile(h.logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		h.fail(fmt.Errorf("打开日志文件失败: %w", err))
		return err
	}
	defer logFile.Close()

	cmd, err := newDshCommand("web",
		"--no-open", "--host", "127.0.0.1", "--port", fmt.Sprintf("%d", h.port))
	if err != nil {
		h.fail(fmt.Errorf("未检测到 dsh 命令, 请先安装 Harness: %w", err))
		return err
	}
	cmd.Dir = h.dshHome
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append(os.Environ(), "DSH_HOME="+h.dshHome)
	// 独立进程组, 便于退出时兜底回收整棵进程树。
	setProcessGroup(cmd)

	if err := cmd.Start(); err != nil {
		h.fail(fmt.Errorf("启动 harness 子进程失败: %w", err))
		return err
	}
	// 子进程退出会写 done, 配合就绪轮询捕获提前退出。
	done := make(chan struct{})
	var waitErr error
	h.mu.Lock()
	h.cmd = cmd
	h.done = done
	h.processGroup = cmd.Process.Pid
	h.mu.Unlock()
	go func() {
		waitErr = cmd.Wait()
		close(done)
	}()

	if err := h.waitReady(done, &waitErr); err != nil {
		h.fail(err)
		// 清理已拉起的进程树。
		h.mu.Lock()
		pgid := h.processGroup
		h.mu.Unlock()
		if pgid != 0 {
			_ = killProcessTree(pgid)
			<-done
		}
		h.mu.Lock()
		h.cmd = nil
		h.done = nil
		h.processGroup = 0
		h.mu.Unlock()
		return err
	}
	h.mu.Lock()
	h.state = StateReady
	h.mu.Unlock()
	return nil
}

// waitReady 以固定间隔轮询 HTTP 端点, 直到就绪 / 子进程退出 / 超时。
func (h *Harness) waitReady(done <-chan struct{}, waitErr *error) error {
	timer := time.NewTimer(ReadyTimeout)
	defer timer.Stop()

	for {
		if servicePresent(h.port) {
			return nil
		}
		select {
		case <-done:
			if *waitErr == nil {
				return fmt.Errorf("harness 子进程意外退出")
			}
			return fmt.Errorf("harness 子进程退出: %v", *waitErr)
		case <-timer.C:
			return fmt.Errorf("等待 harness 就绪超时(%v)", ReadyTimeout)
		default:
		}
		time.Sleep(PollInterval)
	}
}

// Stop 优雅终止 harness 子进程, 超时后强杀整个进程组兜底回收, 防止僵尸进程。
func (h *Harness) Stop() error {
	h.mu.Lock()
	if h.cmd == nil || h.processGroup == 0 {
		if h.state != StateFailed {
			h.state = StateStopped
		}
		h.mu.Unlock()
		return nil
	}

	h.state = StateStopping
	cmd := h.cmd
	done := h.done
	pgid := h.processGroup
	h.mu.Unlock()
	// 向整个进程组发终止信号, 请求优雅关闭。
	if err := terminateProcessTree(pgid); err != nil && !errors.Is(err, ErrProcessGone) {
		h.mu.Lock()
		h.processGroup = 0
		h.cmd = nil
		h.done = nil
		h.state = StateStopped
		h.mu.Unlock()
		return err
	}

	select {
	case <-done:
		// 正常退出
	case <-time.After(ShutdownGrace):
		// 强杀整个进程组兜底。
		_ = killProcessTree(pgid)
		<-done
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	// 只清理由本次 Stop 截取的进程，避免覆盖并发产生的新实例。
	if h.cmd != cmd {
		return nil
	}
	h.cmd = nil
	h.done = nil
	h.processGroup = 0
	if h.state == StateFailed {
		// 保持失败态
	} else {
		h.state = StateStopped
	}
	return nil
}

// Restart 停止后重新启动, 用于菜单中的重启子进程动作。
func (h *Harness) Restart() error {
	_ = h.Stop()
	return h.Start()
}

// fail 置失败态并记录错误信息, lastError 由 failTo 固化。
func (h *Harness) fail(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastError = err.Error()
	h.state = StateFailed
}

// servicePresent 探测 127.0.0.1:<port> 上是否已有可用服务。
func servicePresent(port int) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	// 任意 HTTP 响应(含 4xx/5xx)均视为已有服务, 避免重复拉起。
	return true
}
