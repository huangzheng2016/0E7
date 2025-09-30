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
		return fmt.Errorf("Elasticsearch连接错误: %s", res.String())
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
				"src_ip": {
					"type": "keyword"
				},
				"dst_ip": {
					"type": "keyword"
				},
				"src_port": {
					"type": "keyword"
				},
				"dst_port": {
					"type": "keyword"
				},
				"tags": {
					"type": "keyword"
				},
				"timestamp": {
					"type": "integer"
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
		return fmt.Errorf("Elasticsearch客户端未初始化")
	}

	// 获取并拼接流量数据
	flowItems, err := s.getFlowData(pcapRecord)
	if err != nil {
		log.Printf("获取流量数据失败 (PcapID: %d): %v", pcapRecord.ID, err)
		// 即使获取流量数据失败，也尝试索引其他元数据
	}

	var contentBuilder strings.Builder
	for _, item := range flowItems {
		decoded, err := base64.StdEncoding.DecodeString(item.B64)
		if err != nil {
			log.Printf("Base64解码失败 (PcapID: %d, FlowItemTime: %d): %v", pcapRecord.ID, item.Time, err)
			continue
		}
		contentBuilder.WriteString(string(decoded))
		contentBuilder.WriteString("\n")
	}

	doc := map[string]interface{}{
		"pcap_id":    pcapRecord.ID,
		"content":    contentBuilder.String(),
		"src_ip":     pcapRecord.SrcIP,
		"dst_ip":     pcapRecord.DstIP,
		"src_port":   pcapRecord.SrcPort,
		"dst_port":   pcapRecord.DstPort,
		"tags":       pcapRecord.Tags,
		"timestamp":  pcapRecord.Time,
		"created_at": pcapRecord.CreatedAt,
	}

	docJSON, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("序列化文档失败: %v", err)
	}

	req := esapi.IndexRequest{
		Index:      s.index,
		DocumentID: fmt.Sprintf("pcap-%d", pcapRecord.ID),
		Body:       bytes.NewReader(docJSON),
		Refresh:    "true",
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

// getFlowData 获取流量数据（复用Bleve中的逻辑）
func (s *ElasticsearchService) getFlowData(pcapRecord database.Pcap) ([]FlowItem, error) {
	if pcapRecord.FlowFile == "" {
		return nil, fmt.Errorf("流量文件路径为空")
	}

	// 读取JSON文件
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

		decompressedData, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("解压缩数据失败: %v", err)
		}
		jsonData = decompressedData
	}

	// 解析JSON
	var flowEntry FlowEntry
	err = json.Unmarshal(jsonData, &flowEntry)
	if err != nil {
		return nil, fmt.Errorf("解析流量数据失败: %v", err)
	}

	return flowEntry.Flow, nil
}

// Search 执行搜索
func (s *ElasticsearchService) Search(queryStr string, page, pageSize int) ([]SearchResult, int64, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.client == nil {
		return nil, 0, fmt.Errorf("Elasticsearch客户端未初始化")
	}

	// 构建搜索查询
	searchQuery := s.buildSearchQuery(queryStr)

	// 构建搜索请求体
	searchBody := map[string]interface{}{
		"query": searchQuery,
		"from":  (page - 1) * pageSize,
		"size":  pageSize,
		"highlight": map[string]interface{}{
			"fields": map[string]interface{}{
				"content": map[string]interface{}{},
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
	totalValue, _ := total["value"].(float64)

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

		score, _ := hitMap["_score"].(float64)
		pcapID, _ := source["pcap_id"].(float64)

		// 获取原始Pcap记录以获取其他字段
		var pcapRecord database.Pcap
		err := config.Db.Where("id = ?", int(pcapID)).First(&pcapRecord).Error
		if err != nil {
			log.Printf("搜索结果中找不到对应的PCAP记录 (ID: %d): %v", int(pcapID), err)
			continue
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

		result := SearchResult{
			ID:         fmt.Sprintf("pcap-%d", int(pcapID)),
			PcapID:     int(pcapID),
			SrcIP:      pcapRecord.SrcIP,
			DstIP:      pcapRecord.DstIP,
			SrcPort:    pcapRecord.SrcPort,
			DstPort:    pcapRecord.DstPort,
			Tags:       pcapRecord.Tags,
			Timestamp:  pcapRecord.Time,
			Score:      score,
			Highlights: highlights,
		}
		results = append(results, result)
	}

	return results, int64(totalValue), nil
}

// buildSearchQuery 构建搜索查询
func (s *ElasticsearchService) buildSearchQuery(queryStr string) map[string]interface{} {
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
				"content": map[string]interface{}{
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
				"content": queryStr,
			},
		}
	}

	// 单个词，使用匹配查询
	return map[string]interface{}{
		"match": map[string]interface{}{
			"content": queryStr,
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
		DocumentID: fmt.Sprintf("pcap-%d", pcapID),
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
