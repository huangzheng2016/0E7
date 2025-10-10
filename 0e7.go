package main

import (
	"0E7/service/client"
	"0E7/service/config"
	"0E7/service/flag"
	"0E7/service/pcap"
	"0E7/service/route"
	"0E7/service/server"
	"0E7/service/udpcast"
	"0E7/service/update"
	"0E7/service/webui"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"

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
	err = config.Init_conf()
	if err != nil {
		log.Println("Config load error: ", err)
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
		_ = flag.GetFlagDetector()
		log.Println("Flag检测器已启动")

		fp, _ := fs.Sub(f, "dist")
		r_server.StaticFS("/", http.FS(fp))

		if config.Server_tls == true {
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
		select {}
	} else {
		log.Println("Configuration file error, please check")
	}
}
