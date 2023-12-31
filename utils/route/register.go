package route

import (
	"github.com/gin-gonic/gin"
)

var exploit_bucket map[string]string

func Register(router *gin.Engine) {
	router.POST("/api/heartbeat", heartbeat)
	router.POST("/api/exploit", exploit)
	router.POST("/api/exploit_download", exploit_download)
	router.POST("/api/exploit_output", exploit_output)
	router.POST("/api/flag", flag)
	router.POST("/api/monitor", monitor)

	exploit_bucket = make(map[string]string)
}
