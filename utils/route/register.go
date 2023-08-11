package route

import (
	"0E7/utils/config"
	"github.com/gin-gonic/gin"
)

var conf config.Conf

func Register(sconf config.Conf, router *gin.Engine) {
	conf = sconf
	router.POST("/api/heartbeat", heartbeat)
	router.POST("/api/exploit", exploit)
	router.POST("/api/exploit_download", exploit_download)
}
