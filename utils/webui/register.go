package webui

import (
	_ "embed"

	"github.com/gin-gonic/gin"
	"github.com/traefik/yaegi/interp"
)

var programs map[int]*interp.Program

func Register(router *gin.Engine) {
	router.POST("/webui/exploit", exploit)
	router.POST("/webui/exploit_rename", exploit_rename)
	router.POST("/webui/exploit_show_output", exploit_show_output)
	router.POST("/webui/exploit_get_by_uuid", exploit_get_by_uuid)
	router.POST("/webui/action", action)
	router.POST("/webui/action_show", action_show)

	router.POST("/webui/pcap_upload", pcap_upload)
}
