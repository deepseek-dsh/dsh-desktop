//go:build !windows

package harness

import (
	"errors"
	"os/exec"
	"syscall"
)

// ErrProcessGone 表示目标进程组已不存在(正常退出)。
var ErrProcessGone = errors.New("进程组已不存在")

// setProcessGroup 让子进程运行在独立进程组, 便于退出时整棵进程树回收。
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessTree 强杀整个进程组(含孙进程)。
func killProcessTree(pid int) error {
	if pid == 0 {
		return nil
	}
	return normalizeKillError(syscall.Kill(-pid, syscall.SIGKILL))
}

// terminateProcessTree 向进程组发送 SIGTERM, 请求优雅退出。
func terminateProcessTree(pid int) error {
	if pid == 0 {
		return nil
	}
	return normalizeKillError(syscall.Kill(-pid, syscall.SIGTERM))
}

// normalizeKillError 把 ESRCH(进程组已退出)归一化为 ErrProcessGone。
func normalizeKillError(err error) error {
	if errors.Is(err, syscall.ESRCH) {
		return ErrProcessGone
	}
	return err
}
