package search

import (
	"0E7/service/config"
	"0E7/service/database"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

// ElasticsearchService Elasticsearch搜索服务
type ElasticsearchService struct {
	client *elasticsearch.Client
	index  string
	mutex  sync.RWMutex
}

var (
	elasticsearchServiceInstance *ElasticsearchService
	elasticsearchOnce            sync.Once
)

// GetElasticsearchService 获取Elasticsearch搜索服务单例
func GetElasticsearchService() *ElasticsearchService {
	elasticsearchOnce.Do(func() {
		elasticsearchServiceInstance = &ElasticsearchService{
			index: "pcap_traffic",
		}
		err := elasticsearchServiceInstance.Init()
		if err != nil {
			log.Printf("Failed to initialize Elasticsearch service: %v", err)
			// 如果Elasticsearch初始化失败，不退出程序，只是记录错误
		}
	})
	return elasticsearchServiceInstance
}

// Init 初始化Elasticsearch客户端
func (s *ElasticsearchService) Init() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 配置Elasticsearch客户端
	cfg := elasticsearch.Config{
		Addresses: []string{
			config.Search_elasticsearch_url, // 使用配置文件中的地址
		},
	}

	// 如果配置了用户名和密码，则添加认证信息
	if config.Search_elasticsearch_username != "" {
		cfg.Username = config.Search_elasticsearch_username
		cfg.Password = config.Search_elasticsearch_password
	}

	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("创建Elasticsearch客户端失败: %v", err)
	}

	// 测试连接
	res, err := client.Info()
	if err != nil {
		return fmt.Errorf("连接Elasticsearch失败: %v", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("elasticsearch连接错误: %s", res.String())
	}

	s.client = client

	// 创建索引映射
	err = s.createIndexMapping()
	if err != nil {
		return fmt.Errorf("创建索引映射失败: %v", err)
	}

	log.Println("Elasticsearch服务初始化成功")
	return nil
}

// createIndexMapping 创建索引映射
func (s *ElasticsearchService) createIndexMapping() error {
	mapping := `{
		"mappings": {
			"properties": {
				"pcap_id": {
					"type": "integer"
				},
				"content": {
					"type": "text",
					"analyzer": "standard",
					"search_analyzer": "standard"
				},
				"client_content": {
					"type": "text",
					"analyzer": "standard",
					"search_analyzer": "standard"
				},
				"server_content": {
					"type": "text",
					"analyzer": "standard",
					"search_analyzer": "standard"
				},
				"src_ip": {
					"type": "keyword"
				},
				"dst_ip": {
					"type": "keyword"
				},
                "src_port": {
                    "type": "integer"
                },
                "dst_port": {
                    "type": "integer"
                },
				"tags": {
					"type": "keyword"
				},
				"timestamp": {
					"type": "integer"
				},
				"duration": {
					"type": "integer"
				},
				"num_packets": {
					"type": "integer"
				},
				"size": {
					"type": "integer"
				},
				"filename": {
					"type": "keyword"
				},
				"blocked": {
					"type": "keyword"
				},
				"created_at": {
					"type": "date"
				}
			}
		}
	}`

	req := esapi.IndicesCreateRequest{
		Index: s.index,
		Body:  strings.NewReader(mapping),
	}

	res, err := req.Do(context.Background(), s.client)
	if err != nil {
		// 如果索引已存在，忽略错误
		if res != nil && res.StatusCode == 400 {
			log.Println("Elasticsearch索引已存在，跳过创建")
			return nil
		}
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("创建索引失败: %s", string(body))
	}

	log.Printf("Elasticsearch索引 '%s' 创建成功", s.index)
	return nil
}

