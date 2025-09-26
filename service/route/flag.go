package route

import (
	"0E7/service/config"
	"0E7/service/database"
	"strconv"

	"github.com/gin-gonic/gin"
)

func flag(c *gin.Context) {
	exploit_id_str := c.PostForm("exploit_id")
	exploit_id, err := strconv.Atoi(exploit_id_str)
	if err != nil {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "exploit_id error",
		})
		return
	}
	exploit_flag := c.PostForm("flag")
	team := c.PostForm("team")

	var count int64
	err = config.Db.Model(&database.Flag{}).Where("flag = ?", exploit_flag).Count(&count).Error
	if err != nil {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   err.Error(),
		})
		return
	}

	if count == 0 {
		flag := database.Flag{
			ExploitId: exploit_id,
			Flag:      exploit_flag,
			Status:    "QUEUE",
			Team:      team,
		}
		err = config.Db.Create(&flag).Error
		if err != nil {
			c.JSON(400, gin.H{
				"message": "fail",
				"error":   err.Error(),
			})
			return
		}
		c.JSON(200, gin.H{
			"message": "success",
			"error":   "",
		})
	} else {
		flag := database.Flag{
			ExploitId: exploit_id,
			Flag:      exploit_flag,
			Status:    "SKIPPED",
			Team:      team,
		}
		err = config.Db.Create(&flag).Error
		if err != nil {
			c.JSON(400, gin.H{
				"message": "fail",
				"error":   err.Error(),
			})
			return
		}
		c.JSON(202, gin.H{
			"message": "skipped",
			"error":   "",
		})
	}
}
