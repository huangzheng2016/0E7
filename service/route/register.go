package route

import (
	"sync"

	"github.com/gin-gonic/gin"
)

var exploit_bucket sync.Map
var exploit_mutex sync.Mutex

func Register(router *gin.Engine) {
	router.POST("/api/heartbeat", heartbeat)
	router.POST("/api/exploit", exploit)
	router.POST("/api/exploit_download", exploit_download)
	router.POST("/api/exploit_output", exploit_output)
	router.POST("/api/flag", flag)
	router.POST("/api/monitor", monitor)

	// 终端管理相关API
	router.POST("/api/clients", getClients)
	router.POST("/api/traffic_collection", createTrafficCollection)
	router.POST("/api/client_monitors", getClientMonitors)
	router.POST("/api/delete_monitor", deleteMonitor)
}
