package webui

import (
	"0E7/service/config"
	"0E7/service/search"
	"strconv"

	"github.com/gin-gonic/gin"
)

// search_pcap 搜索PCAP数据
func search_pcap(c *gin.Context) {
	query := c.PostForm("query")
	pageStr := c.PostForm("page")
	pageSizeStr := c.PostForm("page_size")

	if query == "" {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "搜索关键字不能为空",
		})
		return
	}

	// 解析分页参数
	page := 1
	pageSize := 20

	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	// 执行搜索
	searchService := search.GetSearchService()
	results, total, err := searchService.Search(query, page, pageSize)
	if err != nil {
		c.JSON(500, gin.H{
			"message": "fail",
			"error":   "搜索失败: " + err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"message":   "success",
		"result":    results,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// search_stats 获取搜索统计信息
func search_stats(c *gin.Context) {
	searchService := search.GetSearchService()
	stats, err := searchService.GetIndexStats()
	if err != nil {
		c.JSON(500, gin.H{
			"message": "fail",
			"error":   "获取统计信息失败: " + err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"message": "success",
		"result":  stats,
	})
}

// search_engine_info 获取搜索引擎信息
func search_engine_info(c *gin.Context) {
	searchService := search.GetSearchService()

	// 获取当前使用的搜索引擎
	engine := "bleve" // 默认
	if searchService.Engine() == search.SearchEngineElasticsearch {
		engine = "elasticsearch"
	}

	// 检查Elasticsearch是否可用
	elasticsearchAvailable := false
	if esService := search.GetElasticsearchService(); esService != nil {
		elasticsearchAvailable = esService.IsAvailable()
	}

	c.JSON(200, gin.H{
		"message": "success",
		"result": gin.H{
			"current_engine":          engine,
			"configured_engine":       config.Search_engine,
			"elasticsearch_url":       config.Search_elasticsearch_url,
			"elasticsearch_available": elasticsearchAvailable,
			"engines":                 []string{"bleve", "elasticsearch"},
			"database": gin.H{
				"engine":   config.Db_engine,
				"host":     config.Db_host,
				"port":     config.Db_port,
				"username": config.Db_username,
				"tables":   config.Db_tables,
			},
		},
	})
}

// switch_search_engine 切换搜索引擎
func switch_search_engine(c *gin.Context) {
	engine := c.PostForm("engine")
	if engine == "" {
		c.JSON(400, gin.H{"message": "fail", "error": "搜索引擎参数不能为空"})
		return
	}

	var searchEngine search.SearchEngine
	switch engine {
	case "elasticsearch":
		searchEngine = search.SearchEngineElasticsearch
	case "bleve":
		searchEngine = search.SearchEngineBleve
	default:
		c.JSON(400, gin.H{"message": "fail", "error": "不支持的搜索引擎: " + engine})
		return
	}

	// 检查Elasticsearch是否可用
	if searchEngine == search.SearchEngineElasticsearch {
		esService := search.GetElasticsearchService()
		if esService == nil || !esService.IsAvailable() {
			c.JSON(400, gin.H{"message": "fail", "error": "Elasticsearch服务不可用"})
			return
		}
	}

	// 这里需要重新初始化搜索服务，但由于单例模式，实际应用中可能需要重启服务
	// 或者修改搜索服务以支持动态切换
	c.JSON(200, gin.H{
		"message":        "success",
		"info":           "搜索引擎切换请求已接收，需要重启服务生效",
		"current_engine": engine,
	})
}