// IndexPcap 索引PCAP数据
func (s *ElasticsearchService) IndexPcap(pcapRecord database.Pcap) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.client == nil {
		return fmt.Errorf("elasticsearch客户端未初始化")
	}

	// 获取并拼接流量数据
	flowItems, err := s.getFlowData(pcapRecord)
	if err != nil {
		log.Printf("获取流量数据失败 (PcapID: %d): %v", pcapRecord.ID, err)
		// 即使获取流量数据失败，也尝试索引其他元数据
	}

	// 分别处理客户端和服务器端内容
	var allContentBuilder, clientContentBuilder, serverContentBuilder strings.Builder

	for _, item := range flowItems {
		decoded, err := base64.StdEncoding.DecodeString(item.B64)
		if err != nil {
			log.Printf("Base64解码失败 (PcapID: %d, FlowItemTime: %d): %v", pcapRecord.ID, item.Time, err)
			continue
		}

		decodedStr := string(decoded)

		// 添加到总内容（与Bleve一致，使用空格拼接）
		allContentBuilder.WriteString(decodedStr)
		allContentBuilder.WriteString(" ")

		// 根据方向分别添加到客户端或服务器端内容
		if item.From == "c" {
			// 客户端到服务器
			clientContentBuilder.WriteString(decodedStr)
			clientContentBuilder.WriteString(" ")
		} else if item.From == "s" {
			// 服务器到客户端
			serverContentBuilder.WriteString(decodedStr)
			serverContentBuilder.WriteString(" ")
		}
	}

	// 端口转为整数，便于数值检索
	toPortInt := func(s string) int {
		if s == "" {
			return 0
		}
		var n int
		fmt.Sscanf(s, "%d", &n)
		return n
	}

	doc := map[string]interface{}{
		"pcap_id":        pcapRecord.ID,
		"content":        allContentBuilder.String(),
		"client_content": clientContentBuilder.String(),
		"server_content": serverContentBuilder.String(),
		"src_ip":         pcapRecord.SrcIP,
		"dst_ip":         pcapRecord.DstIP,
		"src_port":       toPortInt(pcapRecord.SrcPort),
		"dst_port":       toPortInt(pcapRecord.DstPort),
		"tags":           pcapRecord.Tags,
		"timestamp":      pcapRecord.Time,
		"duration":       pcapRecord.Duration,
		"num_packets":    pcapRecord.NumPackets,
		"size":           pcapRecord.Size,
		"filename":       pcapRecord.Filename,
		"blocked":        pcapRecord.Blocked,
		"created_at":     pcapRecord.CreatedAt,
	}

	docJSON, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("序列化文档失败: %v", err)
	}

	req := esapi.IndexRequest{
		Index:      s.index,
		DocumentID: fmt.Sprintf("pcap_%d", pcapRecord.ID),
		Body:       bytes.NewReader(docJSON),
		// 为性能考虑，不强制刷新，由上层批量控制
	}

	res, err := req.Do(context.Background(), s.client)
	if err != nil {
		return fmt.Errorf("索引PCAP数据失败: %v", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("索引PCAP数据失败: %s", string(body))
	}

	log.Printf("成功索引PCAP数据到Elasticsearch (ID: %d)", pcapRecord.ID)
	return nil
}

