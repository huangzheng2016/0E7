package update

import (
	"github.com/gin-gonic/gin"
)

var (
	allowedPlatforms = map[string]bool{
		"darwin":  true,
		"linux":   true,
		"windows": true,
	}
	allowedArchs = map[string]bool{
		"amd64": true,
		"arm64": true,
		"386":   true,
	}
)

func Register(router *gin.Engine) {
	router.POST("/api/update", func(c *gin.Context) {
		platform := c.PostForm("platform")
		arch := c.PostForm("arch")

		// 验证平台和架构是否在白名单中
		if !allowedPlatforms[platform] || !allowedArchs[arch] {
			c.JSON(400, gin.H{"error": "invalid platform or arch"})
			return
		}

		fileName := "0e7_" + platform + "_" + arch
		if platform == "windows" {
			fileName += ".exe"
		}

		c.Header("Content-Disposition", "attachment; filename="+fileName)
		c.Header("Content-Type", "application/octet-stream")
		c.File(fileName)
	})
}
