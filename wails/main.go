package main

import (
	"context"
	"crypto/rand"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	goRuntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed bin/backend.bin
var embeddedBackend []byte

const (
	backendMinPort        = 45000
	backendMaxPort        = 55000
	backendPortRetryCount = 50
	backendReadyTimeout   = 60 * time.Second
	backendReadyInterval  = 500 * time.Millisecond
)

func main() {
	app := NewDesktopApp()

	// 在窗口显示前先启动后端并等待就绪
	if err := app.startBackend(); err != nil {
		log.Fatalf("启动后端失败: %v", err)
	}

	// 等待后端就绪
	select {
	case <-app.readyCh:
		// 后端已就绪
	case <-time.After(backendReadyTimeout):
		log.Fatalf("后端启动超时")
	}

	backendURL := fmt.Sprintf("http://127.0.0.1:%d", app.backendPort)
	redirectHTML := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<meta http-equiv="refresh" content="0;url=%s">
	<script>window.location.href='%s';</script>
</head>
<body>
	<p>正在跳转到后端服务器...</p>
	<p><a href="%s">如果未自动跳转，请点击这里</a></p>
</body>
</html>`, backendURL, backendURL, backendURL)

	if err := wails.Run(&options.App{
		Title:     "0E7 Desktop",
		Width:     1366,
		Height:    900,
		MinWidth:  1200,
		MinHeight: 720,
		AssetServer: &assetserver.Options{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/" {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.Write([]byte(redirectHTML))
					return
				}
				app.handleRequest(w, r)
			}),
		},
		OnStartup:  app.onStartup,
		OnShutdown: app.onShutdown,
		Debug: options.Debug{
			OpenInspectorOnStartup: isDevMode(),
		},
		WindowStartState:         options.Normal,
		Frameless:                false,
		EnableDefaultContextMenu: true,
		DisableResize:            false,
		Menu:                     nil,
	}); err != nil {
		log.Fatalf("failed to launch Wails shell: %v", err)
	}
}

func isDevMode() bool {
	env := strings.ToLower(os.Getenv("WAILS_ENV"))
	if env == "dev" {
		return true
	}
	env = strings.ToLower(os.Getenv("NODE_ENV"))
	return env == "development"
}

type DesktopApp struct {
	mu          sync.RWMutex
	backendCmd  *exec.Cmd
	backendPort int
	proxy       *httputil.ReverseProxy
	readyOnce   sync.Once
	readyCh     chan struct{}
	ctx         context.Context
}

func NewDesktopApp() *DesktopApp {
	return &DesktopApp{
		readyCh: make(chan struct{}),
	}
}

func (a *DesktopApp) onStartup(ctx context.Context) {
	a.ctx = ctx
	if err := a.startBackend(); err != nil {
		wailsRuntime.MessageDialog(ctx, wailsRuntime.MessageDialogOptions{
			Type:    wailsRuntime.ErrorDialog,
			Title:   "0E7 启动失败",
			Message: err.Error(),
		})
		wailsRuntime.Quit(ctx)
		return
	}
	wailsRuntime.LogInfo(ctx, fmt.Sprintf("0E7 backend is listening on %d", a.backendPort))
}

func (a *DesktopApp) onShutdown(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.backendCmd != nil && a.backendCmd.Process != nil {
		_ = a.backendCmd.Process.Kill()
		a.backendCmd = nil
	}
}

func (a *DesktopApp) handleRequest(w http.ResponseWriter, r *http.Request) {
	proxy := a.getProxy()
	if proxy == nil {
		http.Error(w, "0E7 backend is still starting...", http.StatusServiceUnavailable)
		return
	}

	// 检查是否是 WebSocket 升级请求
	if r.Header.Get("Upgrade") == "websocket" {
		a.handleWebSocket(w, r)
		return
	}

	proxy.ServeHTTP(w, r)
}

func (a *DesktopApp) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	backendPort := a.backendPort
	a.mu.RUnlock()

	if backendPort == 0 {
		http.Error(w, "0E7 backend is still starting...", http.StatusServiceUnavailable)
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "WebSocket not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hj.Hijack()
	if err != nil {
		http.Error(w, fmt.Sprintf("无法劫持连接: %v", err), http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	backendAddr := fmt.Sprintf("127.0.0.1:%d", backendPort)
	backendConn, err := net.Dial("tcp", backendAddr)
	if err != nil {
		clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer backendConn.Close()

	r.URL.Scheme = "http"
	r.URL.Host = backendAddr
	if err := r.Write(backendConn); err != nil {
		return
	}

	errChan := make(chan error, 2)

	go func() {
		_, err := io.Copy(backendConn, clientConn)
		if err != nil {
			errChan <- err
		}
	}()

	go func() {
		_, err := io.Copy(clientConn, backendConn)
		if err != nil {
			errChan <- err
		}
	}()

	<-errChan
}

func (a *DesktopApp) getProxy() *httputil.ReverseProxy {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.proxy
}

func (a *DesktopApp) startBackend() error {
	port, err := findFreePort()
	if err != nil {
		return fmt.Errorf("无法找到可用端口: %w", err)
	}

	backendPath, err := a.prepareBackendBinary()
	if err != nil {
		return err
	}

	userDir, err := ensureUserDataDir()
	if err != nil {
		return err
	}

	configPath := filepath.Join(userDir, "config.ini")
	args := []string{"--server", "--config", configPath, "--server-port", fmt.Sprintf("%d", port)}

	cmd := exec.Command(backendPath, args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("OE7_SERVER_PORT=%d", port))
	cmd.Dir = userDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 0E7 后端失败: %w", err)
	}

	if err := waitForBackend(port); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("0E7 后端未能在预期时间内启动: %w", err)
	}

	targetURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, proxyErr error) {
		http.Error(w, fmt.Sprintf("0E7 后端不可用: %v", proxyErr), http.StatusBadGateway)
	}

	a.mu.Lock()
	a.backendCmd = cmd
	a.backendPort = port
	a.proxy = proxy
	a.mu.Unlock()

	go func() {
		if err := cmd.Wait(); err != nil {
			wailsRuntime.EventsEmit(a.ctx, "backend-exit", err.Error())
			wailsRuntime.MessageDialog(a.ctx, wailsRuntime.MessageDialogOptions{
				Type:    wailsRuntime.ErrorDialog,
				Title:   "0E7 已退出",
				Message: fmt.Sprintf("后端进程已退出: %v", err),
			})
			wailsRuntime.Quit(a.ctx)
		}
	}()

	a.readyOnce.Do(func() {
		close(a.readyCh)
	})

	return nil
}

func ensureUserDataDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("无法获取用户目录: %w", err)
	}
	dataDir := filepath.Join(homeDir, ".0e7")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", fmt.Errorf("无法创建数据目录 %s: %w", dataDir, err)
	}
	return dataDir, nil
}

func (a *DesktopApp) prepareBackendBinary() (string, error) {
	if custom := strings.TrimSpace(os.Getenv("OE7_WAILS_BACKEND")); custom != "" {
		if _, err := os.Stat(custom); err == nil {
			return custom, nil
		}
		return "", fmt.Errorf("指定的 OE7_WAILS_BACKEND 不存在: %s", custom)
	}

	if len(embeddedBackend) == 0 {
		return "", errors.New("未找到嵌入的 0E7 后端，请先运行 build-wails.sh")
	}

	userDir, err := ensureUserDataDir()
	if err != nil {
		return "", err
	}

	binDir := filepath.Join(userDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", fmt.Errorf("无法创建 bin 目录: %w", err)
	}

	ext := ""
	if goRuntime.GOOS == "windows" {
		ext = ".exe"
	}
	target := filepath.Join(binDir, fmt.Sprintf("0e7_%s_%s%s", goRuntime.GOOS, goRuntime.GOARCH, ext))
	if err := os.WriteFile(target, embeddedBackend, 0o755); err != nil {
		return "", fmt.Errorf("写入后端文件失败: %w", err)
	}
	return target, nil
}

func findFreePort() (int, error) {
	for i := 0; i < backendPortRetryCount; i++ {
		port := backendMinPort + randInt(backendMaxPort-backendMinPort)
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			continue
		}
		_ = ln.Close()
		return port, nil
	}
	return 0, errors.New("无法找到可用端口")
}

func randInt(max int) int {
	if max <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max+1)))
	if err != nil {
		return max / 2
	}
	return int(n.Int64())
}

func waitForBackend(port int) error {
	deadline := time.Now().Add(backendReadyTimeout)
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := http.Client{Timeout: 3 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(backendReadyInterval)
	}
	return fmt.Errorf("在 %s 内未能连通 %s", backendReadyTimeout, url)
}

func dialWebSocket(wsURL string, headers http.Header) (*websocket.Conn, *http.Response, error) {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	backendHeaders := make(http.Header)
	for k, v := range headers {
		lowerKey := strings.ToLower(k)
		if lowerKey == "connection" || lowerKey == "upgrade" || lowerKey == "sec-websocket-key" ||
			lowerKey == "sec-websocket-version" || lowerKey == "sec-websocket-extensions" ||
			lowerKey == "sec-websocket-protocol" || lowerKey == "origin" {
			continue
		}
		backendHeaders[k] = v
	}

	return dialer.Dial(wsURL, backendHeaders)
}