// BulkIndexPcaps 批量索引PCAP数据（性能优化）
func (s *ElasticsearchService) BulkIndexPcaps(pcapRecords []database.Pcap) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.client == nil {
		return fmt.Errorf("elasticsearch客户端未初始化")
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)

	for _, pcapRecord := range pcapRecords {
		// 获取并拼接流量数据
		flowItems, err := s.getFlowData(pcapRecord)
		if err != nil {
			log.Printf("获取流量数据失败 (PcapID: %d): %v", pcapRecord.ID, err)
			// 继续处理其他记录
		}

		// 分别处理客户端和服务器端内容（与Bleve一致，使用空格）
		var allContentBuilder, clientContentBuilder, serverContentBuilder strings.Builder
		for _, item := range flowItems {
			decoded, err := base64.StdEncoding.DecodeString(item.B64)
			if err != nil {
				log.Printf("Base64解码失败 (PcapID: %d, FlowItemTime: %d): %v", pcapRecord.ID, item.Time, err)
				continue
			}
			decodedStr := string(decoded)
			allContentBuilder.WriteString(decodedStr)
			allContentBuilder.WriteString(" ")
			if item.From == "c" {
				clientContentBuilder.WriteString(decodedStr)
				clientContentBuilder.WriteString(" ")
			} else if item.From == "s" {
				serverContentBuilder.WriteString(decodedStr)
				serverContentBuilder.WriteString(" ")
			}
		}

		toPortInt := func(s string) int {
			if s == "" {
				return 0
			}
			var n int
			fmt.Sscanf(s, "%d", &n)
			return n
		}

		doc := map[string]interface{}{
			"pcap_id":        pcapRecord.ID,
			"content":        allContentBuilder.String(),
			"client_content": clientContentBuilder.String(),
			"server_content": serverContentBuilder.String(),
			"src_ip":         pcapRecord.SrcIP,
			"dst_ip":         pcapRecord.DstIP,
			"src_port":       toPortInt(pcapRecord.SrcPort),
			"dst_port":       toPortInt(pcapRecord.DstPort),
			"tags":           pcapRecord.Tags,
			"timestamp":      pcapRecord.Time,
			"duration":       pcapRecord.Duration,
			"num_packets":    pcapRecord.NumPackets,
			"size":           pcapRecord.Size,
			"filename":       pcapRecord.Filename,
			"blocked":        pcapRecord.Blocked,
			"created_at":     pcapRecord.CreatedAt,
		}

		// Bulk action/metadata line
		meta := map[string]map[string]string{
			"index": {
				"_index": s.index,
				"_id":    fmt.Sprintf("pcap_%d", pcapRecord.ID),
			},
		}
		if err := enc.Encode(meta); err != nil {
			return fmt.Errorf("写入bulk meta失败: %v", err)
		}
		if err := enc.Encode(doc); err != nil {
			return fmt.Errorf("写入bulk doc失败: %v", err)
		}
	}

	res, err := s.client.Bulk(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return fmt.Errorf("bulk索引失败: %v", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("bulk索引错误: %s", string(body))
	}

	log.Printf("成功批量索引 %d 条PCAP数据到Elasticsearch", len(pcapRecords))
	return nil
}

// getFlowData 获取流量数据
// 优先从数据库FlowData字段读取（<256KB），如果为空则从FlowFile文件读取（>=256KB）
func (s *ElasticsearchService) getFlowData(pcapRecord database.Pcap) ([]FlowItem, error) {
	var dataToParse []byte

	// 优先从数据库FlowData字段读取
	if pcapRecord.FlowData != "" {
		dataToParse = []byte(pcapRecord.FlowData)
	} else if pcapRecord.FlowFile != "" {
		// 从文件读取
		jsonData, err := os.ReadFile(pcapRecord.FlowFile)
		if err != nil {
			return nil, fmt.Errorf("读取流量文件失败: %v", err)
		}

		// 检查是否是压缩文件（通过文件扩展名或内容判断）
		if strings.HasSuffix(pcapRecord.FlowFile, ".gz") || (len(jsonData) > 2 && jsonData[0] == 0x1f && jsonData[1] == 0x8b) {
			// 解压缩gzip数据
			reader, err := gzip.NewReader(bytes.NewReader(jsonData))
			if err != nil {
				return nil, fmt.Errorf("创建gzip读取器失败: %v", err)
			}
			defer reader.Close()

			dataToParse, err = io.ReadAll(reader)
			if err != nil {
				return nil, fmt.Errorf("解压缩数据失败: %v", err)
			}
		} else {
			dataToParse = jsonData
		}
	} else {
		return nil, fmt.Errorf("流量数据为空（FlowData和FlowFile都为空）")
	}

	// 解析JSON
	var flowEntry FlowEntry
	err := json.Unmarshal(dataToParse, &flowEntry)
	if err != nil {
		return nil, fmt.Errorf("解析流量数据失败: %v", err)
	}

	return flowEntry.Flow, nil
}

