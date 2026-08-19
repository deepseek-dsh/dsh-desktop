package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"dsh-desktop/internal/cfg"
	"dsh-desktop/internal/harness"
)

// App 是 Wails 暴露给前端唯一的绑定实例。
type App struct {
	ctx            context.Context
	cfg            *cfg.Config
	harness        *harness.Harness
	startupErr     string // 目录初始化失败时记录, 供前端展示
	mu             sync.Mutex
	monitorOnce    sync.Once
	monitorDone    chan struct{}
	closeOnce      sync.Once
	preinstallBusy bool
}

// New 创建后端应用实例。
func New(c *cfg.Config) *App {
	return &App{cfg: c, monitorDone: make(chan struct{})}
}

// Startup 在窗口启动时被调用, 缓存上下文、创建目录并预置子进程对象。
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

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

// Start 异步拉起 harness 子进程, 就绪后置 ready; 返回当前状态快照。
func (a *App) Start() harness.Status {
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
		// 异步执行，前端通过低频 Status 轮询观察 ready/failed。
		// 不从后端事件回调中执行整页导航，避免 Linux WebKit 同步 JS 通道被导航卡住。
		if err := h.Start(); err != nil {
			return
		}
		a.startPluginMonitor()

		// 进入主页后在后台静默安装插件市场, 不阻塞、不打扰。
		// 市场是 bundle 插件, 装好后需刷新/重启 Harness 才在当前会话生效。
		go a.silentlyEnsurePlugins(h.DshHome())
	}()

	return h.CurrentStatus()
}

// Status 返回当前运行状态快照, 供前端轮询。
func (a *App) Status() harness.Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.startupErr) != 0 {
		return harness.Status{State: harness.StateFailed, Error: a.startupErr}
	}
	if a.harness == nil {
		return harness.Status{State: harness.StateIdle}
	}
	return a.harness.CurrentStatus()
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

// silentlyEnsurePlugins 在后台静默安装默认插件(插件市场), 不阻塞进入主页。
// profile 清单变化由 plugin monitor 观察，并在确实需要时显示原生重启提示。
func (a *App) silentlyEnsurePlugins(workDir string) {
	a.mu.Lock()
	a.preinstallBusy = true
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.preinstallBusy = false
		a.mu.Unlock()
	}()

	harness.EnsurePreinstalled(workDir, nil)
}

// Platform 返回当前平台信息, 供前端展示。
func (a *App) Platform() string {
	return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
}
