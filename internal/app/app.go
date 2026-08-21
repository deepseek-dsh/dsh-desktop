package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"dsh-desktop/internal/cfg"
	"dsh-desktop/internal/harness"
)

// StepStatus 描述启动检查单个步骤的状态。
type StepStatus string

const (
	StepPending StepStatus = "pending"
	StepRunning StepStatus = "running"
	StepDone    StepStatus = "done"
	StepFailed  StepStatus = "failed"
)

// StartupStep 是启动页右侧展示的一个启动检查步骤。
type StartupStep struct {
	ID     string     `json:"id"`
	Title  string     `json:"title"`
	Status StepStatus `json:"status"`
	Detail string     `json:"detail,omitempty"`
}

// InstallGuide 是未检测到 harness 时返回给前端的安装引导。
type InstallGuide struct {
	NodeMissing bool     `json:"nodeMissing"`
	Command     string   `json:"command"`
	Steps       []string `json:"steps"`
}

// StartupStatus 是对前端暴露的启动状态: 运行时快照 + 启动步骤清单。
type StartupStatus struct {
	harness.Status
	Theme string        `json:"theme"` // harness 外观偏好: dark/system/light
	Steps []StartupStep `json:"steps"`
	Guide *InstallGuide `json:"guide,omitempty"` // 未检测到 harness 时的安装引导
}

// stepPause 是步骤之间的最小展示时长, 保证启动页能看到每一步的进度动画。
const stepPause = 600 * time.Millisecond

// App 是 Wails 暴露给前端唯一的绑定实例。
type App struct {
	ctx            context.Context
	cfg            *cfg.Config
	harness        *harness.Harness
	startupErr     string // 目录初始化失败时记录, 供前端展示
	startupSteps   []StartupStep
	mu             sync.Mutex
	monitorOnce    sync.Once
	monitorDone    chan struct{}
	closeOnce      sync.Once
	preinstallBusy bool
	installGuide   *InstallGuide // 未检测到 harness 时的安装引导
}

// New 创建后端应用实例。
func New(c *cfg.Config) *App {
	return &App{cfg: c, monitorDone: make(chan struct{})}
}

// Startup 在窗口启动时被调用, 缓存上下文、创建目录并预置子进程对象。
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	// 启动时补齐 dsh/node 的常见安装目录到 PATH, 保证桌面启动器也能检测到。
	harness.EnsurePATH()

	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.cfg.EnsureDirs(); err != nil {
		a.startupErr = err.Error()
		return
	}
	a.startupErr = ""

	logFile := filepath.Join(a.cfg.LogDir, "harness.log")
	if a.harness == nil {
		a.harness = harness.New(a.cfg.DshHome, logFile, a.cfg.Port)
	}
}

// Shutdown 在应用退出时被调用, 优雅关闭 harness 子进程。
func (a *App) Shutdown(ctx context.Context) {
	a.closeOnce.Do(func() { close(a.monitorDone) })

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.harness != nil {
		_ = a.harness.Stop()
	}
}

// Start 按启动检查步骤推进: 检查 harness -> 启动 harness -> 安装插件市场。
// 全程异步执行, 前端通过低频 Status 轮询观察各步骤状态。
func (a *App) Start() StartupStatus {
	a.mu.Lock()
	if len(a.startupErr) != 0 {
		return StartupStatus{Status: harness.Status{State: harness.StateFailed, Error: a.startupErr}}
	}
	if a.harness == nil {
		return StartupStatus{Status: harness.Status{State: harness.StateFailed, Error: "harness 尚未初始化"}}
	}
	h := a.harness

	a.startupSteps = []StartupStep{
		{ID: "harness", Title: "检查 Harness", Status: StepPending},
		{ID: "market", Title: "安装插件市场", Status: StepPending},
		{ID: "start", Title: "启动 Harness", Status: StepPending},
	}
	status := a.startupStatus()
	a.mu.Unlock()

	go func() {
		// 步骤 1: 检查系统是否已装 harness。未装则不自动安装, 提示用户自行安装。
		// 先从桌面/终端合并 dsh/node 的常见安装目录到 PATH, 保证检测与子进程可用。
		a.setStep("harness", StepRunning, "检测 dsh 命令...")
		time.Sleep(stepPause)
		harness.EnsurePATH()
		mode := harness.LaunchMode()
		if mode == "" {
			nodeMissing := !harness.NodeAvailable()
			a.setInstallGuide(nodeMissing)
			msg := "未检测到 Harness, 请执行 npm install -g @deepseek-ai/dsh 后重试"
			if nodeMissing {
				msg = "未检测到 Harness, 且未找到 npm, 请先安装 Node.js"
			}
			a.setStep("harness", StepFailed, msg)
			return
		}
		detail := "已检测到 dsh"
		if mode == "npx" {
			detail = "未检测到 dsh, 将通过 npx 按需运行"
		}
		a.setStep("harness", StepDone, detail)
		time.Sleep(stepPause)

		// 步骤 2: 安装插件市场。必须在启动 harness 之前完成,
		// 否则 web profile 引用的插件未装, dsh 加载 profile 会直接退出。
		a.setStep("market", StepRunning, "检测并安装插件市场...")
		if !a.installPluginsForStartup(h.DshHome()) {
			return
		}
		a.setStep("market", StepDone, "插件市场就绪")
		time.Sleep(stepPause)

		// 步骤 3: 启动 harness, 就绪后才算通过。
		a.setStep("start", StepRunning, "通过 dsh 拉起 web...")
		if err := h.Start(); err != nil {
			a.setStep("start", StepFailed, err.Error())
			return
		}
		time.Sleep(stepPause)
		a.setStep("start", StepDone, "就绪: "+h.URL())

		// 插件安装稳定后监控 profile 变化, 需要重启的插件由原生提示接管。
		a.startPluginMonitor()
	}()

	return status
}

