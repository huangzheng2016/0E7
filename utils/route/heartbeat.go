package route

import (
	"0E7/utils/config"
	"0E7/utils/database"
	"0E7/utils/update"
	"log"

	"github.com/gin-gonic/gin"
)

func heartbeat(c *gin.Context) {
	client_uuid := c.PostForm("uuid")
	hostname := c.PostForm("hostname")
	platform := c.PostForm("platform")
	arch := c.PostForm("arch")
	cpu := c.PostForm("cpu")
	cpu_use := c.PostForm("cpu_use")
	memory_use := c.PostForm("memory_use")
	memory_max := c.PostForm("memory_max")
	pcap := c.PostForm("pcap")

	if hostname == "" || platform == "" || arch == "" || cpu == "" || cpu_use == "" || memory_use == "" || memory_max == "" {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "missing parameters",
			"sha256":  update.Sha256Hash,
		})
		c.Abort()
		return
	}

	var count int64
	err := config.Db.Model(&database.Client{}).Where("uuid = ? AND platform = ? AND arch = ?", client_uuid, platform, arch).Count(&count).Error
	if err != nil {
		log.Println("Failed to query database:", err)
	} else {
		if count == 0 {
			client := database.Client{
				UUID:      client_uuid,
				Hostname:  hostname,
				Platform:  platform,
				Arch:      arch,
				CPU:       cpu,
				CPUUse:    cpu_use,
				MemoryUse: memory_use,
				MemoryMax: memory_max,
				Pcap:      pcap,
			}
			err = config.Db.Create(&client).Error
		} else {
			err = config.Db.Model(&database.Client{}).Where("uuid = ? AND platform = ? AND arch = ?", client_uuid, platform, arch).Updates(map[string]interface{}{
				"hostname":   hostname,
				"platform":   platform,
				"arch":       arch,
				"cpu":        cpu,
				"cpu_use":    cpu_use,
				"memory_use": memory_use,
				"memory_max": memory_max,
				"pcap":       pcap,
			}).Error
		}
		if err != nil {
			c.JSON(400, gin.H{
				"message": "fail",
				"error":   err.Error(),
				"sha256":  update.Sha256Hash,
			})
			c.Abort()
		}
	}
	c.JSON(200, gin.H{
		"message": "success",
		"error":   "",
		"sha256":  update.Sha256Hash,
	})
}
