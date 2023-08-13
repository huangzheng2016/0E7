package main

import (
	"0E7/utils/client"
	"0E7/utils/config"
	"0E7/utils/route"
	"0E7/utils/update"
	"0E7/utils/webui"
	"fmt"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

var err error

func init() {
	err = config.Init_conf()
	if err != nil {
		fmt.Println(err)
	}
	update.Init_update()
}

func main() {
	if config.Server_mode {
		r_server := gin.Default()
		r_server.LoadHTMLGlob("template/*")
		fmt.Println("host listening on port ", config.Server_port)
		route.Register(r_server)
		webui.Register(r_server)
		update.Register(r_server)
		go r_server.Run(":" + config.Server_port)
	}

	if config.Client_mode {
		client.Register()
	}

	select {}
}
