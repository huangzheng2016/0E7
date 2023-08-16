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
)

var err error

func init() {
	err = config.Init_conf()
	if err != nil {
		fmt.Println(err)
	}
	update.InitUpdate()
}

func main() {
	update.CheckStatus()

	if config.Server_mode {
		r_server := gin.Default()
		r_server.Use(gin.Recovery())
		r_server.Use(gzip.Gzip(gzip.DefaultCompression))
		fmt.Println("host listening on port ", config.Server_port)
		route.Register(r_server)
		webui.Register(r_server)
		update.Register(r_server)
		go r_server.Run(":" + config.Server_port)
		go udpcast.Udp_sent(config.Server_port)
	}

	if config.Client_mode {
		client.Register()
	}

	select {}
}
