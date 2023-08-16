package webui

import (
	"github.com/gin-gonic/gin"
)

func Register(router *gin.Engine) {
	router.GET("/webui/exploit", exploit)
	router.POST("/webui/exploit", exploit)

	router.POST("/webui/exploit/rename", exploit_rename)

	router.POST("/api/exploit_show_output", exploit_show_output)

	router.Static("/assets", "dist/assets")
	router.Static("/js", "dist/js")
	router.StaticFile("/", "dist/index.html")
}
