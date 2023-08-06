package update

import (
	"fmt"
	"github.com/gin-gonic/gin"
)

func Register(router *gin.Engine) {
	router.POST("/api/update", func(c *gin.Context) {
		var fileName string
		platform := c.PostForm("platform")
		if platform == "windows" {
			fileName = "0e7.exe"
		} else {
			fmt.Println(platform)
			c.JSON(400, gin.H{
				"message": "fail",
				"err":     "platform not support",
			})
			c.Abort()
		}
		c.Header("Content-Disposition", "attachment; filename="+fileName)
		c.Header("Content-Type", "application/octet-stream")
		c.File(fileName)
	})
}
