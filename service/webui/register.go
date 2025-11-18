package webui

import (
	_ "embed"

	"github.com/gin-gonic/gin"
)

func Register(router *gin.Engine) {
	router.POST("/webui/exploit", exploit)
	router.POST("/webui/exploit_rename", exploit_rename)
	router.GET("/webui/exploit_show", exploit_show)
	router.GET("/webui/exploit_show_output", exploit_show_output)
	router.GET("/webui/exploit_get_by_id", exploit_get_by_id)
	router.POST("/webui/exploit_delete", exploit_delete)
	router.POST("/webui/action", action)
	router.POST("/webui/action_show", action_show)
	router.GET("/webui/action_get_by_id", action_get_by_id)
	router.POST("/webui/action_delete", action_delete)
	router.POST("/webui/action_execute", action_execute)

	router.POST("/webui/pcap_upload", pcap_upload)
	router.GET("/webui/pcap_show", pcap_show)
	router.GET("/webui/pcap_get_by_id", pcap_get_by_id)
	router.GET("/webui/pcap_download", pcap_download)

	// 搜索相关路由
	router.GET("/webui/search_pcap", search_pcap)

	// Flag管理相关路由
	router.GET("/webui/flag_show", GetFlagList)
	router.POST("/webui/flag_submit", SubmitFlag)
	router.POST("/webui/flag_delete", DeleteFlag)

	// Flag检测和配置相关路由
	router.GET("/webui/flag_config", GetCurrentFlagConfig)
	router.POST("/webui/flag_config_update", UpdateFlagConfig)

	// 代码生成相关路由
	router.GET("/webui/pcap_generate_code", pcap_generate_code)

	// Proxy 缓存监控
	router.GET("/webui/proxy_cache_list", proxy_cache_list)

	// Git 仓库管理
	router.GET("/webui/git_repo_list", git_repo_list)
	router.POST("/webui/git_repo_update_description", git_repo_update_description)
	router.POST("/webui/git_repo_delete", git_repo_delete)

	// 终端管理相关API
	router.GET("/webui/clients", getClients)
	router.POST("/webui/traffic_collection", createTrafficCollection)
	router.POST("/webui/client_monitors", getClientMonitors)
	router.POST("/webui/delete_monitor", deleteMonitor)

	// 日志流式传输WebSocket
	router.GET("/webui/log/ws", handleLogWebSocket)
}
