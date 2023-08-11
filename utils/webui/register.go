package webui

import (
	"0E7/utils/config"
	"github.com/gin-gonic/gin"
)

var conf config.Conf

func Register(sconf config.Conf, router *gin.Engine) {
	conf = sconf
	router.GET("/webui/exploit", exploit)
	router.POST("/webui/exploit", exploit)

	router.POST("/webui/exploit/rename", exploit_rename)
}
