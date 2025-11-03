//go:build windows
// +build windows

package update

import (
	"os/exec"
)

// setUnixProcAttr 在 Windows 上为空实现（Windows 不需要设置进程组）
func setUnixProcAttr(cmd *exec.Cmd) {
	// Windows 上不需要设置进程组
}
