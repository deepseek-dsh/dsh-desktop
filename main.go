package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"

	"dsh-desktop/internal/app"
	"dsh-desktop/internal/cfg"
	"dsh-desktop/internal/harness"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

func main() {
	config, err := cfg.Load()
	if err != nil {
		println("初始化失败:", err.Error())
		return
	}

	instance := app.New(config)

	// WebView 底色跟随 harness 外观主题, 深色主题下导航主界面时不闪白。
	background := options.RGBA{R: 255, G: 255, B: 255, A: 255}
	if harness.ThemePreference(config.DshHome) == "dark" {
		background = options.RGBA{R: 15, G: 23, B: 32, A: 255}
	}

	err = wails.Run(&options.App{
		Title:  "DSH Desktop",
		Width:  1200,
		Height: 800,
		// 与启动页背景一致, 避免导航到 Harness 期间出现白屏/黑屏闪烁。
		BackgroundColour: &background,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Linux: &linux.Options{
			Icon: icon, // Linux 窗口/任务栏图标(favicon)。
		},
		OnStartup:  instance.Startup,
		OnShutdown: instance.Shutdown,
		Bind: []any{
			instance,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