// Search 执行搜索
func (s *ElasticsearchService) Search(queryStr string, page, pageSize int, searchType SearchType) ([]SearchResult, int64, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.client == nil {
		return nil, 0, fmt.Errorf("elasticsearch客户端未初始化")
	}

	// 构建搜索查询
	searchQuery := s.buildSearchQuery(queryStr, searchType)

	// 根据搜索类型选择高亮字段
	var highlightField string
	switch searchType {
	case SearchTypeClient:
		highlightField = "client_content"
	case SearchTypeServer:
		highlightField = "server_content"
	default:
		highlightField = "content"
	}

	// 构建搜索请求体
	searchBody := map[string]interface{}{
		"query": searchQuery,
		"from":  (page - 1) * pageSize,
		"size":  pageSize,
		"sort": []map[string]interface{}{
			{"pcap_id": map[string]interface{}{"order": "desc"}},
		},
		"highlight": map[string]interface{}{
			"fields": map[string]interface{}{
				highlightField: map[string]interface{}{},
			},
		},
	}

	searchBodyJSON, err := json.Marshal(searchBody)
	if err != nil {
		return nil, 0, fmt.Errorf("序列化搜索请求失败: %v", err)
	}

	req := esapi.SearchRequest{
		Index: []string{s.index},
		Body:  bytes.NewReader(searchBodyJSON),
	}

	res, err := req.Do(context.Background(), s.client)
	if err != nil {
		return nil, 0, fmt.Errorf("执行搜索查询失败: %v", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, 0, fmt.Errorf("搜索查询失败: %s", string(body))
	}

	// 解析搜索结果
	var searchResponse map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&searchResponse); err != nil {
		return nil, 0, fmt.Errorf("解析搜索结果失败: %v", err)
	}

	hits, ok := searchResponse["hits"].(map[string]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("搜索结果格式错误")
	}

	total, _ := hits["total"].(map[string]interface{})
	var totalValue float64
	if total != nil {
		if val, ok := total["value"].(float64); ok {
			totalValue = val
		}
	}

	hitsArray, ok := hits["hits"].([]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("搜索结果hits格式错误")
	}

	var results []SearchResult
	for _, hit := range hitsArray {
		hitMap, ok := hit.(map[string]interface{})
		if !ok {
			continue
		}

		source, ok := hitMap["_source"].(map[string]interface{})
		if !ok {
			continue
		}

		var score float64
		if s, ok := hitMap["_score"].(float64); ok {
			score = s
		}

		var pcapID float64
		if p, ok := source["pcap_id"].(float64); ok {
			pcapID = p
		}

		// 处理高亮结果
		highlights := make(map[string][]string)
		if highlight, ok := hitMap["highlight"].(map[string]interface{}); ok {
			for field, fragments := range highlight {
				if fragmentsArray, ok := fragments.([]interface{}); ok {
					var fragmentsStr []string
					for _, fragment := range fragmentsArray {
						if fragmentStr, ok := fragment.(string); ok {
							fragmentsStr = append(fragmentsStr, fragmentStr)
						}
					}
					highlights[field] = fragmentsStr
				}
			}
		}

		// 直接从_source读取，避免额外DB查询（与Bleve的字段返回风格在分页搜索时对齐）
		getString := func(field string) string {
			if val, ok := source[field]; ok && val != nil {
				if str, ok := val.(string); ok {
					return str
				}
			}
			return ""
		}
		getInt := func(field string) int {
			if val, ok := source[field]; ok && val != nil {
				if f, ok := val.(float64); ok {
					return int(f)
				}
				if i, ok := val.(int); ok {
					return i
				}
			}
			return 0
		}

		result := SearchResult{
			ID:         fmt.Sprintf("pcap_%d", int(pcapID)),
			PcapID:     int(pcapID),
			SrcIP:      getString("src_ip"),
			DstIP:      getString("dst_ip"),
			SrcPort:    getString("src_port"),
			DstPort:    getString("dst_port"),
			Tags:       getString("tags"),
			Timestamp:  getInt("timestamp"),
			Duration:   getInt("duration"),
			NumPackets: getInt("num_packets"),
			Size:       getInt("size"),
			Filename:   getString("filename"),
			Blocked:    getString("blocked"),
			Score:      score,
			Highlights: highlights,
		}
		results = append(results, result)
	}

	return results, int64(totalValue), nil
}

