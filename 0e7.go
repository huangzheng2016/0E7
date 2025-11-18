package main

import (
	"0E7/service/client"
	"0E7/service/config"
	flagService "0E7/service/flag"
	"0E7/service/git"
	"0E7/service/pcap"
	"0E7/service/proxy"
	"0E7/service/route"
	"0E7/service/search"
	"0E7/service/server"
	"0E7/service/udpcast"
	"0E7/service/update"
	"0E7/service/webui"
	"0E7/service/windows"
	"archive/tar"
	"bytes"
	gzipcompress "compress/gzip"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

var (
	err error

	cleanupOnce sync.Once
	cleanupMu   sync.Mutex
	cleanupFns  []func()
)

//go:embed dist
var f embed.FS

// registerCleanup 注册一个在程序退出时执行的清理函数
func registerCleanup(fn func()) {
	if fn == nil {
		return
	}
	cleanupMu.Lock()
	cleanupFns = append(cleanupFns, fn)
	cleanupMu.Unlock()
}

// runCleanup 按注册顺序逆序执行所有清理函数，仅执行一次
func runCleanup() {
	cleanupOnce.Do(func() {
		cleanupMu.Lock()
		funcs := make([]func(), len(cleanupFns))
		copy(funcs, cleanupFns)
		cleanupFns = nil
		cleanupMu.Unlock()

		for i := len(funcs) - 1; i >= 0; i-- {
			if funcs[i] == nil {
				continue
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("执行清理函数时发生异常: %v", r)
					}
				}()
				funcs[i]()
			}()
		}
	})
}

// archiveLogFile 归档日志文件：创建 log 文件夹，将日志文件压缩为 tar.gz
func archiveLogFile(logFilePath string) error {
	// 检查日志文件是否存在且有内容
	info, err := os.Stat(logFilePath)
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		// 文件为空，不需要归档
		return nil
	}

	// 创建 log 文件夹
	logDir := "log"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建 log 文件夹失败: %v", err)
	}

	// 生成时间戳文件名：年份时间到毫秒，格式：2025-11-18_12-30-45.123.tar.gz
	now := time.Now()
	timestamp := fmt.Sprintf("%04d-%02d-%02d_%02d-%02d-%02d.%03d",
		now.Year(), now.Month(), now.Day(),
		now.Hour(), now.Minute(), now.Second(),
		now.Nanosecond()/1000000)
	archiveName := fmt.Sprintf("%s.tar.gz", timestamp)
	archivePath := filepath.Join(logDir, archiveName)

	// 打开日志文件
	logFile, err := os.Open(logFilePath)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %v", err)
	}
	defer logFile.Close()

	// 创建 tar.gz 文件
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("创建归档文件失败: %v", err)
	}
	defer archiveFile.Close()

	// 创建 gzip writer
	gzipWriter := gzipcompress.NewWriter(archiveFile)
	defer gzipWriter.Close()

	// 创建 tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// 获取文件信息
	fileInfo, err := logFile.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %v", err)
	}

	// 创建 tar header
	header := &tar.Header{
		Name:    filepath.Base(logFilePath),
		Size:    fileInfo.Size(),
		Mode:    int64(fileInfo.Mode()),
		ModTime: fileInfo.ModTime(),
	}

	// 写入 header
	if err := tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("写入 tar header 失败: %v", err)
	}

	// 复制文件内容到 tar
	if _, err := io.Copy(tarWriter, logFile); err != nil {
		return fmt.Errorf("复制文件内容失败: %v", err)
	}

	return nil
}

