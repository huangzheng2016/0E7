package route

import (
	"0E7/utils/config"
	"0E7/utils/update"
	"fmt"
	"github.com/gin-gonic/gin"
	"time"
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
	updated := time.Now().Format(time.DateTime)

	var count int
	err := config.Db.QueryRow("SELECT COUNT(*) FROM `0e7_client` WHERE uuid=? AND platform=? AND arch=?", client_uuid, platform, arch).Scan(&count)
	if err != nil {
		fmt.Println("Failed to query database:", err)
	}
	if count == 0 {
		_, err = config.Db.Exec("INSERT INTO `0e7_client` (uuid,hostname,platform,arch,cpu,cpu_use,memory_use,memory_max,updated) VALUES (?,?,?,?,?,?,?,?,?)", client_uuid, hostname, platform, arch, cpu, cpu_use, memory_use, memory_max, updated)
	} else {
		_, err = config.Db.Exec("UPDATE `0e7_client` SET hostname=?,platform=?,arch=?,cpu=?,cpu_use=?,memory_use=?,memory_max=?,updated=? WHERE uuid=? AND platform=? AND arch=?", hostname, platform, arch, cpu, cpu_use, memory_use, memory_max, updated, client_uuid, platform, arch)
	}
	if err != nil {
		c.JSON(400, gin.H{
			"message": "fail",
			"err":     err,
			"sha256":  update.Sha256Hash,
		})
		c.Abort()
	}
	c.JSON(200, gin.H{
		"message": "success",
		"sha256":  update.Sha256Hash,
	})
}
