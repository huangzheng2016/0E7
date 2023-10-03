package webui

import (
	"github.com/gin-gonic/gin"
	"github.com/traefik/yaegi/interp"
)

var programs map[int]*interp.Program

func Register(router *gin.Engine) {
	router.POST("/webui/exploit", exploit)
	router.POST("/webui/exploit_rename", exploit_rename)
	router.POST("/webui/exploit_show_output", exploit_show_output)
	router.POST("/webui/action", action)
	router.POST("/webui/action_show", action_show)

	router.Static("/assets", "dist/assets")
	router.Static("/js", "dist/js")
	router.Static("/css", "dist/css")
	router.StaticFile("/", "dist/index.html")

}
