// 冒烟验证: 真实拉起 dsh web, 确认就绪后优雅关闭。
// 仅用于开发期验证, 不作为产品入口。
package main

import (
	"fmt"
	"path/filepath"
	"time"

	"dsh-desktop/internal/cfg"
	"dsh-desktop/internal/harness"
)

func main() {
	config, err := cfg.Load()
	if err != nil {
		fmt.Printf("配置加载失败: %v\n", err)
		return
	}
	if err := config.EnsureDirs(); err != nil {
		fmt.Printf("创建数据目录失败: %v\n", err)
		return
	}

	logFile := filepath.Join(config.LogDir, "smoke-harness.log")
	h := harness.New(config.DshHome, logFile, config.Port)

	fmt.Printf("开始拉起 harness, 目标端口 %d, DSH_HOME=%s\n", config.Port, config.DshHome)
	start := time.Now()
	if err := h.Start(); err != nil {
		fmt.Printf("启动失败: %v\n", err)
		return
	}
	fmt.Printf("就绪耗时 %v, URL=%s, 状态=%s\n",
		time.Since(start).Round(time.Millisecond), h.URL(), h.CurrentStatus().State)

	fmt.Println("执行一次受管重启...")
	restart := time.Now()
	if err := h.Restart(); err != nil {
		fmt.Printf("重启失败: %v\n", err)
		return
	}
	fmt.Printf("重启完成, 耗时 %v, 状态=%s\n",
		time.Since(restart).Round(time.Millisecond), h.CurrentStatus().State)

	fmt.Println("等待 1s 后优雅关闭...")
	time.Sleep(time.Second)
	if err := h.Stop(); err != nil {
		fmt.Printf("关闭出错: %v\n", err)
		return
	}
	fmt.Printf("已优雅关闭, 最终状态=%s\n", h.CurrentStatus().State)
}