func main() {

	fmt.Print("  ___   _____    _____  ____                            _  _\n" +
		" / _ \\ | ____||___  |  / ___|   ___   ___  _   _  _ __ (_)| |_  _   _\n" +
		"| | | ||  _|     / /   \\___ \\  / _ \\ / __|| | | || '__|| || __|| | | |\n" +
		"| |_| || |___   / /     ___) ||  __/| (__ | |_| || |   | || |_ | |_| |\n" +
		" \\___/ |_____| /_/     |____/  \\___| \\___| \\__,_||_|   |_| \\__| \\__, |\n" +
		"                                                              |___/\n\n")

	log.Println("0E7 For Security")

	defer runCleanup()

	// 定义命令行参数
	var (
		configFile   = flag.String("config", "config.ini", "指定配置文件路径")
		serverMode   = flag.Bool("server", false, "以服务器模式启动")
		serverPort   = flag.String("server-port", "", "在运行时覆盖配置文件中的 server.port")
		help         = flag.Bool("help", false, "显示帮助信息")
		installGuide = flag.Bool("install-guide", false, "显示Windows依赖安装指南")
		cpuProfile   = flag.String("cpu-profile", "", "启用CPU性能分析并将结果写入指定文件")
		memProfile   = flag.String("mem-profile", "", "启用内存性能分析并将结果写入指定文件")
		logFile      = flag.String("log-file", "0e7.log", "指定日志文件路径")
	)

	// 支持短参数
	flag.BoolVar(serverMode, "s", false, "以服务器模式启动（等同于 --server）")
	flag.BoolVar(help, "h", false, "显示帮助信息（等同于 --help）")
	flag.StringVar(serverPort, "p", "", "以短参数形式覆盖 server.port（等同于 --server-port）")

	// 解析命令行参数
	flag.Parse()
	envServerPort := os.Getenv("OE7_SERVER_PORT")

	// 初始化性能分析
	if *cpuProfile != "" {
		cpuFile, err := os.Create(*cpuProfile)
		if err != nil {
			log.Printf("无法创建CPU性能分析文件 %s: %v", *cpuProfile, err)
			runCleanup()
			os.Exit(1)
		}
		if err := pprof.StartCPUProfile(cpuFile); err != nil {
			log.Printf("启动CPU性能分析失败: %v", err)
			_ = cpuFile.Close()
			runCleanup()
			os.Exit(1)
		}
		registerCleanup(func() {
			pprof.StopCPUProfile()
			if err := cpuFile.Close(); err != nil {
				log.Printf("关闭CPU性能分析文件失败: %v", err)
			}
		})
		log.Printf("CPU性能分析已启用，输出文件: %s", *cpuProfile)
	}

	if *memProfile != "" {
		memPath := *memProfile
		memFile, err := os.Create(memPath)
		if err != nil {
			log.Printf("无法创建内存性能分析文件 %s: %v", memPath, err)
			runCleanup()
			os.Exit(1)
		}
		if err := memFile.Close(); err != nil {
			log.Printf("关闭内存性能分析文件失败 %s: %v", memPath, err)
			runCleanup()
			os.Exit(1)
		}

		registerCleanup(func() {
			f, err := os.Create(memPath)
			if err != nil {
				log.Printf("无法写入内存性能分析文件 %s: %v", memPath, err)
				return
			}
			defer func() {
				if err := f.Close(); err != nil {
					log.Printf("关闭内存性能分析文件失败 %s: %v", memPath, err)
				}
			}()
			runtime.GC()
			if err := pprof.WriteHeapProfile(f); err != nil {
				log.Printf("写入内存性能分析文件失败 %s: %v", memPath, err)
			}
		})
		log.Printf("内存性能分析将在退出时写入: %s", memPath)
	}

	// 处理帮助信息
	if *help {
		showHelp()
		return
	}

	// 处理Windows安装指南
	if *installGuide {
		if runtime.GOOS == "windows" {
			fmt.Println(windows.GetInstallationGuide())
		} else {
			fmt.Println("此功能仅在Windows上可用")
		}
		return
	}

	// 处理服务器模式
	if *serverMode {
		// 服务器模式：检查并生成配置文件
		if err := ensureServerConfig(*configFile); err != nil {
			log.Printf("配置文件处理失败: %v", err)
			runCleanup()
			os.Exit(1)
		}
	}

	// 在Windows下进行依赖检查
	if err := windows.CheckWindowsDependencies(); err != nil {
		log.Printf("Windows依赖检查完成: %v", err)
	}

	err = config.Init_conf(*configFile)
	if err != nil {
		log.Printf("Config load error from %s: %v", *configFile, err)
	}

	overridePort := ""
	if serverPort != nil && *serverPort != "" {
		overridePort = *serverPort
	} else if envServerPort != "" {
		overridePort = envServerPort
	}
	if overridePort != "" {
		config.Server_port = overridePort
		if config.Server_mode {
			if config.Server_tls {
				config.Server_url = "https://127.0.0.1:" + config.Server_port
			} else {
				config.Server_url = "http://127.0.0.1:" + config.Server_port
			}
		}
		log.Printf("Server port overridden to %s", config.Server_port)
	}

	// 日志归档：在启动前压缩现有日志文件
	if _, err := os.Stat(*logFile); err == nil {
		// 日志文件存在，进行归档
		if err := archiveLogFile(*logFile); err != nil {
			// 归档失败不影响启动，只记录错误
			fmt.Printf("警告: 日志归档失败: %v\n", err)
		}
	}

	file, err := os.OpenFile(*logFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		log.Printf("日志文件初始化失败: %v", err)
		runCleanup()
		os.Exit(1)
	}
	defer file.Close()
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	if config.Global_debug {
		multiWriter := io.MultiWriter(os.Stdout, file)
		// 使用日志拦截Writer，将日志同时发送到WebSocket广播器
		logWriter := webui.NewLogWriter(multiWriter)
		log.SetOutput(logWriter)
	} else {
		// 使用日志拦截Writer，将日志同时发送到WebSocket广播器
		logWriter := webui.NewLogWriter(file)
		log.SetOutput(logWriter)
	}

	update.InitUpdate()
	update.CheckStatus()

	if config.Server_mode {
		if config.Global_debug {
			gin.SetMode(gin.DebugMode)
		} else {
			gin.SetMode(gin.ReleaseMode)
		}

		// 统一 Gin 日志到标准日志输出，并自定义格式
		gin.DisableConsoleColor()
		gin.DefaultWriter = log.Writer()
		gin.DefaultErrorWriter = log.Writer()

		r_server := gin.New()

		r_server.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
			// 统一格式: 时间 | 状态码 | 耗时 | 客户端IP | 方法 路径 | 错误
			// 使用紧凑格式，避免过多空格
			return fmt.Sprintf("%s | %d | %v | %s | %s %s | %s\n",
				param.TimeStamp.Format("2006/01/02 15:04:05"),
				param.StatusCode,
				param.Latency,
				param.ClientIP,
				param.Method,
				param.Path,
				param.ErrorMessage,
			)
		}))
		r_server.Use(gin.Recovery())
		r_server.Use(etagMiddleware())
		r_server.Use(gzip.Gzip(gzip.DefaultCompression))

		log.Printf("Server listening on port: %s", config.Server_port)

		// 检查 Git 命令（Git 服务需要）
		git.CheckAndWarnGit()

		route.Register(r_server)
		webui.Register(r_server)
		update.Register(r_server)
		server.Register(r_server)
		git.Register(r_server)
		proxy.RegisterRoutes(r_server)

		// 启动flag检测器
		_ = flagService.GetFlagDetector()
		log.Println("Flag检测器已启动")

		fp, _ := fs.Sub(f, "dist")
		fpStatic, _ := fs.Sub(f, "dist/static")

		// 静态资源处理器，带缓存头
		r_server.GET("/static/*filepath", func(c *gin.Context) {
			path := strings.TrimPrefix(c.Param("filepath"), "/")
			serveStaticFile(c, fpStatic, path, true)
		})

		// 根路径返回 index.html，带缓存头
		r_server.GET("/", func(c *gin.Context) {
			serveStaticFile(c, fp, "index.html", false)
		})

		if config.Server_tls {
			r_server.RedirectTrailingSlash = true
			r_server.RedirectFixedPath = true
			log.Printf("Starting TLS server on port: %s", config.Server_port)
			go func() {
				if err := r_server.RunTLS(":"+config.Server_port, "cert/certificate.crt", "cert/private.key"); err != nil {
					log.Printf("Failed to start TLS server on port %s: %v", config.Server_port, err)
					runCleanup()
					os.Exit(1)
				}
			}()
		} else {
			log.Printf("Starting HTTP server on port: %s", config.Server_port)
			go func() {
				if err := r_server.Run(":" + config.Server_port); err != nil {
					log.Printf("Failed to start HTTP server on port %s: %v", config.Server_port, err)
					runCleanup()
					os.Exit(1)
				}
			}()
		}
		go udpcast.Udp_sent(config.Server_tls, config.Server_port)

		pcap.SetFlagRegex(config.Server_flag)

		pcap.InitPcapQueue()

		go pcap.WatchDir("pcap")
	}

	// 客户端独立代理（当未启用服务端时才生效）
	if !config.Server_mode && config.Client_mode && config.Client_proxy_enable {
		if config.Global_debug {
			gin.SetMode(gin.DebugMode)
		} else {
			gin.SetMode(gin.ReleaseMode)
		}
		rClientProxy := gin.New()
		rClientProxy.Use(gin.Recovery())
		proxy.RegisterRoutes(rClientProxy)
		log.Printf("Client proxy listening on port: %s", config.Client_proxy_port)
		go func() {
			if err := rClientProxy.Run(":" + config.Client_proxy_port); err != nil {
				log.Printf("client proxy server stopped: %v", err)
			}
		}()
	}

	if config.Client_mode {
		client.Register()
	}

	if config.Client_mode || config.Server_mode {
		// 设置信号处理，确保程序退出时正确关闭资源
		setupGracefulShutdown()
		select {}
	} else {
		log.Println("Configuration file error, please check")
	}
}

