package webui

import (
	"github.com/gin-gonic/gin"
)

func Register(router *gin.Engine) {
	router.GET("/webui/exploit", exploit)
	router.POST("/webui/exploit", exploit)

	router.POST("/webui/exploit/rename", exploit_rename)

	router.Static("/assets", "./frontend/dist/assets")
	router.Static("/js", "./frontend/dist/js")
	router.StaticFile("/", "./frontend/dist/index.html")
}
