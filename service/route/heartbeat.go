package route

import (
	"0E7/service/config"
	"0E7/service/database"
	"0E7/service/update"
	"strconv"

	"github.com/gin-gonic/gin"
)

func heartbeat(c *gin.Context) {
	client_id_str := c.PostForm("client_id")
	client_id, err := strconv.ParseInt(client_id_str, 10, 64)
	if err != nil {
		client_id = 0
	}
	client_name := c.PostForm("name")
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

	var client database.Client
	var found bool = false

	// 如果client_id存在且不为0，先根据ID查询
	if client_id > 0 {
		err = config.Db.Where("id = ?", client_id).First(&client).Error
		if err == nil {
			found = true
		}
	}

	// 如果根据ID没找到，且client_name不为空，根据name查询
	if !found && client_name != "" {
		err = config.Db.Where("name = ? AND platform = ? AND arch = ?", client_name, platform, arch).First(&client).Error
		if err == nil {
			found = true
			client_id = int64(client.ID) // 更新client_id为找到的记录的ID
		}
	}

	if found {
		// 更新现有记录
		err = config.Db.Model(&client).Updates(map[string]interface{}{
			"hostname":   hostname,
			"platform":   platform,
			"arch":       arch,
			"cpu":        cpu,
			"cpu_use":    cpu_use,
			"memory_use": memory_use,
			"memory_max": memory_max,
			"pcap":       pcap,
		}).Error
		if err != nil {
			c.JSON(400, gin.H{
				"message": "fail",
				"error":   err.Error(),
				"sha256":  update.Sha256Hash,
			})
			c.Abort()
			return
		}
		client_id = int64(client.ID) // 确保返回正确的ID
	} else {
		// 创建新记录
		client = database.Client{
			Name:      client_name,
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
		if err != nil {
			c.JSON(400, gin.H{
				"message": "fail",
				"error":   err.Error(),
				"sha256":  update.Sha256Hash,
			})
			c.Abort()
			return
		}
		client_id = int64(client.ID) // 获取新创建记录的ID
	}
	c.JSON(200, gin.H{
		"message": "success",
		"error":   "",
		"sha256":  update.Sha256Hash,
		"id":      client_id,
	})
}
