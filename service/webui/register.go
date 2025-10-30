package webui

import (
	_ "embed"

	"github.com/gin-gonic/gin"
)

func Register(router *gin.Engine) {
	router.POST("/webui/exploit", exploit)
	router.POST("/webui/exploit_rename", exploit_rename)
	router.POST("/webui/exploit_show", exploit_show)
	router.POST("/webui/exploit_show_output", exploit_show_output)
	router.POST("/webui/exploit_get_by_id", exploit_get_by_id)
	router.POST("/webui/exploit_delete", exploit_delete)
	router.POST("/webui/action", action)
	router.POST("/webui/action_show", action_show)
	router.POST("/webui/action_get_by_id", action_get_by_id)
	router.POST("/webui/action_delete", action_delete)
	router.POST("/webui/action_execute", action_execute)

	router.POST("/webui/pcap_upload", pcap_upload)
	router.POST("/webui/pcap_show", pcap_show)
	router.POST("/webui/pcap_get_by_id", pcap_get_by_id)
	router.POST("/webui/pcap_download", pcap_download)

	// 搜索相关路由
	router.POST("/webui/search_pcap", search_pcap)
	router.POST("/webui/search_stats", search_stats)
	router.POST("/webui/search_engine_info", search_engine_info)
	router.POST("/webui/switch_search_engine", switch_search_engine)

	// Flag管理相关路由
	router.POST("/webui/flag_show", GetFlagList)
	router.POST("/webui/flag_submit", SubmitFlag)
	router.POST("/webui/flag_delete", DeleteFlag)

	// Flag检测和配置相关路由
	router.POST("/webui/flag_config", GetCurrentFlagConfig)
	router.POST("/webui/flag_config_update", UpdateFlagConfig)

	// 代码生成相关路由
	router.POST("/webui/pcap_generate_code", pcap_generate_code)

	// Proxy 缓存监控
	router.POST("/webui/proxy_cache_list", proxy_cache_list)
}
