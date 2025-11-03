//go:build !windows
// +build !windows

package update

import (
	"os/exec"
	"syscall"
)

// setUnixProcAttr 为 Unix/Linux 系统设置进程属性
func setUnixProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}
}

