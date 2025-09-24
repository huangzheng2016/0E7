package route

import (
	"0E7/utils/config"
	"0E7/utils/database"

	"github.com/gin-gonic/gin"
)

func flag(c *gin.Context) {
	exploit_uuid := c.PostForm("uuid")
	exploit_flag := c.PostForm("flag")

	var count int64
	err := config.Db.Model(&database.Flag{}).Where("flag = ?", exploit_flag).Count(&count).Error
	if err != nil {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   err.Error(),
		})
		return
	}

	if count == 0 {
		flag := database.Flag{
			UUID:   exploit_uuid,
			Flag:   exploit_flag,
			Status: "QUEUE",
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
			UUID:   exploit_uuid,
			Flag:   exploit_flag,
			Status: "SKIPPED",
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