// SearchWithPort 执行带端口过滤的搜索
func (s *ElasticsearchService) SearchWithPort(queryStr, port string, page, pageSize int, searchType SearchType) ([]SearchResult, int64, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.client == nil {
		return nil, 0, fmt.Errorf("elasticsearch客户端未初始化")
	}

	// 构建搜索查询
	searchQuery := s.buildSearchQuery(queryStr, searchType)

	// 如果指定了端口，添加端口过滤条件（数值精确匹配）
	if port != "" {
		var portNum int
		fmt.Sscanf(port, "%d", &portNum)
		// 添加端口过滤到bool查询的filter部分
		if boolQuery, ok := searchQuery["bool"].(map[string]interface{}); ok {
			if boolQuery["filter"] == nil {
				boolQuery["filter"] = []interface{}{}
			}
			filters, ok := boolQuery["filter"].([]interface{})
			if !ok {
				// 如果filter不是[]interface{}类型，重新创建
				boolQuery["filter"] = []interface{}{}
				filters = boolQuery["filter"].([]interface{})
			}

			// 添加端口过滤条件（源端口或目标端口匹配）
			portFilter := map[string]interface{}{
				"bool": map[string]interface{}{
					"should": []interface{}{
						map[string]interface{}{
							"term": map[string]interface{}{
								"src_port": portNum,
							},
						},
						map[string]interface{}{
							"term": map[string]interface{}{
								"dst_port": portNum,
							},
						},
					},
					"minimum_should_match": 1,
				},
			}
			filters = append(filters, portFilter)
			boolQuery["filter"] = filters
		}
	}

	// 根据搜索类型选择高亮字段
	var highlightField string
	switch searchType {
	case SearchTypeClient:
		highlightField = "client_content"
	case SearchTypeServer:
		highlightField = "server_content"
	default:
		highlightField = "content"
	}

	// 构建搜索请求
	searchRequest := map[string]interface{}{
		"query": searchQuery,
		"from":  (page - 1) * pageSize,
		"size":  pageSize,
		"sort": []map[string]interface{}{
			{"pcap_id": map[string]interface{}{"order": "desc"}},
		},
		"highlight": map[string]interface{}{
			"fields": map[string]interface{}{
				highlightField: map[string]interface{}{},
			},
		},
	}

	// 执行搜索
	searchBody, err := json.Marshal(searchRequest)
	if err != nil {
		return nil, 0, err
	}

	res, err := s.client.Search(
		s.client.Search.WithIndex(s.index),
		s.client.Search.WithBody(bytes.NewReader(searchBody)),
	)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, 0, fmt.Errorf("elasticsearch搜索错误: %s", res.String())
	}

	// 解析搜索结果
	var searchResponse map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&searchResponse); err != nil {
		return nil, 0, err
	}

	hits, ok := searchResponse["hits"].(map[string]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("搜索结果格式错误")
	}

	total, _ := hits["total"].(map[string]interface{})
	var totalValue float64
	if total != nil {
		if val, ok := total["value"].(float64); ok {
			totalValue = val
		}
	}

	hitsArray, ok := hits["hits"].([]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("搜索结果hits格式错误")
	}

	var results []SearchResult
	for _, hit := range hitsArray {
		hitMap, ok := hit.(map[string]interface{})
		if !ok {
			continue
		}

		source, ok := hitMap["_source"].(map[string]interface{})
		if !ok {
			continue
		}

		var score float64
		if s, ok := hitMap["_score"].(float64); ok {
			score = s
		}

		var pcapID float64
		if p, ok := source["pcap_id"].(float64); ok {
			pcapID = p
		}

		// 获取高亮信息
		var highlights []string
		if highlight, ok := hitMap["highlight"].(map[string]interface{}); ok {
			if highlightFragments, ok := highlight[highlightField].([]interface{}); ok {
				for _, fragment := range highlightFragments {
					if fragmentStr, ok := fragment.(string); ok {
						highlights = append(highlights, fragmentStr)
					}
				}
			}
		}

		// 从_source 直接获取字段（与Bleve的 searchBleveWithPort 风格一致）
		getString := func(field string) string {
			if val, ok := source[field]; ok && val != nil {
				if str, ok := val.(string); ok {
					return str
				}
			}
			return ""
		}
		getInt := func(field string) int {
			if val, ok := source[field]; ok && val != nil {
				if f, ok := val.(float64); ok {
					return int(f)
				}
			}
			return 0
		}

		result := SearchResult{
			ID:         fmt.Sprintf("pcap_%d", int(pcapID)),
			PcapID:     int(pcapID),
			Content:    getString("content"),
			SrcIP:      getString("src_ip"),
			DstIP:      getString("dst_ip"),
			SrcPort:    getString("src_port"),
			DstPort:    getString("dst_port"),
			Tags:       getString("tags"),
			Timestamp:  getInt("timestamp"),
			Duration:   getInt("duration"),
			NumPackets: getInt("num_packets"),
			Size:       getInt("size"),
			Filename:   getString("filename"),
			Blocked:    getString("blocked"),
			Score:      score,
			Highlight:  strings.Join(highlights, " | "),
		}
		results = append(results, result)
	}

	return results, int64(totalValue), nil
}

