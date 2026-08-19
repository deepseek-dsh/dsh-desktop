//go:build windows

package harness

import (
	"errors"
	"os/exec"
	"strconv"
	"syscall"
)

// ErrProcessGone 表示目标进程已不存在(正常退出)。
var ErrProcessGone = errors.New("进程已不存在")

// setProcessGroup 让子进程运行在独立进程组, 便于退出时整棵进程树回收。
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// killProcessTree 强杀整个进程树(含子进程)。
func killProcessTree(pid int) error {
	if pid == 0 {
		return nil
	}
	return normalizeKillError(exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run())
}

// terminateProcessTree 请求进程树优雅退出。
func terminateProcessTree(pid int) error {
	if pid == 0 {
		return nil
	}
	return normalizeKillError(exec.Command("taskkill", "/PID", strconv.Itoa(pid)).Run())
}

// normalizeKillError 把 taskkill 的失败(进程已不存在)归一化为 ErrProcessGone。
func normalizeKillError(err error) error {
	if err != nil {
		return ErrProcessGone
	}
	return nil
}
