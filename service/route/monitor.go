package route

import (
	"0E7/service/config"
	"0E7/service/database"
	"log"
	"strconv"

	"github.com/gin-gonic/gin"
)

func monitor(c *gin.Context) {
	client_id_str := c.PostForm("client_id")
	if client_id_str == "" {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "missing parameters",
			"result":  "",
		})
		c.Abort()
		return
	}

	// 转换uuid为int
	client_id, err := strconv.Atoi(client_id_str)
	if err != nil {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "invalid uuid: " + err.Error(),
			"result":  "",
		})
		log.Println("Invalid uuid:", err)
		c.Abort()
		return
	}

	var monitors []database.Monitor
	err = config.Db.Where("client_id = ?", client_id).Find(&monitors).Error
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
	if !found {
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
