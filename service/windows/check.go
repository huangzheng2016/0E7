//go:build windows
// +build windows

package windows

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"unsafe"

	"github.com/google/gopacket/pcap"
	"golang.org/x/sys/windows"
)

// Windows API constants
const (
	TOKEN_QUERY = 0x0008
)

var (
	advapi32                = windows.NewLazyDLL("advapi32.dll")
	procOpenProcessToken    = advapi32.NewProc("OpenProcessToken")
	procGetTokenInformation = advapi32.NewProc("GetTokenInformation")
	procCloseHandle         = windows.NewLazyDLL("kernel32.dll").NewProc("CloseHandle")
)

// TokenElevation represents the elevation status of a token
type TokenElevation struct {
	TokenIsElevated uint32
}

// CheckWindowsDependencies 检查Windows下的依赖项
func CheckWindowsDependencies() error {
	if runtime.GOOS != "windows" {
		return nil // 非Windows系统跳过检查
	}

	log.Println("正在检查Windows依赖项...")

	// 检查管理员权限（仅提示，不阻止运行）
	if err := checkAdminPrivileges(); err != nil {
		log.Printf("警告 权限检查: %v", err)
		log.Println("提示: 没有管理员权限，实时网络监控功能可能受限，但pcap文件分析功能仍可正常使用")
		log.Println("建议: 如需完整功能，请右键以管理员身份运行程序")
	} else {
		log.Println("成功 具有管理员权限，所有功能可用")
	}

	// 检查pcap库（仅提示，不阻止运行）
	if err := checkPcapLibrary(); err != nil {
		log.Printf("警告 PCAP库检查: %v", err)
		log.Println("提示: 未检测到WinPcap/Npcap，实时网络监控功能不可用，但pcap文件分析功能仍可正常使用")
		showDownloadLinks()
	} else {
		log.Println("成功 PCAP库可用，网络监控功能正常")
	}

	// 检查网络适配器（仅提示）
	if err := checkNetworkAdapters(); err != nil {
		log.Printf("信息 网络适配器: %v", err)
		log.Println("提示: 网络适配器检查失败，但不影响pcap文件分析功能")
	}

	log.Println("成功 Windows依赖检查完成，程序可以正常运行")
	return nil
}

// showDownloadLinks 显示下载链接
func showDownloadLinks() {
	log.Println("如需完整功能，请下载安装:")
	log.Println("   Npcap (推荐): https://npcap.com/")
	log.Println("   WinPcap: https://www.winpcap.org/install/")
	log.Println("   Nmap (包含WinPcap): https://nmap.org/download.html")
	log.Println("")
	log.Println("安装提示:")
	log.Println("   1. 推荐使用Npcap (支持Windows 10/11)")
	log.Println("   2. 右键以管理员身份运行安装程序")
	log.Println("   3. 安装完成后重启程序")
	log.Println("   4. 如遇问题，可尝试关闭杀毒软件后安装")
}

// checkAdminPrivileges 检查是否具有管理员权限
func checkAdminPrivileges() error {
	log.Println("检查管理员权限...")

	// 方法1: 检查当前进程的令牌
	if isElevated, err := isProcessElevated(); err == nil {
		if isElevated {
			log.Println("成功 当前进程具有管理员权限")
			return nil
		} else {
			log.Println("警告 当前进程没有管理员权限")
			return fmt.Errorf("需要管理员权限以访问网络适配器")
		}
	}

	// 方法2: 尝试打开需要管理员权限的资源
	if err := testAdminAccess(); err != nil {
		log.Println("警告 无法访问需要管理员权限的资源")
		return fmt.Errorf("权限不足: %v", err)
	}

	log.Println("成功 权限检查通过")
	return nil
}

// isProcessElevated 检查当前进程是否具有提升的权限
func isProcessElevated() (bool, error) {
	handle, err := windows.GetCurrentProcess()
	if err != nil {
		return false, err
	}

	var token windows.Token
	err = windows.OpenProcessToken(handle, TOKEN_QUERY, &token)
	if err != nil {
		return false, err
	}
	defer token.Close()

	var elevation TokenElevation
	var returnedLen uint32
	err = windows.GetTokenInformation(token, windows.TokenElevation, (*byte)(unsafe.Pointer(&elevation)), uint32(unsafe.Sizeof(elevation)), &returnedLen)
	if err != nil {
		return false, err
	}

	return elevation.TokenIsElevated != 0, nil
}

// testAdminAccess 测试管理员访问权限
func testAdminAccess() error {
	// 尝试访问需要管理员权限的注册表项
	cmd := exec.Command("reg", "query", "HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall", "/s", "/f", "WinPcap")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("无法访问注册表: %v", err)
	}

	// 如果能执行到这里，说明有足够的权限
	_ = output
	return nil
}

