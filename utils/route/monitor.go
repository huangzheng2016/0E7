package route

import (
	"0E7/utils/config"
	"0E7/utils/database"

	"github.com/gin-gonic/gin"
)

func monitor(c *gin.Context) {
	uuid := c.PostForm("uuid")
	if uuid == "" {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "missing parameters",
			"result":  "",
		})
		c.Abort()
		return
	}
	var monitors []database.Monitor
	err := config.Db.Where("uuid = ?", uuid).Find(&monitors).Error
	if err != nil {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   err.Error(),
			"result":  []interface{}{},
		})
		return
	}

	var ret []map[string]interface{}
	found := false
	for _, monitor := range monitors {
		element := map[string]interface{}{
			"id":       monitor.ID,
			"types":    monitor.Types,
			"data":     monitor.Data,
			"interval": monitor.Interval,
		}
		ret = append(ret, element)
		found = true
	}
	if found == false {
		c.JSON(202, gin.H{
			"message": "success",
			"error":   "",
			"result":  []interface{}{},
		})
		return
	}
	c.JSON(200, gin.H{
		"message": "success",
		"error":   "",
		"result":  ret,
	})
}