// showHelp 显示帮助信息
func showHelp() {
	fmt.Println("0E7 - AWD攻防演练工具箱")
	fmt.Println("")
	fmt.Println("用法:")
	fmt.Println("  0e7                           # 正常启动（使用默认配置文件）")
	fmt.Println("  0e7 -config <file>            # 指定配置文件路径")
	fmt.Println("  0e7 --server, -s              # 服务器模式启动")
	fmt.Println("  0e7 --server -config <file>   # 服务器模式启动并指定配置文件")
	fmt.Println("  0e7 --server-port 6200        # 覆盖配置文件中的 server.port")
	fmt.Println("  0e7 --cpu-profile cpu.prof    # 启用CPU性能分析输出文件")
	fmt.Println("  0e7 --mem-profile mem.prof    # 启用内存性能分析输出文件")
	fmt.Println("  0e7 --log-file custom.log    # 指定日志文件路径")
	fmt.Println("  0e7 --help, -h                # 显示帮助信息")
	fmt.Println("  0e7 --install-guide           # 显示Windows依赖安装指南")
	fmt.Println("")
	fmt.Println("参数说明:")
	fmt.Println("  -config, --config <file>      指定配置文件路径（默认: config.ini）")
	fmt.Println("  --server, -s                  以服务器模式启动")
	fmt.Println("  --help, -h                    显示帮助信息")
	fmt.Println("  --server-port, -p <port>      运行时覆盖 server.port（优先级高于配置）")
	fmt.Println("  --install-guide               显示Windows依赖安装指南")
	fmt.Println("  --cpu-profile <file>          启用CPU性能分析并写入指定文件")
	fmt.Println("  --mem-profile <file>          启用内存性能分析并在退出时写入指定文件")
	fmt.Println("  --log-file <file>             指定日志文件路径（默认: 0e7.log）")
	fmt.Println("")
}

