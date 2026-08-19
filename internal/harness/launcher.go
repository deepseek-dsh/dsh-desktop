package harness

import (
	"os/exec"
)

// newDshCommand 优先复用系统 PATH 中安装的 dsh；未安装时才通过 npx
// 启动兼容版本，使桌面端在干净系统上仍可运行。
func newDshCommand(args ...string) *exec.Cmd {
	if dshPath, err := exec.LookPath("dsh"); err == nil {
		return exec.Command(dshPath, args...)
	}
	fallbackArgs := append([]string{"--yes", DshPackage}, args...)
	return exec.Command("npx", fallbackArgs...)
}