// buildSearchQuery 构建搜索查询
func (s *ElasticsearchService) buildSearchQuery(queryStr string, searchType SearchType) map[string]interface{} {
	// 根据搜索类型选择字段
	var searchField string
	switch searchType {
	case SearchTypeClient:
		searchField = "client_content"
	case SearchTypeServer:
		searchField = "server_content"
	default:
		searchField = "content"
	}

	// 如果查询为空或只有通配符，返回匹配所有文档的查询
	if queryStr == "" || queryStr == "*" {
		return map[string]interface{}{
			"match_all": map[string]interface{}{},
		}
	}

	// 检查是否是正则表达式查询
	if strings.HasPrefix(queryStr, "/") && strings.HasSuffix(queryStr, "/") {
		// 正则表达式查询
		regexPattern := strings.Trim(queryStr, "/")
		return map[string]interface{}{
			"regexp": map[string]interface{}{
				searchField: map[string]interface{}{
					"value": regexPattern,
				},
			},
		}
	}

	// 检查是否包含特殊字符，决定使用短语查询还是匹配查询
	if strings.Contains(queryStr, " ") {
		// 包含空格，使用短语查询
		return map[string]interface{}{
			"match_phrase": map[string]interface{}{
				searchField: queryStr,
			},
		}
	}

	// 单个词，使用匹配查询
	return map[string]interface{}{
		"match": map[string]interface{}{
			searchField: queryStr,
		},
	}
}

// DeletePcap 删除PCAP索引
func (s *ElasticsearchService) DeletePcap(pcapID int) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.client == nil {
		return fmt.Errorf("Elasticsearch客户端未初始化")
	}

	req := esapi.DeleteRequest{
		Index:      s.index,
		DocumentID: fmt.Sprintf("pcap_%d", pcapID),
	}

	res, err := req.Do(context.Background(), s.client)
	if err != nil {
		return fmt.Errorf("删除PCAP索引失败: %v", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("删除PCAP索引失败: %s", string(body))
	}

	log.Printf("成功删除PCAP索引 (ID: %d)", pcapID)
	return nil
}

// GetIndexStats 获取索引统计信息
func (s *ElasticsearchService) GetIndexStats() (map[string]interface{}, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.client == nil {
		return nil, fmt.Errorf("Elasticsearch客户端未初始化")
	}

	req := esapi.IndicesStatsRequest{
		Index: []string{s.index},
	}

	res, err := req.Do(context.Background(), s.client)
	if err != nil {
		return nil, fmt.Errorf("获取索引统计信息失败: %v", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("获取索引统计信息失败: %s", string(body))
	}

	var stats map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("解析索引统计信息失败: %v", err)
	}

	return stats, nil
}