// checkPcapLibrary 检查pcap库是否可用
func checkPcapLibrary() error {
	log.Println("检查PCAP库...")

	// 检查WinPcap
	if isWinPcapInstalled() {
		log.Println("成功 检测到WinPcap已安装")
		return nil
	}

	// 检查Npcap
	if isNpcapInstalled() {
		log.Println("成功 检测到Npcap已安装")
		return nil
	}

	// 尝试使用pcap库进行基本测试
	devices, err := pcap.FindAllDevs()
	if err != nil {
		log.Printf("警告 pcap库测试失败: %v", err)
		log.Println("提示: 这不会影响pcap文件分析功能，只是实时网络监控功能不可用")
		return fmt.Errorf("pcap库不可用，实时网络监控功能受限")
	}

	if len(devices) == 0 {
		log.Println("警告 未找到网络设备")
		log.Println("提示: 这不会影响pcap文件分析功能，只是实时网络监控功能不可用")
		return fmt.Errorf("未找到网络设备，实时网络监控功能受限")
	}

	log.Printf("成功 找到 %d 个网络设备", len(devices))
	return nil
}

// isWinPcapInstalled 检查WinPcap是否安装
func isWinPcapInstalled() bool {
	// 检查注册表
	cmd := exec.Command("reg", "query", "HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall", "/s", "/f", "WinPcap")
	output, err := cmd.Output()
	if err == nil && strings.Contains(string(output), "WinPcap") {
		return true
	}

	// 检查文件系统
	winPcapPaths := []string{
		"C:\\Windows\\System32\\wpcap.dll",
		"C:\\Windows\\System32\\packet.dll",
		"C:\\Program Files\\WinPcap",
		"C:\\Program Files (x86)\\WinPcap",
	}

	for _, path := range winPcapPaths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}

// isNpcapInstalled 检查Npcap是否安装
func isNpcapInstalled() bool {
	// 检查注册表
	cmd := exec.Command("reg", "query", "HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall", "/s", "/f", "Npcap")
	output, err := cmd.Output()
	if err == nil && strings.Contains(string(output), "Npcap") {
		return true
	}

	// 检查文件系统
	npcapPaths := []string{
		"C:\\Windows\\System32\\Npcap",
		"C:\\Program Files\\Npcap",
		"C:\\Program Files (x86)\\Npcap",
	}

	for _, path := range npcapPaths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}

// checkNetworkAdapters 检查网络适配器
func checkNetworkAdapters() error {
	log.Println("检查网络适配器...")

	devices, err := pcap.FindAllDevs()
	if err != nil {
		return fmt.Errorf("无法获取网络设备列表: %v", err)
	}

	if len(devices) == 0 {
		return fmt.Errorf("未找到可用的网络适配器")
	}

	log.Printf("成功 找到 %d 个网络适配器:", len(devices))
	for i, device := range devices {
		if i < 3 { // 只显示前3个设备
			log.Printf("  - %s: %s", device.Name, device.Description)
		}
	}
	if len(devices) > 3 {
		log.Printf("  ... 还有 %d 个设备", len(devices)-3)
	}

	return nil
}

// RequestAdminPrivileges 请求管理员权限（重新启动程序）
func RequestAdminPrivileges() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("此功能仅在Windows上可用")
	}

	log.Println("正在请求管理员权限...")

	// 获取当前程序路径
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法获取程序路径: %v", err)
	}

	// 使用runas命令以管理员身份重新启动
	cmd := exec.Command("runas", "/user:Administrator", exePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("无法以管理员身份启动程序: %v", err)
	}

	// 退出当前进程
	os.Exit(0)
	return nil
}

// GetInstallationGuide 获取安装指南
func GetInstallationGuide() string {
	guide := `
Windows PCAP库安装指南:

1. Npcap (推荐 - 支持Windows 10/11):
   下载地址: https://npcap.com/
   支持Windows 10/11
   支持现代网络功能
   更好的性能和稳定性

2. WinPcap (传统选择 - 支持旧系统):
   下载地址: https://www.winpcap.org/install/
   支持Windows XP/7/8
   不支持Windows 10/11的某些功能

3. 通过Nmap安装 (包含WinPcap):
   下载地址: https://nmap.org/download.html
   包含WinPcap组件
   同时获得网络扫描工具

安装步骤:
1. 下载对应版本的安装包
2. 右键以管理员身份运行安装程序
3. 按照安装向导完成安装
4. 重启程序以启用完整功能

注意事项:
- 安装时需要管理员权限
- 某些杀毒软件可能会阻止安装
- 建议关闭杀毒软件实时保护后再安装
- 安装后可能需要重启计算机

故障排除:
- 如果安装失败，请检查是否有其他网络监控软件冲突
- 确保系统版本与安装包兼容
- 可以尝试以兼容模式运行安装程序
`
	return guide
}
