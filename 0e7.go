package main

import (
	"0E7/service/client"
	"0E7/service/config"
	flagService "0E7/service/flag"
	"0E7/service/pcap"
	"0E7/service/route"
	"0E7/service/search"
	"0E7/service/server"
	"0E7/service/udpcast"
	"0E7/service/update"
	"0E7/service/webui"
	"0E7/service/windows"
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

var err error

//go:embed dist
var f embed.FS

func main() {

	fmt.Print("  ___   _____    _____  ____                            _  _\n" +
		" / _ \\ | ____||___  |  / ___|   ___   ___  _   _  _ __ (_)| |_  _   _\n" +
		"| | | ||  _|     / /   \\___ \\  / _ \\ / __|| | | || '__|| || __|| | | |\n" +
		"| |_| || |___   / /     ___) ||  __/| (__ | |_| || |   | || |_ | |_| |\n" +
		" \\___/ |_____| /_/     |____/  \\___| \\___| \\__,_||_|   |_| \\__| \\__, |\n" +
		"                                                              |___/\n\n")

	log.Println("0E7 For Security")

	// 定义命令行参数
	var (
		configFile   = flag.String("config", "config.ini", "指定配置文件路径")
		serverMode   = flag.Bool("server", false, "以服务器模式启动")
		help         = flag.Bool("help", false, "显示帮助信息")
		installGuide = flag.Bool("install-guide", false, "显示Windows依赖安装指南")
	)

	// 支持短参数
	flag.BoolVar(serverMode, "s", false, "以服务器模式启动（等同于 --server）")
	flag.BoolVar(help, "h", false, "显示帮助信息（等同于 --help）")

	// 解析命令行参数
	flag.Parse()

	// 处理帮助信息
	if *help {
		showHelp()
		os.Exit(0)
	}

	// 处理Windows安装指南
	if *installGuide {
		if runtime.GOOS == "windows" {
			fmt.Println(windows.GetInstallationGuide())
		} else {
			fmt.Println("此功能仅在Windows上可用")
		}
		os.Exit(0)
	}

	// 处理服务器模式
	if *serverMode {
		// 服务器模式：检查并生成配置文件
		if err := ensureServerConfig(*configFile); err != nil {
			log.Printf("配置文件处理失败: %v", err)
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

	file, err := os.OpenFile("0e7.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	if config.Global_debug {
		multiWriter := io.MultiWriter(os.Stdout, file)
		log.SetOutput(multiWriter)
	} else {
		log.SetOutput(file)
	}

	update.InitUpdate()
	update.CheckStatus()

	if config.Server_mode {
		if config.Global_debug {
			gin.SetMode(gin.DebugMode)
		} else {
			gin.SetMode(gin.ReleaseMode)
		}
		r_server := gin.Default()

		r_server.Use(gin.Recovery())
		r_server.Use(gzip.Gzip(gzip.DefaultCompression))

		log.Printf("Server listening on port: %s", config.Server_port)

		route.Register(r_server)
		webui.Register(r_server)
		update.Register(r_server)
		server.Register(r_server)

		// 启动flag检测器
		_ = flagService.GetFlagDetector()
		log.Println("Flag检测器已启动")

		fp, _ := fs.Sub(f, "dist")
		r_server.StaticFS("/", http.FS(fp))

		if config.Server_tls {
			r_server.RedirectTrailingSlash = true
			r_server.RedirectFixedPath = true
			log.Printf("Starting TLS server on port: %s", config.Server_port)
			go func() {
				if err := r_server.RunTLS(":"+config.Server_port, "cert/certificate.crt", "cert/private.key"); err != nil {
					log.Fatalf("Failed to start TLS server on port %s: %v", config.Server_port, err)
				}
			}()
		} else {
			log.Printf("Starting HTTP server on port: %s", config.Server_port)
			go func() {
				if err := r_server.Run(":" + config.Server_port); err != nil {
					log.Fatalf("Failed to start HTTP server on port %s: %v", config.Server_port, err)
				}
			}()
		}
		go udpcast.Udp_sent(config.Server_tls, config.Server_port)

		pcap.SetFlagRegex(config.Server_flag)
		go pcap.WatchDir("pcap")
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
	fmt.Println("  0e7 --help, -h                # 显示帮助信息")
	fmt.Println("  0e7 --install-guide           # 显示Windows依赖安装指南")
	fmt.Println("")
	fmt.Println("参数说明:")
	fmt.Println("  -config, --config <file>      指定配置文件路径（默认: config.ini）")
	fmt.Println("  --server, -s                  以服务器模式启动")
	fmt.Println("  --help, -h                    显示帮助信息")
	fmt.Println("  --install-guide               显示Windows依赖安装指南")
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
update     = true
worker     = 5
monitor    = false

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

		log.Println("程序已优雅退出")
		os.Exit(0)
	}()
}
