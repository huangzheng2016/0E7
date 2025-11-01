package webui

import (
	"0E7/service/config"
	"0E7/service/database"
	"0E7/service/search"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// search_pcap 搜索PCAP数据
func search_pcap(c *gin.Context) {
	query := c.Query("query")
	pageStr := c.Query("page")
	pageSizeStr := c.Query("page_size")
	searchTypeStr := c.Query("search_type") // 新增搜索类型参数
	searchMode := c.Query("search_mode")    // 新增搜索模式参数：keyword(关键词) 或 string(字符串匹配)
	port := c.Query("port")                 // 新增端口搜索参数

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

	// 解析搜索类型参数
	searchType := search.SearchTypeAll // 默认为搜索所有内容
	if searchTypeStr != "" {
		if st, err := strconv.Atoi(searchTypeStr); err == nil {
			switch st {
			case 1:
				searchType = search.SearchTypeClient
			case 2:
				searchType = search.SearchTypeServer
			default:
				searchType = search.SearchTypeAll
			}
		}
	}

	// 执行搜索
	searchService := search.GetSearchService()

	// 根据搜索模式选择搜索方法
	if searchMode == "string" {
		// 字符串匹配模式：直接查询数据库
		results, total, err := searchPcapByString(query, port, page, pageSize, searchType)
		if err != nil {
			c.JSON(500, gin.H{
				"message": "fail",
				"error":   "字符串搜索失败: " + err.Error(),
			})
			return
		}

		c.JSON(200, gin.H{
			"message":     "success",
			"result":      results,
			"total":       total,
			"page":        page,
			"page_size":   pageSize,
			"search_type": int(searchType),
			"search_mode": "string",
		})
		return
	}

	// 检查是否是标签查询
	if strings.HasPrefix(query, "tags:FLAG-IN") {
		// 解析查询：tags:FLAG-IN [AND keyword]
		var keyword string
		if strings.Contains(query, " AND ") {
			parts := strings.Split(query, " AND ")
			if len(parts) > 1 {
				keyword = strings.TrimSpace(parts[1])
			}
		}

		// 直接查询数据库，使用标签过滤
		results, total, err := searchPcapByTag("FLAG-IN", keyword, port, page, pageSize, searchType)
		if err != nil {
			c.JSON(500, gin.H{
				"message": "fail",
				"error":   "搜索失败: " + err.Error(),
			})
			return
		}

		c.JSON(200, gin.H{
			"message":     "success",
			"result":      results,
			"total":       total,
			"page":        page,
			"page_size":   pageSize,
			"search_type": int(searchType),
		})
		return
	} else if strings.HasPrefix(query, "tags:FLAG-OUT") {
		// 解析查询：tags:FLAG-OUT [AND keyword]
		var keyword string
		if strings.Contains(query, " AND ") {
			parts := strings.Split(query, " AND ")
			if len(parts) > 1 {
				keyword = strings.TrimSpace(parts[1])
			}
		}

		// 直接查询数据库，使用标签过滤
		results, total, err := searchPcapByTag("FLAG-OUT", keyword, port, page, pageSize, searchType)
		if err != nil {
			c.JSON(500, gin.H{
				"message": "fail",
				"error":   "搜索失败: " + err.Error(),
			})
			return
		}

		c.JSON(200, gin.H{
			"message":     "success",
			"result":      results,
			"total":       total,
			"page":        page,
			"page_size":   pageSize,
			"search_type": int(searchType),
		})
		return
	} else if strings.HasPrefix(query, "tags:FLAG-IN AND tags:FLAG-OUT") {
		// 解析查询：tags:FLAG-IN AND tags:FLAG-OUT [AND keyword]
		var keyword string
		if strings.Contains(query, " AND ") {
			parts := strings.Split(query, " AND ")
			if len(parts) > 2 {
				keyword = strings.TrimSpace(parts[2])
			}
		}

		// 直接查询数据库，使用两个标签过滤
		results, total, err := searchPcapByTags([]string{"FLAG-IN", "FLAG-OUT"}, keyword, port, page, pageSize, searchType)
		if err != nil {
			c.JSON(500, gin.H{
				"message": "fail",
				"error":   "搜索失败: " + err.Error(),
			})
			return
		}

		c.JSON(200, gin.H{
			"message":     "success",
			"result":      results,
			"total":       total,
			"page":        page,
			"page_size":   pageSize,
			"search_type": int(searchType),
		})
		return
	}

	// 普通搜索
	results, total, err := searchService.SearchWithPort(query, port, page, pageSize, searchType)
	if err != nil {
		c.JSON(500, gin.H{
			"message": "fail",
			"error":   "搜索失败: " + err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"message":     "success",
		"result":      results,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"search_type": int(searchType),
	})
}

// searchPcapByString 通过字符串匹配搜索PCAP数据
func searchPcapByString(query, port string, page, pageSize int, searchType search.SearchType) ([]search.SearchResult, int64, error) {
	var results []search.SearchResult
	var total int64

	// 构建数据库查询
	dbQuery := config.Db.Model(&database.Pcap{})

	// 根据搜索类型选择搜索字段
	var searchField string
	switch searchType {
	case search.SearchTypeClient:
		searchField = "client_content"
	case search.SearchTypeServer:
		searchField = "server_content"
	default:
		// 搜索全部内容，使用OR条件
		dbQuery = dbQuery.Where("client_content LIKE ? OR server_content LIKE ?", "%"+query+"%", "%"+query+"%")
	}

	// 如果指定了特定字段，使用该字段搜索
	if searchField != "" {
		dbQuery = dbQuery.Where(searchField+" LIKE ?", "%"+query+"%")
	}

	// 如果指定了端口，添加端口过滤条件（同时搜索源端口和目标端口）
	if port != "" {
		dbQuery = dbQuery.Where("src_port = ? OR dst_port = ?", port, port)
	}

	// 获取总数
	err := dbQuery.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	var pcaps []database.Pcap
	offset := (page - 1) * pageSize
	err = dbQuery.Offset(offset).Limit(pageSize).Order("id DESC").Find(&pcaps).Error
	if err != nil {
		return nil, 0, err
	}

	// 转换为搜索结果格式
	for _, pcap := range pcaps {
		result := search.SearchResult{
			ID:         fmt.Sprintf("pcap_%d", pcap.ID),
			PcapID:     pcap.ID,
			Content:    pcap.ClientContent + " " + pcap.ServerContent,
			SrcIP:      pcap.SrcIP,
			DstIP:      pcap.DstIP,
			SrcPort:    pcap.SrcPort,
			DstPort:    pcap.DstPort,
			Tags:       pcap.Tags,
			Timestamp:  pcap.Time,
			Duration:   pcap.Duration,
			NumPackets: pcap.NumPackets,
			Size:       pcap.Size,
			Filename:   pcap.Filename,
			Blocked:    pcap.Blocked,
			Score:      1.0, // 字符串匹配固定分数
		}
		results = append(results, result)
	}

	return results, total, nil
}

// searchPcapByTag 通过标签搜索PCAP数据
func searchPcapByTag(tag, keyword, port string, page, pageSize int, searchType search.SearchType) ([]search.SearchResult, int64, error) {
	var results []search.SearchResult
	var total int64

	// 构建数据库查询
	dbQuery := config.Db.Model(&database.Pcap{}).Where("tags LIKE ?", "%"+tag+"%")

	// 如果有关键词，添加内容搜索条件
	if keyword != "" && keyword != "*" {
		// 根据搜索类型选择搜索字段
		switch searchType {
		case search.SearchTypeClient:
			dbQuery = dbQuery.Where("client_content LIKE ?", "%"+keyword+"%")
		case search.SearchTypeServer:
			dbQuery = dbQuery.Where("server_content LIKE ?", "%"+keyword+"%")
		default:
			// 搜索全部内容，使用OR条件
			dbQuery = dbQuery.Where("client_content LIKE ? OR server_content LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
		}
	}

	// 如果指定了端口，添加端口过滤条件（同时搜索源端口和目标端口）
	if port != "" {
		dbQuery = dbQuery.Where("src_port = ? OR dst_port = ?", port, port)
	}

	// 获取总数
	err := dbQuery.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	var pcaps []database.Pcap
	offset := (page - 1) * pageSize
	err = dbQuery.Offset(offset).Limit(pageSize).Order("id DESC").Find(&pcaps).Error
	if err != nil {
		return nil, 0, err
	}

	// 转换为搜索结果格式
	for _, pcap := range pcaps {
		result := search.SearchResult{
			ID:         fmt.Sprintf("pcap_%d", pcap.ID),
			PcapID:     pcap.ID,
			Content:    pcap.ClientContent + " " + pcap.ServerContent,
			SrcIP:      pcap.SrcIP,
			DstIP:      pcap.DstIP,
			SrcPort:    pcap.SrcPort,
			DstPort:    pcap.DstPort,
			Tags:       pcap.Tags,
			Timestamp:  pcap.Time,
			Duration:   pcap.Duration,
			NumPackets: pcap.NumPackets,
			Size:       pcap.Size,
			Filename:   pcap.Filename,
			Blocked:    pcap.Blocked,
			Score:      1.0, // 固定分数
		}
		results = append(results, result)
	}

	return results, total, nil
}

// searchPcapByTags 通过多个标签搜索PCAP数据
func searchPcapByTags(tags []string, keyword, port string, page, pageSize int, searchType search.SearchType) ([]search.SearchResult, int64, error) {
	var results []search.SearchResult
	var total int64

	// 构建数据库查询
	dbQuery := config.Db.Model(&database.Pcap{})

	// 添加所有标签的过滤条件（使用OR逻辑）
	if len(tags) > 0 {
		var tagConditions []string
		var tagArgs []interface{}
		for _, tag := range tags {
			tagConditions = append(tagConditions, "tags LIKE ?")
			tagArgs = append(tagArgs, "%"+tag+"%")
		}
		dbQuery = dbQuery.Where(strings.Join(tagConditions, " OR "), tagArgs...)
	}

	// 如果有关键词，添加内容搜索条件
	if keyword != "" && keyword != "*" {
		// 根据搜索类型选择搜索字段
		switch searchType {
		case search.SearchTypeClient:
			dbQuery = dbQuery.Where("client_content LIKE ?", "%"+keyword+"%")
		case search.SearchTypeServer:
			dbQuery = dbQuery.Where("server_content LIKE ?", "%"+keyword+"%")
		default:
			// 搜索全部内容，使用OR条件
			dbQuery = dbQuery.Where("client_content LIKE ? OR server_content LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
		}
	}

	// 如果指定了端口，添加端口过滤条件（同时搜索源端口和目标端口）
	if port != "" {
		dbQuery = dbQuery.Where("src_port = ? OR dst_port = ?", port, port)
	}

	// 获取总数
	err := dbQuery.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	var pcaps []database.Pcap
	offset := (page - 1) * pageSize
	err = dbQuery.Offset(offset).Limit(pageSize).Order("id DESC").Find(&pcaps).Error
	if err != nil {
		return nil, 0, err
	}

	// 转换为搜索结果格式
	for _, pcap := range pcaps {
		result := search.SearchResult{
			ID:         fmt.Sprintf("pcap_%d", pcap.ID),
			PcapID:     pcap.ID,
			Content:    pcap.ClientContent + " " + pcap.ServerContent,
			SrcIP:      pcap.SrcIP,
			DstIP:      pcap.DstIP,
			SrcPort:    pcap.SrcPort,
			DstPort:    pcap.DstPort,
			Tags:       pcap.Tags,
			Timestamp:  pcap.Time,
			Duration:   pcap.Duration,
			NumPackets: pcap.NumPackets,
			Size:       pcap.Size,
			Filename:   pcap.Filename,
			Blocked:    pcap.Blocked,
			Score:      1.0, // 固定分数
		}
		results = append(results, result)
	}

	return results, total, nil
}
