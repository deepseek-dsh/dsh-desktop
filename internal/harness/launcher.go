package harness

import "os/exec"

// IsInstalled 检测系统 PATH 中是否已安装 dsh 命令。
func IsInstalled() bool {
	_, err := exec.LookPath("dsh")
	return err == nil
}

// NodeAvailable 检测系统是否已安装 npm(Node.js), 用于未装 harness 时给出安装指引。
func NodeAvailable() bool {
	_, err := exec.LookPath("npm")
	return err == nil
}

// newDshCommand 使用系统安装的 dsh 命令执行; 未安装时返回错误, 由上层提示用户自行安装。
func newDshCommand(args ...string) (*exec.Cmd, error) {
	dshPath, err := exec.LookPath("dsh")
	if err != nil {
		return nil, err
	}
	return exec.Command(dshPath, args...), nil
}