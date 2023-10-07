package route

import (
	"0E7/utils/config"
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
	rows, err := config.Db.Query("SELECT id,types,data,interval FROM `0e7_monitor` WHERE uuid=?", uuid)
	if err != nil {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   err.Error(),
			"result":  []interface{}{},
		})
		return
	}
	defer rows.Close()

	var ret []map[string]interface{}
	found := false
	for rows.Next() {
		var id int
		var types, data, interval string
		err := rows.Scan(&id, &types, &data, &interval)
		if err != nil {
			c.JSON(400, gin.H{
				"message": "fail",
				"error":   err.Error(),
				"result":  []interface{}{},
			})
			return
		}
		element := map[string]interface{}{
			"id":       id,
			"types":    types,
			"data":     data,
			"interval": interval,
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
