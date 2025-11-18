package webui

import (
	"0E7/service/config"
	"0E7/service/database"
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// 获取所有在线客户端
func getClients(c *gin.Context) {
	var clients []database.Client

	// 获取最近1分钟内有心跳的客户端
	oneMinuteAgo := time.Now().Add(-1 * time.Minute)
	err := config.Db.Where("updated_at > ?", oneMinuteAgo).Find(&clients).Error
	if err != nil {
		c.JSON(500, gin.H{
			"message": "fail",
			"error":   err.Error(),
			"result":  []interface{}{},
		})
		return
	}

	// 解析每个客户端的网卡信息
	result := make([]map[string]interface{}, 0) // 确保始终是数组而不是 nil
	for _, client := range clients {
		clientInfo := map[string]interface{}{
			"id":         client.ID,
			"name":       client.Name,
			"hostname":   client.Hostname,
			"platform":   client.Platform,
			"arch":       client.Arch,
			"cpu":        client.CPU,
			"cpu_use":    client.CPUUse,
			"memory_use": client.MemoryUse,
			"memory_max": client.MemoryMax,
			"updated_at": client.UpdatedAt,
			"interfaces": []map[string]interface{}{},
		}

		// 解析网卡信息
		if client.Pcap != "" {
			var interfaces []map[string]interface{}
			err := json.Unmarshal([]byte(client.Pcap), &interfaces)
			if err == nil {
				clientInfo["interfaces"] = interfaces
			}
		}

		result = append(result, clientInfo)
	}

	c.JSON(200, gin.H{
		"message": "success",
		"error":   "",
		"result":  result,
	})
}

// 下发流量采集任务
func createTrafficCollection(c *gin.Context) {
	clientIdStr := c.PostForm("client_id")
	interfaceName := c.PostForm("interface_name") // 网卡名称，为空表示所有网卡
	bpf := c.PostForm("bpf")                      // BPF过滤器，为空表示采集所有流量
	intervalStr := c.PostForm("interval")         // 采集间隔，默认60秒
	description := c.PostForm("description")      // 任务描述

	if clientIdStr == "" {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "missing client_id parameter",
			"result":  "",
		})
		return
	}

	clientId, err := strconv.Atoi(clientIdStr)
	if err != nil {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "invalid client_id: " + err.Error(),
			"result":  "",
		})
		return
	}

	// 解析采集间隔
	interval := 60 // 默认60秒
	if intervalStr != "" {
		if parsedInterval, err := strconv.Atoi(intervalStr); err == nil && parsedInterval > 0 {
			interval = parsedInterval
		}
	}

	// 构建监控任务数据
	taskData := map[string]interface{}{
		"name":        interfaceName,
		"description": description,
		"bpf":         bpf,
	}

	taskDataJSON, err := json.Marshal(taskData)
	if err != nil {
		c.JSON(500, gin.H{
			"message": "fail",
			"error":   "failed to marshal task data: " + err.Error(),
			"result":  "",
		})
		return
	}

	// 创建监控任务
	monitor := database.Monitor{
		ClientId: clientId,
		Name:     interfaceName,
		Types:    "pcap",
		Data:     string(taskDataJSON),
		Interval: interval,
	}

	err = config.Db.Create(&monitor).Error
	if err != nil {
		c.JSON(500, gin.H{
			"message": "fail",
			"error":   "failed to create monitor task: " + err.Error(),
			"result":  "",
		})
		return
	}

	c.JSON(200, gin.H{
		"message": "success",
		"error":   "",
		"result": map[string]interface{}{
			"id":        monitor.ID,
			"client_id": monitor.ClientId,
			"name":      monitor.Name,
			"types":     monitor.Types,
			"data":      monitor.Data,
			"interval":  monitor.Interval,
		},
	})
}

// 获取客户端的监控任务列表
func getClientMonitors(c *gin.Context) {
	clientIdStr := c.PostForm("client_id")
	if clientIdStr == "" {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "missing client_id parameter",
			"result":  []interface{}{},
		})
		return
	}

	clientId, err := strconv.Atoi(clientIdStr)
	if err != nil {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "invalid client_id: " + err.Error(),
			"result":  []interface{}{},
		})
		return
	}

	var monitors []database.Monitor
	err = config.Db.Where("client_id = ?", clientId).Find(&monitors).Error
	if err != nil {
		c.JSON(500, gin.H{
			"message": "fail",
			"error":   err.Error(),
			"result":  []interface{}{},
		})
		return
	}

	result := make([]map[string]interface{}, 0) // 确保始终是数组而不是 nil
	for _, monitor := range monitors {
		result = append(result, map[string]interface{}{
			"id":         monitor.ID,
			"client_id":  monitor.ClientId,
			"name":       monitor.Name,
			"types":      monitor.Types,
			"data":       monitor.Data,
			"interval":   monitor.Interval,
			"created_at": monitor.CreatedAt,
			"updated_at": monitor.UpdatedAt,
		})
	}

	c.JSON(200, gin.H{
		"message": "success",
		"error":   "",
		"result":  result,
	})
}

// 删除监控任务
func deleteMonitor(c *gin.Context) {
	monitorIdStr := c.PostForm("monitor_id")
	if monitorIdStr == "" {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "missing monitor_id parameter",
			"result":  "",
		})
		return
	}

	monitorId, err := strconv.Atoi(monitorIdStr)
	if err != nil {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "invalid monitor_id: " + err.Error(),
			"result":  "",
		})
		return
	}

	err = config.Db.Delete(&database.Monitor{}, monitorId).Error
	if err != nil {
		c.JSON(500, gin.H{
			"message": "fail",
			"error":   "failed to delete monitor: " + err.Error(),
			"result":  "",
		})
		return
	}

	c.JSON(200, gin.H{
		"message": "success",
		"error":   "",
		"result":  "monitor deleted successfully",
	})
}

