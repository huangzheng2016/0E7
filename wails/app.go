//go:build wails
// +build wails

package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	winOptions "github.com/wailsapp/wails/v2/pkg/options/windows"
)

// 不需要嵌入前端资源，因为我们使用 AssetServer.Handler 代理到后端服务器

// App struct
type App struct {
	ctx         context.Context
	backendCmd  *exec.Cmd
	backendPort int
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	// 清理后端进程
	if a.backendCmd != nil && a.backendCmd.Process != nil {
		a.backendCmd.Process.Kill()
		a.backendCmd.Wait()
	}
}

// getUserDataDir 获取用户数据目录：~/.0e7/
func getUserDataDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	dataDir := filepath.Join(homeDir, ".0e7")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "."
	}
	return dataDir
}

// findFreePort 查找可用端口（参考 electron 的逻辑：45000-55000）
func findFreePort(min, max, retries int) (int, error) {
	for i := 0; i < retries; i++ {
		rng := time.Now().UnixNano() + int64(i)
		port := min + int(rng%int64(max-min+1))

		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			addr := listener.Addr().(*net.TCPAddr)
			port := addr.Port
			listener.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("无法找到可用端口")
}

// resolveBinaryPath 解析二进制文件路径（参考 electron 的逻辑）
func resolveBinaryPath() (string, error) {
	var binaryName string

	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			binaryName = "0e7_darwin_arm64"
		} else {
			binaryName = "0e7_darwin_amd64"
		}
	case "linux":
		binaryName = "0e7_linux_amd64"
	case "windows":
		binaryName = "0e7_windows_amd64.exe"
	default:
		return "", fmt.Errorf("不支持的平台: %s", runtime.GOOS)
	}

	// 开发模式下，从项目根目录查找（相对于 wails 目录）
	binaryPathInRoot := filepath.Join("..", binaryName)
	if _, err := os.Stat(binaryPathInRoot); err == nil {
		absPath, _ := filepath.Abs(binaryPathInRoot)
		return absPath, nil
	}

	// 打包后，从 Resources/bin 目录查找（参考 electron 的结构）
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	exeDir := filepath.Dir(exePath)

	// macOS: 从 .app/Contents/Resources/bin 查找
	// Windows/Linux: 从可执行文件同目录的 bin 查找
	var resourcesBinDir string
	if runtime.GOOS == "darwin" {
		// macOS: 可执行文件在 .app/Contents/MacOS/，Resources 在 .app/Contents/Resources/
		resourcesBinDir = filepath.Join(exeDir, "..", "Resources", "bin")
	} else {
		// Windows/Linux: 从可执行文件同目录的 bin 查找
		resourcesBinDir = filepath.Join(exeDir, "bin")
	}

	resourcesBinDir, _ = filepath.Abs(resourcesBinDir)
	binaryPath := filepath.Join(resourcesBinDir, binaryName)

	if _, err := os.Stat(binaryPath); err == nil {
		return binaryPath, nil
	}

	// 如果找不到，尝试从可执行文件同目录查找
	binaryPath = filepath.Join(exeDir, binaryName)
	if _, err := os.Stat(binaryPath); err == nil {
		return binaryPath, nil
	}

	return "", fmt.Errorf("未找到二进制文件: %s (已查找: %s, %s)", binaryName, filepath.Join(resourcesBinDir, binaryName), binaryPath)
}

// launchBackend 启动后端进程（参考 electron 的逻辑）
func launchBackend(port int) (*exec.Cmd, error) {
	binaryPath, err := resolveBinaryPath()
	if err != nil {
		return nil, fmt.Errorf("解析二进制路径失败: %v", err)
	}

	userDataDir := getUserDataDir()
	configPath := filepath.Join(userDataDir, "config.ini")

	args := []string{
		"--server",
		"--config", configPath,
		"--server-port", fmt.Sprintf("%d", port),
	}

	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = userDataDir
	cmd.Env = append(os.Environ(), fmt.Sprintf("OE7_SERVER_PORT=%d", port))

	// 重定向输出
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动后端进程失败: %v", err)
	}

	return cmd, nil
}

// waitForServer 等待服务器启动
func waitForServer(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://127.0.0.1:%d", port)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 1*time.Second)
		if err == nil {
			conn.Close()
			// 再等待一下确保服务器完全启动
			time.Sleep(500 * time.Millisecond)
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("等待服务器启动超时: %s", url)
}

func main() {
	// 查找可用端口（参考 electron：45000-55000）
	port, err := findFreePort(45000, 55000, 50)
	if err != nil {
		fmt.Fprintf(os.Stderr, "无法找到可用端口: %v\n", err)
		os.Exit(1)
	}

	// 启动后端进程
	backendCmd, err := launchBackend(port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "启动后端失败: %v\n", err)
		os.Exit(1)
	}

	// 设置信号处理，确保程序退出时关闭后端进程
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		if backendCmd != nil && backendCmd.Process != nil {
			backendCmd.Process.Kill()
			backendCmd.Wait()
		}
		os.Exit(0)
	}()

	// 等待服务器启动
	if err := waitForServer(port, 60*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		if backendCmd != nil && backendCmd.Process != nil {
			backendCmd.Process.Kill()
		}
		os.Exit(1)
	}

	// 创建应用
	app := NewApp()
	app.backendCmd = backendCmd
	app.backendPort = port

	// 创建应用选项
	backendURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	appOptions := &options.App{
		Title:     "0E7 Desktop",
		Width:     1366,
		Height:    900,
		MinWidth:  1200,
		MinHeight: 720,
		// 使用 AssetServer.Handler 代理所有请求到后端服务器
		AssetServer: &assetserver.Options{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// 创建代理请求
				proxyReq, err := http.NewRequest(r.Method, backendURL+r.URL.Path+"?"+r.URL.RawQuery, r.Body)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				// 复制请求头
				for key, values := range r.Header {
					for _, value := range values {
						proxyReq.Header.Add(key, value)
					}
				}
				// 执行请求
				client := &http.Client{Timeout: 30 * time.Second}
				resp, err := client.Do(proxyReq)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadGateway)
					return
				}
				defer resp.Body.Close()
				// 复制响应头
				for key, values := range resp.Header {
					for _, value := range values {
						w.Header().Add(key, value)
					}
				}
				w.WriteHeader(resp.StatusCode)
				// 复制响应体
				io.Copy(w, resp.Body)
			}),
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
	}

	// macOS 特定配置：确保关闭按钮不遮挡内容
	if runtime.GOOS == "darwin" {
		appOptions.Mac = &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  false,
				FullSizeContent:            true, // 允许内容延伸到标题栏下方
				UseToolbar:                 false,
			},
			Appearance:           mac.NSAppearanceNameAqua,
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		}
	}

	// Windows 特定配置
	if runtime.GOOS == "windows" {
		appOptions.Windows = &winOptions.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		}
	}

	// 创建并运行应用
	if err := wails.Run(appOptions); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		if backendCmd != nil && backendCmd.Process != nil {
			backendCmd.Process.Kill()
		}
		os.Exit(1)
	}
}
