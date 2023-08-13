package route

import (
	"0E7/utils/config"
	"fmt"
	"github.com/gin-gonic/gin"
	"time"
)

func flag(c *gin.Context) {
	exploit_uuid := c.PostForm("uuid")
	exploit_flag := c.PostForm("flag")
	updated := time.Now().Format(time.DateTime)
	var count int
	err := config.Db.QueryRow("SELECT COUNT(*) FROM `0e7_flag` WHERE flag=?", exploit_flag).Scan(&count)
	if err != nil {
		fmt.Println("Failed to query database:", err)
		c.JSON(400, gin.H{
			"message": "fail",
		})
		return
	}
	if count == 0 {
		_, err = config.Db.Exec("INSERT INTO `0e7_flag` (uuid,flag,updated) VALUES (?,?,?)", exploit_uuid, exploit_flag, updated)
		c.JSON(200, gin.H{
			"message": "success",
		})
	} else {
		_, err = config.Db.Exec("INSERT INTO `0e7_flag` (uuid,flag,status,udpated) VALUES (?,?,?,?)", exploit_uuid, exploit_flag, "SKIPPED", updated)
		c.JSON(204, gin.H{
			"message": "SKIPPED",
		})
	}
}