// ensureServerConfig 确保服务器配置文件存在，如果不存在则生成默认配置
func ensureServerConfig(configFile string) error {

	// 检查配置文件是否存在
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		log.Printf("配置文件 %s 不存在，正在生成默认配置...", configFile)

		// 生成默认服务器配置
		defaultConfig := `[global]
timeout_http     = 5
timeout_download = 60
debug            = true

[client]
enable     = true
id         = 
name       = 
server_url = http://remotehost:6102
pypi       = https://pypi.tuna.tsinghua.edu.cn/simple
update     = false
worker     = 20
monitor    = true
only_monitor = false
pcap_workers = 0

[server]
enable      = true
port        = 6102
db_engine   = sqlite3
db_host     = localhost
db_port     = 3306
db_username = 
db_password = 
db_tables   = 
server_url  = http://localhost:6102
flag        = flag{.*}
tls         = false
pcap_zip    = false

[search]
search_engine                 = bleve
search_elasticsearch_url      = http://localhost:9200
search_elasticsearch_username = 
search_elasticsearch_password = 
`

		// 写入配置文件
		if err := os.WriteFile(configFile, []byte(defaultConfig), 0644); err != nil {
			return fmt.Errorf("无法创建配置文件: %v", err)
		}

		log.Printf("成功生成默认配置文件: %s", configFile)
		log.Println("提示: 您可以根据需要修改配置文件中的设置")
	} else {
		log.Printf("配置文件 %s 已存在", configFile)
	}

	return nil
}

// responseWriter 包装 gin.ResponseWriter 以捕获响应体
type responseWriter struct {
	gin.ResponseWriter
	body    *bytes.Buffer
	written bool
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.body.Write(b)
		return len(b), nil // 先不写入，等待计算 ETag
	}
	return w.ResponseWriter.Write(b)
}

func (w *responseWriter) WriteString(s string) (int, error) {
	if !w.written {
		w.body.WriteString(s)
		return len(s), nil // 先不写入，等待计算 ETag
	}
	return w.ResponseWriter.WriteString(s)
}

