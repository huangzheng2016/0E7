package webui

import (
	"0E7/service/proxy"
	"net/http"

	"github.com/gin-gonic/gin"
)

func proxy_cache_list(c *gin.Context) {
	list := proxy.ListCacheEntries()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    list,
	})
}


