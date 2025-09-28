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
	router.POST("/webui/flow_download", flow_download)

	// Flag管理相关路由
	router.POST("/webui/flag_show", GetFlagList)
	router.POST("/webui/flag/submit", SubmitFlag)
	router.POST("/webui/flag_delete", DeleteFlag)
	router.POST("/webui/flag/stats", GetFlagStats)
	router.POST("/webui/flag/batch_update", BatchUpdateFlagStatus)
}