func (w *responseWriter) WriteHeader(code int) {
	// 保存状态码，但不立即写入
	w.ResponseWriter.WriteHeader(code)
}

// etagMiddleware 为 API 响应添加 ETag 支持
func etagMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 跳过静态文件和 index.html（它们已有专门的处理器）
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/static/") || path == "/" {
			c.Next()
			return
		}

		// 包装响应写入器以捕获响应体
		writer := &responseWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = writer

		// 继续处理请求
		c.Next()

		// 获取状态码
		statusCode := writer.ResponseWriter.Status()
		if statusCode == 0 {
			// 如果没有设置状态码，默认使用 200
			statusCode = 200
		}

		// 只对成功的响应添加 ETag（2xx 状态码）
		if statusCode >= 200 && statusCode < 300 {
			// 生成 ETag（基于响应体，即使是空响应也生成）
			bodyBytes := writer.body.Bytes()
			hash := sha256.Sum256(bodyBytes)
			hashStr := hex.EncodeToString(hash[:16])
			etag := fmt.Sprintf(`"%s"`, hashStr)

			// 检查 If-None-Match 请求头（条件请求）
			if match := c.GetHeader("If-None-Match"); match != "" {
				matchCleaned := strings.ReplaceAll(strings.ReplaceAll(match, `"`, ""), " ", "")
				if strings.Contains(matchCleaned, hashStr) {
					// 匹配，返回 304 Not Modified
					writer.ResponseWriter.Header().Set("ETag", etag)
					writer.ResponseWriter.Header().Del("Content-Length") // 删除可能存在的 Content-Length
					writer.ResponseWriter.WriteHeader(http.StatusNotModified)
					return
				}
			}

			// 设置 ETag 和缓存控制头
			writer.ResponseWriter.Header().Set("ETag", etag)
			// 删除可能已经设置的 Content-Length，让后续的 gzip 中间件根据压缩后的数据设置
			writer.ResponseWriter.Header().Del("Content-Length")
			if c.Request.Method == "GET" || c.Request.Method == "HEAD" {
				writer.ResponseWriter.Header().Set("Cache-Control", "no-cache, must-revalidate")
			} else {
				writer.ResponseWriter.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
			}
		}

		// 写入响应体（原始未压缩内容）
		// Content-Length 会在后续的 gzip 中间件中根据压缩后的数据自动设置
		if writer.body.Len() > 0 {
			writer.written = true
			writer.ResponseWriter.Write(writer.body.Bytes())
		}

		// 恢复原始写入器
		c.Writer = writer.ResponseWriter
	}
}

// serveStaticFile 提供静态文件服务，带简单缓存
func serveStaticFile(c *gin.Context, fileSystem fs.FS, path string, isStaticResource bool) {
	// 读取文件
	data, err := fs.ReadFile(fileSystem, path)
	if err != nil {
		c.String(http.StatusNotFound, "file not found")
		return
	}

	// 设置缓存策略
	if isStaticResource {
		// 静态资源（JS/CSS等）：1年缓存
		c.Header("Cache-Control", "public, max-age=31536000")
	} else {
		// index.html：不缓存
		c.Header("Cache-Control", "no-cache")
	}

	// 设置 Content-Type
	contentType := http.DetectContentType(data)
	if path == "index.html" || strings.HasSuffix(path, ".html") {
		contentType = "text/html; charset=utf-8"
	} else if strings.HasSuffix(path, ".js") {
		contentType = "application/javascript; charset=utf-8"
	} else if strings.HasSuffix(path, ".css") {
		contentType = "text/css; charset=utf-8"
	} else if strings.HasSuffix(path, ".json") {
		contentType = "application/json; charset=utf-8"
	} else if strings.HasSuffix(path, ".ico") {
		contentType = "image/x-icon"
	}

	c.Data(http.StatusOK, contentType, data)
}

// setupGracefulShutdown 设置优雅关闭处理
func setupGracefulShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Println("收到退出信号，正在优雅关闭...")

		// 关闭搜索服务
		if searchService := search.GetSearchService(); searchService != nil {
			if err := searchService.Close(); err != nil {
				log.Printf("关闭搜索服务失败: %v", err)
			} else {
				log.Println("搜索服务已关闭")
			}
		}

		runCleanup()
		log.Println("程序已优雅退出")
		os.Exit(0)
	}()
}
