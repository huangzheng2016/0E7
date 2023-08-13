package route

import (
	"github.com/gin-gonic/gin"
)

func Register(router *gin.Engine) {
	router.POST("/api/heartbeat", heartbeat)
	router.POST("/api/exploit", exploit)
	router.POST("/api/exploit_download", exploit_download)
	router.POST("/api/exploit_output", exploit_output)
	router.POST("/api/exploit_show_output", exploit_show_output)
	router.POST("/api/flag", flag)
}
