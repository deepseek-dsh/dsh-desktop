package harness

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Plugin 描述一个要预装进 web profile 的插件。
type Plugin struct {
	// Name 是展示名, 会出现在启动页状态文案里。
	Name string
	// Source 是 dsh plugin add 的安装来源(如 npm 包名或 github:...#ref)。
	Source string
}

// DefaultPlugins 是启动时默认预装的插件清单。
var DefaultPlugins = []Plugin{
	{Name: "dshmarket", Source: "dshmarket"},
}

// UsagePluginSourceEnv 可指定本地或远程的用量插件安装来源。
// 开发时通常指向 @dsh-plugins/usage 的本地 package 目录。
const UsagePluginSourceEnv = "DSH_DESKTOP_USAGE_PLUGIN"

// preinstalledPlugins 返回本次启动需要确保安装的插件。
// 用量插件保持显式配置，避免发布后的 desktop 绑定开发机绝对路径。
func preinstalledPlugins() []Plugin {
	plugins := append([]Plugin(nil), DefaultPlugins...)
	if source := strings.TrimSpace(os.Getenv(UsagePluginSourceEnv)); source != "" {
		plugins = append(plugins, Plugin{Name: "@dsh-plugins/usage", Source: source})
	}
	return plugins
}

// MarketInstallTimeout 是单个插件安装的上限。
const MarketInstallTimeout = 180 * time.Second

// skipPreinstall 控制是否跳过一切插件预装(环境变量 DSH_DESKTOP_SKIP_PLUGINS=1)。
func skipPreinstall() bool {
	return os.Getenv("DSH_DESKTOP_SKIP_PLUGINS") == "1"
}

// PluginInstalled 探测 web profile 是否已装指定插件(按目录名)。
func PluginInstalled(workDir, name string) bool {
	info, err := os.Stat(filepath.Join(workDir, "profiles", "web", "node_modules", name))
	return err == nil && info.IsDir()
}

// AllowlistedBuildScripts 是被放行 build 脚本的常用安全原生依赖(pnpm allowBuilds)。
// 从插件市场安装用到这些依赖的插件时, 不再被 pnpm 默认拦截; 白名单外的包仍按默认拦截,
// 需要用户在此手动放行, 以保留供应链安全。
var AllowlistedBuildScripts = []string{
	"node-pty",     // dsh-plugin-terminal / dsh-bash-terminal
	"cloudflared",  // dsh-web-ui-all 等网络插件
	"cpu-features", // ssh2 的原生依赖
	"ssh2",         // dsh-web-ui-all 等远程/SSH 插件
}

// ensureBuildAllowlist 在 web profile 的 pnpm-workspace.yaml 中写入 allowBuilds 白名单。
// 幂等: 已存在 allowBuilds 块则跳过; 不破坏 dsh 生成的既有配置。
func ensureBuildAllowlist(workDir string) {
	wsFile := filepath.Join(workDir, "profiles", "web", "pnpm-workspace.yaml")
	data, err := os.ReadFile(wsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[dsh-desktop] 跳过 build 白名单配置(pnpm-workspace.yaml 不可读): %v\n", err)
		return
	}
	if bytes.Contains(data, []byte("allowBuilds:")) {
		return
	}

	var b strings.Builder
	b.Write(bytes.TrimRight(data, "\n"))
	b.WriteString("\nallowBuilds:\n")
	for _, pkg := range AllowlistedBuildScripts {
		_, _ = b.WriteString("  " + pkg + ": true\n")
	}
	if err := os.WriteFile(wsFile, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[dsh-desktop] 写入 build 白名单失败: %v\n", err)
	}
}

// EnsurePreinstalled 依次检测并安装默认插件, 每步通过 onStage 报告状态。
// 幂等: 已装直接跳过; 缺 pnpm/离线等安装失败仅报告, 不阻断后续或整体启动。
// onStage(stage, name, ok) — stage 为 "detect"/"install"/"ok"/"skip"。
func EnsurePreinstalled(workDir string, onStage func(stage, name string, ok bool)) {
	plugins := preinstalledPlugins()
	if skipPreinstall() {
		if onStage != nil {
			for _, p := range plugins {
				onStage("skip", p.Name, true)
			}
		}
		return
	}

	// 先写入 build 白名单, 再依次安装插件, 避免后续从市场装的插件被 pnpm 拦截原生构建。
	ensureBuildAllowlist(workDir)

	for _, p := range plugins {
		// 检测
		if onStage != nil {
			onStage("detect", p.Name, true)
		}
		if PluginInstalled(workDir, p.Name) {
			if onStage != nil {
				onStage("ok", p.Name, true)
			}
			continue
		}

		// 安装
		if onStage != nil {
			onStage("install", p.Name, true)
		}
		if err := installPlugin(workDir, p.Name, p.Source); err != nil {
			fmt.Fprintf(os.Stderr, "[dsh-desktop] 插件 %s 安装失败, 已跳过: %v\n", p.Name, err)
			if onStage != nil {
				onStage("skip", p.Name, false)
			}
			continue
		}
		if onStage != nil {
			onStage("ok", p.Name, true)
		}
	}
}

// installPlugin 通过 dsh CLI 把指定插件装进 web profile。
func installPlugin(workDir, name, source string) error {
	cmd, err := newDshCommand("plugin", "--profile", "web", "add", source)
	if err != nil {
		return fmt.Errorf("未检测到 dsh 命令, 请先安装 Harness: %w", err)
	}
	cmd.Dir = workDir
	// 与 Harness 创建 profile 时保持同一 pnpm store。若在这里另设 XDG_DATA_HOME，
	// pnpm 会因现有 node_modules 来自另一 store 而报 ERR_PNPM_UNEXPECTED_STORE。
	cmd.Env = append(os.Environ(), "DSH_HOME="+workDir)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	// 独立进程组, 便于超时后回收整棵安装进程树。
	setProcessGroup(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动插件 %s 安装失败: %w", name, err)
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("插件 %s 安装失败: %v", name, err)
		}
		return nil
	case <-time.After(MarketInstallTimeout):
		// 超时清理进程组, 避免残留安装进程。
		_ = killProcessTree(cmd.Process.Pid)
		<-done
		return fmt.Errorf("插件 %s 安装超时(%v)", name, MarketInstallTimeout)
	}
}