// IsAvailable 检查Elasticsearch服务是否可用
func (s *ElasticsearchService) IsAvailable() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.client == nil {
		return false
	}

	res, err := s.client.Info()
	if err != nil {
		return false
	}
	defer res.Body.Close()

	return !res.IsError()
}

// SearchByPcapIDs 在指定的PCAP ID范围内执行搜索
func (s *ElasticsearchService) SearchByPcapIDs(queryStr string, pcapIDs []int, searchType SearchType) ([]SearchResult, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.client == nil {
		return nil, fmt.Errorf("Elasticsearch客户端未初始化")
	}

	// 构建搜索查询
	searchQuery := s.buildSearchQuery(queryStr, searchType)

	// 添加ID范围过滤
	if len(pcapIDs) > 0 {
		termsQuery := map[string]interface{}{
			"terms": map[string]interface{}{
				"pcap_id": pcapIDs,
			},
		}

		// 组合查询：flag查询 AND ID范围查询
		combinedQuery := map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					searchQuery,
					termsQuery,
				},
			},
		}
		searchQuery = combinedQuery
	}

	// 执行搜索
	searchRequest := map[string]interface{}{
		"query": searchQuery,
		"size":  10000, // 设置足够大的size以获取所有结果
		"sort": []map[string]interface{}{
			{"pcap_id": map[string]interface{}{"order": "desc"}},
		},
	}

	// 根据搜索类型选择高亮字段
	var highlightField string
	switch searchType {
	case SearchTypeClient:
		highlightField = "client_content"
	case SearchTypeServer:
		highlightField = "server_content"
	default:
		highlightField = "content"
	}

	searchRequest["highlight"] = map[string]interface{}{
		"fields": map[string]interface{}{
			highlightField: map[string]interface{}{},
		},
	}

	res, err := s.client.Search(
		s.client.Search.WithIndex(s.index),
		s.client.Search.WithBody(strings.NewReader(toJSON(searchRequest))),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch搜索失败: %v", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch搜索错误: %s", res.String())
	}

	var searchResponse map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&searchResponse); err != nil {
		return nil, fmt.Errorf("解析搜索结果失败: %v", err)
	}

	// 转换结果
	var results []SearchResult
	if hits, ok := searchResponse["hits"].(map[string]interface{}); ok {
		if hitsList, ok := hits["hits"].([]interface{}); ok {
			for _, hit := range hitsList {
				if hitMap, ok := hit.(map[string]interface{}); ok {
					if source, ok := hitMap["_source"].(map[string]interface{}); ok {
						// 安全地获取字段值，避免nil指针异常
						getString := func(field string) string {
							if val, ok := source[field]; ok && val != nil {
								if str, ok := val.(string); ok {
									return str
								}
							}
							return ""
						}

						getInt := func(field string) int {
							if val, ok := source[field]; ok && val != nil {
								if f, ok := val.(float64); ok {
									return int(f)
								}
								if i, ok := val.(int); ok {
									return i
								}
							}
							return 0
						}

						result := SearchResult{
							PcapID:     getInt("pcap_id"),
							SrcIP:      getString("src_ip"),
							DstIP:      getString("dst_ip"),
							SrcPort:    getString("src_port"),
							DstPort:    getString("dst_port"),
							Tags:       getString("tags"),
							Timestamp:  getInt("timestamp"),
							Duration:   getInt("duration"),
							NumPackets: getInt("num_packets"),
							Size:       getInt("size"),
							Filename:   getString("filename"),
							Blocked:    getString("blocked"),
						}

						// 添加高亮信息
						if highlight, ok := hitMap["highlight"].(map[string]interface{}); ok {
							if highlightContent, ok := highlight[highlightField].([]interface{}); ok && len(highlightContent) > 0 {
								if highlightStr, ok := highlightContent[0].(string); ok {
									result.Highlight = highlightStr
								}
							}
						}

						results = append(results, result)
					}
				}
			}
		}
	}

	return results, nil
}

// toJSON 将对象转换为JSON字符串
func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
