package update

import (
	"github.com/gin-gonic/gin"
)

func Register(router *gin.Engine) {
	router.POST("/api/update", func(c *gin.Context) {
		platform := c.PostForm("platform")
		arch := c.PostForm("arch")
		fileName := "0e7_" + platform + "_" + arch
		if platform == "windows" {
			fileName += ".exe"
		}
		c.Header("Content-Disposition", "attachment; filename="+fileName)
		c.Header("Content-Type", "application/octet-stream")
		c.File(fileName)
	})
}
