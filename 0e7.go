package main

import (
	"0E7/utils/client"
	"0E7/utils/config"
	"0E7/utils/route"
	"0E7/utils/udpcast"
	"0E7/utils/update"
	"0E7/utils/webui"
	"fmt"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"os"
)

var err error

func main() {

	file, err := os.OpenFile("0e7.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
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
	fmt.Println(
		"  ___   _____  _____    __                ____                            _  _\n" +
			"/ _ \\ | ____||___  |  / _|  ___   _ __  / ___|   ___   ___  _   _  _ __ (_)| |_  _   _\n" +
			"| | | ||  _|     / /  | |_  / _ \\ | '__| \\___ \\  / _ \\ / __|| | | || '__|| || __|| | | |\n" +
			"| |_| || |___   / /   |  _|| (_) || |     ___) ||  __/| (__ | |_| || |   | || |_ | |_| |\n" +
			" \\___/ |_____| /_/    |_|   \\___/ |_|    |____/  \\___| \\___| \\__,_||_|   |_| \\__| \\__, |\n" +
			"                                                                                  |___/")

	log.Println("0E7 For Security")
	err = config.Init_conf()
	if err != nil {
		log.Println("Config load error: ", err)
	}
	update.InitUpdate()
	update.CheckStatus()

	if config.Server_mode {
		if !config.Global_debug {
			gin.SetMode(gin.ReleaseMode)
		}
		r_server := gin.Default()

		r_server.Use(gin.Recovery())
		r_server.Use(gzip.Gzip(gzip.DefaultCompression))

		log.Println("Server listening on port ", config.Server_port)

		route.Register(r_server)
		webui.Register(r_server)
		update.Register(r_server)

		if config.Server_tls == true {
			r_server.RedirectTrailingSlash = true
			r_server.RedirectFixedPath = true
			go r_server.RunTLS(":"+config.Server_port, "cert/certificate.crt", "cert/private.key")
		} else {
			go r_server.Run(":" + config.Server_port)
		}
		go udpcast.Udp_sent(config.Server_tls, config.Server_port)
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
