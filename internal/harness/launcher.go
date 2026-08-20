package harness

import (
	"os/exec"
)

// LaunchMethod 返回实际会采用的启动方式: 系统已装 dsh 返回 "dsh", 否则返回 "npx"。
func LaunchMethod() string {
	if _, err := exec.LookPath("dsh"); err == nil {
		return "dsh"
	}
	return "npx"
}

// NpxAvailable 检测系统是否可用 npx(Node.js)。
func NpxAvailable() bool {
	_, err := exec.LookPath("npx")
	return err == nil
}

// newDshCommand 优先复用系统 PATH 中安装的 dsh；未安装时才通过 npx
// 启动兼容版本，使桌面端在干净系统上仍可运行。
func newDshCommand(args ...string) *exec.Cmd {
	if dshPath, err := exec.LookPath("dsh"); err == nil {
		return exec.Command(dshPath, args...)
	}
	fallbackArgs := append([]string{"--yes", DshPackage}, args...)
	return exec.Command("npx", fallbackArgs...)
}
