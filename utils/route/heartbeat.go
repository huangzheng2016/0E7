package route

import (
	"0E7/utils/config"
	"0E7/utils/update"
	"github.com/gin-gonic/gin"
	"log"
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
	pcap := c.PostForm("pcap")
	updated := time.Now().Format(time.DateTime)
	if hostname == "" || platform == "" || arch == "" || cpu == "" || cpu_use == "" || memory_use == "" || memory_max == "" {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "missing parameters",
			"sha256":  update.Sha256Hash,
		})
		c.Abort()
		return
	}
	var count int
	err := config.Db.QueryRow("SELECT COUNT(*) FROM `0e7_client` WHERE uuid=? AND platform=? AND arch=?", client_uuid, platform, arch).Scan(&count)
	if err != nil {
		log.Println("Failed to query database:", err)
	} else {
		if count == 0 {
			_, err = config.Db.Exec("INSERT INTO `0e7_client` (uuid,hostname,platform,arch,cpu,cpu_use,memory_use,memory_max,pcap,updated) VALUES (?,?,?,?,?,?,?,?,?,?)", client_uuid, hostname, platform, arch, cpu, cpu_use, memory_use, memory_max, pcap, updated)
		} else {
			_, err = config.Db.Exec("UPDATE `0e7_client` SET hostname=?,platform=?,arch=?,cpu=?,cpu_use=?,memory_use=?,memory_max=?,pcap=?,updated=? WHERE uuid=? AND platform=? AND arch=?", hostname, platform, arch, cpu, cpu_use, memory_use, memory_max, pcap, updated, client_uuid, platform, arch)
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