// Status 返回当前启动状态快照, 供前端轮询。
func (a *App) Status() StartupStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.startupErr) != 0 {
		return StartupStatus{Status: harness.Status{State: harness.StateFailed, Error: a.startupErr}}
	}
	if a.harness == nil {
		return StartupStatus{Status: harness.Status{State: harness.StateIdle}}
	}
	return a.startupStatus()
}

// startupStatus 汇总 harness 快照、外观主题与启动步骤, 调用方需持有 mu。
func (a *App) startupStatus() StartupStatus {
	status := StartupStatus{
		Status: a.harness.CurrentStatus(),
		Theme:  harness.ThemePreference(a.cfg.DshHome),
		Steps:  append([]StartupStep(nil), a.startupSteps...),
	}
	if a.installGuide != nil {
		guide := *a.installGuide
		status.Guide = &guide
	}
	return status
}

// OpenExternal 用系统默认浏览器打开外部链接(如 Node.js 官网)。
func (a *App) OpenExternal(url string) {
	wruntime.BrowserOpenURL(a.ctx, url)
}

// setStep 更新某个启动步骤的状态与详情。
func (a *App) setStep(id string, status StepStatus, detail string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.startupSteps {
		if a.startupSteps[i].ID == id {
			a.startupSteps[i].Status = status
			a.startupSteps[i].Detail = detail
			return
		}
	}
}

// setInstallGuide 构建未检测到 harness 时的安装引导。
func (a *App) setInstallGuide(nodeMissing bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	command := "npm install -g @deepseek-ai/dsh"
	if nodeMissing {
		a.installGuide = &InstallGuide{
			NodeMissing: true,
			Command:     command,
			Steps: []string{
				"安装 Node.js 运行时(官网: nodejs.org)",
				"执行: " + command,
				"完成安装后点击下方重试",
			},
		}
		return
	}
	a.installGuide = &InstallGuide{
		Command: command,
		Steps: []string{
			"执行: " + command,
			"完成安装后点击下方重试",
		},
	}
}

// installPluginsForStartup 同步安装默认插件, 并把进度反映到"安装插件市场"步骤。
// 返回是否全部成功, 失败时由调用方保持失败态。
func (a *App) installPluginsForStartup(workDir string) bool {
	a.mu.Lock()
	a.preinstallBusy = true
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.preinstallBusy = false
		a.mu.Unlock()
	}()

	success := true
	harness.EnsurePreinstalled(workDir, func(stage, name string, ok bool) {
		if !ok {
			success = false
			a.setStep("market", StepFailed, "插件 "+name+" 安装失败")
			return
		}
		switch stage {
		case "skip":
			a.setStep("market", StepRunning, "已跳过插件 "+name)
		case "detect":
			a.setStep("market", StepRunning, "检测插件 "+name+" ...")
		case "install":
			a.setStep("market", StepRunning, "正在安装插件 "+name+" ...")
		case "ok":
			a.setStep("market", StepRunning, "插件 "+name+" 就绪")
		}
	})
	return success
}

// Stop 停止 harness 子进程。
func (a *App) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.startupErr) != 0 {
		return errors.New(a.startupErr)
	}
	if a.harness == nil {
		return fmt.Errorf("harness 尚未初始化")
	}
	return a.harness.Stop()
}

// Restart 重启 harness 子进程。
func (a *App) Restart() harness.Status {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.startupErr) != 0 {
		return harness.Status{State: harness.StateFailed, Error: a.startupErr}
	}
	if a.harness == nil {
		return harness.Status{State: harness.StateFailed, Error: "harness 尚未初始化"}
	}
	h := a.harness

	go func() {
		if err := h.Restart(); err != nil {
			return
		}
		// 当前窗口已经导航到了 Harness；服务重新就绪后刷新它，加载新的插件集合。
		// 外部 Harness 页面没有 Wails runtime，必须执行浏览器原生 reload。
		wruntime.WindowExecJS(a.ctx, "window.location.reload()")
	}()
	return h.CurrentStatus()
}

// OpenInterface 用系统默认浏览器打开 Harness 界面。
func (a *App) OpenInterface() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.startupErr) != 0 {
		return errors.New(a.startupErr)
	}
	if a.harness == nil {
		return fmt.Errorf("harness 尚未初始化")
	}
	if a.harness.CurrentStatus().State != harness.StateReady {
		return fmt.Errorf("harness 尚未就绪, 无法打开界面")
	}
	wruntime.BrowserOpenURL(a.ctx, a.harness.URL())
	return nil
}

// Platform 返回当前平台信息, 供前端展示。
func (a *App) Platform() string {
	return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
}
