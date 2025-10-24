//go:build !windows
// +build !windows

package windows

import (
	"fmt"
	"log"
	"runtime"
)

// CheckWindowsDependencies 在非Windows系统上的空实现
func CheckWindowsDependencies() error {
	if runtime.GOOS == "windows" {
		log.Println("警告: 在Windows系统上使用了非Windows版本的检查函数")
	}
	log.Println("非Windows系统，跳过Windows依赖检查")
	return nil
}

// RequestAdminPrivileges 在非Windows系统上的空实现
func RequestAdminPrivileges() error {
	return fmt.Errorf("此功能仅在Windows上可用")
}

// GetInstallationGuide 在非Windows系统上的空实现
func GetInstallationGuide() string {
	return "此功能仅在Windows上可用"
}
