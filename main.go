package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"

	"dsh-desktop/internal/app"
	"dsh-desktop/internal/cfg"
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

	err = wails.Run(&options.App{
		Title:  "DSH Desktop",
		Width:  1200,
		Height: 800,
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
