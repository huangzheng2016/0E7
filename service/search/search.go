package search

import (
	"0E7/service/config"
	"0E7/service/database"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/standard"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
)

// FlowItem 流量项结构（避免循环导入）
type FlowItem struct {
	From string `json:"f"`
	B64  string `json:"b"`
	Time int    `json:"t"`
}

// FlowEntry 流量条目结构（避免循环导入）
type FlowEntry struct {
	Flow []FlowItem `json:"Flow"`
}

// SearchDocument 搜索文档结构
type SearchDocument struct {
	ID            string    `json:"id"`
	PcapID        int       `json:"pcap_id"`
	Content       string    `json:"content"`        // 所有内容
	ClientContent string    `json:"client_content"` // 客户端内容
	ServerContent string    `json:"server_content"` // 服务器内容
	SrcIP         string    `json:"src_ip"`
	DstIP         string    `json:"dst_ip"`
	SrcPort       string    `json:"src_port"`
	DstPort       string    `json:"dst_port"`
	Tags          string    `json:"tags"`
	Timestamp     int       `json:"timestamp"`
	Duration      int       `json:"duration"`
	NumPackets    int       `json:"num_packets"`
	Size          int       `json:"size"`
	Filename      string    `json:"filename"`
	Blocked       string    `json:"blocked"`
	CreatedAt     time.Time `json:"created_at"`
}

// SearchResult 搜索结果
type SearchResult struct {
	ID         string              `json:"id"`
	PcapID     int                 `json:"pcap_id"`
	Content    string              `json:"content"`
	SrcIP      string              `json:"src_ip"`
	DstIP      string              `json:"dst_ip"`
	SrcPort    string              `json:"src_port"`
	DstPort    string              `json:"dst_port"`
	Tags       string              `json:"tags"`
	Timestamp  int                 `json:"timestamp"`
	Duration   int                 `json:"duration"`
	NumPackets int                 `json:"num_packets"`
	Size       int                 `json:"size"`
	Filename   string              `json:"filename"`
	Blocked    string              `json:"blocked"`
	Score      float64             `json:"score"`
	Highlight  string              `json:"highlight,omitempty"`
	Highlights map[string][]string `json:"highlights,omitempty"`
}

// SearchEngine 搜索引擎类型
type SearchEngine string

const (
	SearchEngineBleve         SearchEngine = "bleve"
	SearchEngineElasticsearch SearchEngine = "elasticsearch"
)

// SearchType 搜索类型
type SearchType int

const (
	SearchTypeAll    SearchType = iota // 搜索所有内容
	SearchTypeClient                   // 只搜索客户端内容
	SearchTypeServer                   // 只搜索服务器端内容
)

// SearchService 搜索服务
type SearchService struct {
	index     bleve.Index
	mutex     sync.RWMutex
	engine    SearchEngine
	esService *ElasticsearchService
}

var (
	searchService *SearchService
	once          sync.Once
)

// GetSearchService 获取搜索服务单例
func GetSearchService() *SearchService {
	once.Do(func() {
		// 根据配置文件选择搜索引擎
		var engine SearchEngine
		switch config.Search_engine {
		case "elasticsearch":
			engine = SearchEngineElasticsearch
		case "bleve":
			fallthrough
		default:
			engine = SearchEngineBleve
		}

		searchService = &SearchService{
			engine: engine,
		}
		searchService.initIndex()
	})
	return searchService
}

// GetSearchServiceWithEngine 获取指定搜索引擎的服务
func GetSearchServiceWithEngine(engine SearchEngine) *SearchService {
	once.Do(func() {
		searchService = &SearchService{
			engine: engine,
		}
		searchService.initIndex()
	})
	return searchService
}

// initIndex 初始化索引
func (s *SearchService) initIndex() {
	switch s.engine {
	case SearchEngineElasticsearch:
		s.esService = GetElasticsearchService()
		if s.esService.IsAvailable() {
			log.Println("使用Elasticsearch搜索引擎")
		} else {
			log.Println("Elasticsearch不可用，回退到Bleve搜索引擎")
			s.engine = SearchEngineBleve
			s.initBleveIndex()
		}
	case SearchEngineBleve:
		fallthrough
	default:
		s.initBleveIndex()
	}
}

// initBleveIndex 初始化Bleve索引
func (s *SearchService) initBleveIndex() {
	indexPath := "bleve"

	// 检查索引是否存在
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		// 创建新索引
		mapping := s.createMapping()
		index, err := bleve.New(indexPath, mapping)
		if err != nil {
			log.Printf("创建Bleve搜索索引失败: %v", err)
			return
		}
		s.index = index
		log.Println("Bleve搜索索引创建成功")
	} else {
		// 打开现有索引
		index, err := bleve.Open(indexPath)
		if err != nil {
			log.Printf("打开Bleve搜索索引失败: %v", err)
			return
		}
		s.index = index
		log.Println("Bleve搜索索引加载成功")
	}
}

// createMapping 创建索引映射
func (s *SearchService) createMapping() mapping.IndexMapping {
	// 创建文档映射
	documentMapping := bleve.NewDocumentMapping()

	// 内容字段 - 使用标准分析器，支持全文搜索
	contentFieldMapping := bleve.NewTextFieldMapping()
	contentFieldMapping.Analyzer = standard.Name
	documentMapping.AddFieldMappingsAt("content", contentFieldMapping)

	// 客户端内容字段 - 使用标准分析器
	clientContentFieldMapping := bleve.NewTextFieldMapping()
	clientContentFieldMapping.Analyzer = standard.Name
	documentMapping.AddFieldMappingsAt("client_content", clientContentFieldMapping)

	// 服务器端内容字段 - 使用标准分析器
	serverContentFieldMapping := bleve.NewTextFieldMapping()
	serverContentFieldMapping.Analyzer = standard.Name
	documentMapping.AddFieldMappingsAt("server_content", serverContentFieldMapping)

	// IP地址字段 - 使用关键词分析器，精确匹配
	ipFieldMapping := bleve.NewTextFieldMapping()
	ipFieldMapping.Analyzer = keyword.Name
	documentMapping.AddFieldMappingsAt("src_ip", ipFieldMapping)
	documentMapping.AddFieldMappingsAt("dst_ip", ipFieldMapping)

	// 端口字段 - 使用关键词分析器
	portFieldMapping := bleve.NewTextFieldMapping()
	portFieldMapping.Analyzer = keyword.Name
	documentMapping.AddFieldMappingsAt("src_port", portFieldMapping)
	documentMapping.AddFieldMappingsAt("dst_port", portFieldMapping)

	// 标签字段 - 使用标准分析器
	tagsFieldMapping := bleve.NewTextFieldMapping()
	tagsFieldMapping.Analyzer = standard.Name
	documentMapping.AddFieldMappingsAt("tags", tagsFieldMapping)

	// 时间戳字段 - 数值类型
	timestampFieldMapping := bleve.NewNumericFieldMapping()
	documentMapping.AddFieldMappingsAt("timestamp", timestampFieldMapping)

	// 数值字段 - 数值类型
	numericFieldMapping := bleve.NewNumericFieldMapping()
	documentMapping.AddFieldMappingsAt("pcap_id", numericFieldMapping)
	documentMapping.AddFieldMappingsAt("duration", numericFieldMapping)
	documentMapping.AddFieldMappingsAt("num_packets", numericFieldMapping)
	documentMapping.AddFieldMappingsAt("size", numericFieldMapping)

	// 字符串字段 - 使用关键词分析器
	stringFieldMapping := bleve.NewTextFieldMapping()
	stringFieldMapping.Analyzer = keyword.Name
	documentMapping.AddFieldMappingsAt("filename", stringFieldMapping)
	documentMapping.AddFieldMappingsAt("blocked", stringFieldMapping)

	// 创建索引映射
	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultMapping = documentMapping

	return indexMapping
}

// IndexPcap 索引PCAP数据
func (s *SearchService) IndexPcap(pcapRecord database.Pcap) error {
	switch s.engine {
	case SearchEngineElasticsearch:
		if s.esService != nil && s.esService.IsAvailable() {
			return s.esService.IndexPcap(pcapRecord)
		}
		// 如果Elasticsearch不可用，回退到Bleve
		fallthrough
	case SearchEngineBleve:
		fallthrough
	default:
		return s.indexPcapBleve(pcapRecord)
	}
}

// indexPcapBleve 使用Bleve索引PCAP数据
func (s *SearchService) indexPcapBleve(pcapRecord database.Pcap) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.index == nil {
		return fmt.Errorf("Bleve搜索索引未初始化")
	}

	// 获取流量数据
	flowData, err := s.getFlowData(pcapRecord)
	if err != nil {
		log.Printf("获取流量数据失败 (ID: %d): %v", pcapRecord.ID, err)
		return err
	}

	// 分别处理客户端和服务器端内容
	var allContentBuilder, clientContentBuilder, serverContentBuilder strings.Builder

	for _, flowItem := range flowData {
		decoded, err := base64.StdEncoding.DecodeString(flowItem.B64)
		if err != nil {
			log.Printf("解码B64数据失败: %v", err)
			continue
		}

		decodedStr := string(decoded)

		// 添加到总内容
		allContentBuilder.WriteString(decodedStr)
		allContentBuilder.WriteString(" ")

		// 根据方向分别添加到客户端或服务器端内容
		if flowItem.From == "c" {
			// 客户端到服务器
			clientContentBuilder.WriteString(decodedStr)
			clientContentBuilder.WriteString(" ")
		} else if flowItem.From == "s" {
			// 服务器到客户端
			serverContentBuilder.WriteString(decodedStr)
			serverContentBuilder.WriteString(" ")
		}
	}

	// 创建搜索文档
	doc := SearchDocument{
		ID:            fmt.Sprintf("pcap_%d", pcapRecord.ID),
		PcapID:        pcapRecord.ID,
		Content:       allContentBuilder.String(),
		ClientContent: clientContentBuilder.String(),
		ServerContent: serverContentBuilder.String(),
		SrcIP:         pcapRecord.SrcIP,
		DstIP:         pcapRecord.DstIP,
		SrcPort:       pcapRecord.SrcPort,
		DstPort:       pcapRecord.DstPort,
		Tags:          pcapRecord.Tags,
		Timestamp:     pcapRecord.Time,
		Duration:      pcapRecord.Duration,
		NumPackets:    pcapRecord.NumPackets,
		Size:          pcapRecord.Size,
		Filename:      pcapRecord.Filename,
		Blocked:       pcapRecord.Blocked,
		CreatedAt:     pcapRecord.CreatedAt,
	}

	// 索引文档
	err = s.index.Index(doc.ID, doc)
	if err != nil {
		log.Printf("索引文档失败 (ID: %s): %v", doc.ID, err)
		return err
	}

	log.Printf("成功索引PCAP数据到Bleve (ID: %d)", pcapRecord.ID)
	return nil
}

// getFlowData 获取流量数据
func (s *SearchService) getFlowData(pcapRecord database.Pcap) ([]FlowItem, error) {
	if pcapRecord.FlowFile == "" {
		return nil, fmt.Errorf("流量文件路径为空")
	}

	// 读取JSON文件
	jsonData, err := os.ReadFile(pcapRecord.FlowFile)
	if err != nil {
		return nil, fmt.Errorf("读取流量文件失败: %v", err)
	}

	// 检查是否是压缩文件（通过文件扩展名或内容判断）
	var dataToParse []byte
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

	// 解析JSON
	var flowEntry FlowEntry
	err = json.Unmarshal(dataToParse, &flowEntry)
	if err != nil {
		return nil, fmt.Errorf("解析流量数据失败: %v", err)
	}

	return flowEntry.Flow, nil
}

// Search 执行搜索
func (s *SearchService) Search(query string, page, pageSize int, searchType SearchType) ([]SearchResult, int64, error) {
	return s.SearchWithPort(query, "", page, pageSize, searchType)
}

// SearchWithPort 执行带端口过滤的搜索
func (s *SearchService) SearchWithPort(query, port string, page, pageSize int, searchType SearchType) ([]SearchResult, int64, error) {
	switch s.engine {
	case SearchEngineElasticsearch:
		if s.esService != nil && s.esService.IsAvailable() {
			return s.esService.SearchWithPort(query, port, page, pageSize, searchType)
		}
		// 如果Elasticsearch不可用，回退到Bleve
		fallthrough
	case SearchEngineBleve:
		fallthrough
	default:
		return s.searchBleveWithPort(query, port, page, pageSize, searchType)
	}
}

// SearchByPcapIDs 在指定的PCAP ID范围内搜索
func (s *SearchService) SearchByPcapIDs(query string, pcapIDs []int, searchType SearchType) ([]SearchResult, error) {
	switch s.engine {
	case SearchEngineElasticsearch:
		if s.esService != nil && s.esService.IsAvailable() {
			return s.esService.SearchByPcapIDs(query, pcapIDs, searchType)
		}
		// 如果Elasticsearch不可用，回退到Bleve
		fallthrough
	case SearchEngineBleve:
		fallthrough
	default:
		return s.searchBleveByPcapIDs(query, pcapIDs, searchType)
	}
}

// searchBleve 使用Bleve执行搜索
func (s *SearchService) searchBleve(query string, page, pageSize int, searchType SearchType) ([]SearchResult, int64, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.index == nil {
		return nil, 0, fmt.Errorf("Bleve搜索索引未初始化")
	}

	// 构建搜索查询
	searchQuery := s.buildSearchQuery(query, searchType)

	// 创建搜索请求
	searchRequest := bleve.NewSearchRequest(searchQuery)
	searchRequest.Size = pageSize
	searchRequest.From = (page - 1) * pageSize
	searchRequest.SortBy([]string{"-pcap_id"}) // 按pcap_id降序排序
	searchRequest.Highlight = bleve.NewHighlight()

	// 根据搜索类型添加高亮字段
	switch searchType {
	case SearchTypeClient:
		searchRequest.Highlight.AddField("client_content")
	case SearchTypeServer:
		searchRequest.Highlight.AddField("server_content")
	default:
		searchRequest.Highlight.AddField("content")
	}

	// 执行搜索
	searchResult, err := s.index.Search(searchRequest)
	if err != nil {
		return nil, 0, fmt.Errorf("搜索执行失败: %v", err)
	}

	// 转换搜索结果
	results := make([]SearchResult, 0, len(searchResult.Hits))
	for _, hit := range searchResult.Hits {
		// 解析文档ID获取PCAP ID
		var pcapID int
		fmt.Sscanf(hit.ID, "pcap_%d", &pcapID)

		result := SearchResult{
			ID:         hit.ID,
			PcapID:     pcapID,
			Score:      hit.Score,
			Highlights: hit.Fragments,
		}

		// 从数据库获取完整信息
		var pcapRecord database.Pcap
		err := config.Db.Where("id = ?", pcapID).First(&pcapRecord).Error
		if err == nil {
			result.SrcIP = pcapRecord.SrcIP
			result.DstIP = pcapRecord.DstIP
			result.SrcPort = pcapRecord.SrcPort
			result.DstPort = pcapRecord.DstPort
			result.Tags = pcapRecord.Tags
			result.Timestamp = pcapRecord.Time
			result.Duration = pcapRecord.Duration
			result.NumPackets = pcapRecord.NumPackets
			result.Size = pcapRecord.Size
			result.Filename = pcapRecord.Filename
			result.Blocked = pcapRecord.Blocked
		}

		results = append(results, result)
	}

	return results, int64(searchResult.Total), nil
}

// searchBleveWithPort 使用Bleve执行带端口过滤的搜索
func (s *SearchService) searchBleveWithPort(query, port string, page, pageSize int, searchType SearchType) ([]SearchResult, int64, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.index == nil {
		return nil, 0, fmt.Errorf("Bleve搜索索引未初始化")
	}

	// 构建搜索查询
	searchQuery := s.buildSearchQuery(query, searchType)

	// 如果指定了端口，添加端口过滤条件
	if port != "" {
		// 创建端口过滤查询
		portQuery := bleve.NewBooleanQuery()
		srcPortQuery := bleve.NewTermQuery(port)
		srcPortQuery.SetField("src_port")
		dstPortQuery := bleve.NewTermQuery(port)
		dstPortQuery.SetField("dst_port")
		portQuery.AddShould(srcPortQuery)
		portQuery.AddShould(dstPortQuery)
		portQuery.SetMinShould(1) // 至少匹配一个端口字段

		// 将端口查询与主查询组合
		combinedQuery := bleve.NewBooleanQuery()
		combinedQuery.AddMust(searchQuery)
		combinedQuery.AddMust(portQuery)

		searchQuery = combinedQuery
	}

	// 创建搜索请求
	searchRequest := bleve.NewSearchRequest(searchQuery)
	searchRequest.Size = pageSize
	searchRequest.From = (page - 1) * pageSize
	searchRequest.SortBy([]string{"-pcap_id"}) // 按pcap_id降序排序
	searchRequest.Fields = []string{"*"}       // 返回所有字段
	searchRequest.Highlight = bleve.NewHighlight()

	// 根据搜索类型添加高亮字段
	switch searchType {
	case SearchTypeClient:
		searchRequest.Highlight.AddField("client_content")
	case SearchTypeServer:
		searchRequest.Highlight.AddField("server_content")
	default:
		searchRequest.Highlight.AddField("content")
		searchRequest.Highlight.AddField("client_content")
		searchRequest.Highlight.AddField("server_content")
	}

	// 执行搜索
	searchResult, err := s.index.Search(searchRequest)
	if err != nil {
		return nil, 0, err
	}

	// 转换搜索结果
	var results []SearchResult
	for _, hit := range searchResult.Hits {
		// 安全地获取字段值，避免nil指针异常
		getString := func(field string) string {
			if val, ok := hit.Fields[field]; ok && val != nil {
				if str, ok := val.(string); ok {
					return str
				}
			}
			return ""
		}

		getInt := func(field string) int {
			if val, ok := hit.Fields[field]; ok && val != nil {
				if f, ok := val.(float64); ok {
					return int(f)
				}
			}
			return 0
		}

		getBool := func(field string) bool {
			if val, ok := hit.Fields[field]; ok && val != nil {
				if b, ok := val.(bool); ok {
					return b
				}
			}
			return false
		}

		result := SearchResult{
			ID:         hit.ID,
			PcapID:     0, // 从ID中提取
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
			Blocked:    fmt.Sprintf("%t", getBool("blocked")),
			Score:      hit.Score,
		}

		// 从ID中提取PcapID
		if strings.HasPrefix(result.ID, "pcap_") {
			if pcapID, err := strconv.Atoi(strings.TrimPrefix(result.ID, "pcap_")); err == nil {
				result.PcapID = pcapID
			}
		}

		// 添加高亮信息
		if len(hit.Fragments) > 0 {
			highlights := make([]string, 0)
			for field, fragments := range hit.Fragments {
				if len(fragments) > 0 {
					highlights = append(highlights, fmt.Sprintf("%s: %s", field, strings.Join(fragments, " ... ")))
				}
			}
			result.Highlight = strings.Join(highlights, " | ")
		}

		results = append(results, result)
	}

	return results, int64(searchResult.Total), nil
}

// escapeRegexSpecialChars 转义正则表达式特殊字符，但保留通配符
func (s *SearchService) escapeRegexSpecialChars(str string) string {
	// 需要转义的正则表达式特殊字符
	specialChars := []string{".", "+", "^", "$", "(", ")", "[", "]", "{", "}", "|", "\\"}

	result := str
	for _, char := range specialChars {
		result = strings.ReplaceAll(result, char, "\\"+char)
	}

	return result
}

// buildSearchQuery 构建搜索查询
func (s *SearchService) buildSearchQuery(queryStr string, searchType SearchType) query.Query {
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
		matchAllQuery := bleve.NewMatchAllQuery()
		return matchAllQuery
	}

	// 检查是否是正则表达式查询
	if strings.HasPrefix(queryStr, "/") && strings.HasSuffix(queryStr, "/") {
		// 正则表达式查询
		regexPattern := strings.Trim(queryStr, "/")
		regexQuery := bleve.NewRegexpQuery(regexPattern)
		regexQuery.SetField(searchField)
		return regexQuery
	}

	// 检查是否包含特殊字符，决定使用短语查询还是匹配查询
	if strings.Contains(queryStr, " ") {
		// 包含空格，使用短语查询
		phraseQuery := bleve.NewPhraseQuery(strings.Fields(queryStr), searchField)
		return phraseQuery
	}

	// 单个词，使用匹配查询
	matchQuery := bleve.NewMatchQuery(queryStr)
	matchQuery.SetField(searchField)
	return matchQuery
}

// DeletePcap 删除PCAP索引
func (s *SearchService) DeletePcap(pcapID int) error {
	switch s.engine {
	case SearchEngineElasticsearch:
		if s.esService != nil && s.esService.IsAvailable() {
			return s.esService.DeletePcap(pcapID)
		}
		// 如果Elasticsearch不可用，回退到Bleve
		fallthrough
	case SearchEngineBleve:
		fallthrough
	default:
		return s.deletePcapBleve(pcapID)
	}
}

// deletePcapBleve 使用Bleve删除PCAP索引
func (s *SearchService) deletePcapBleve(pcapID int) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.index == nil {
		return fmt.Errorf("Bleve搜索索引未初始化")
	}

	docID := fmt.Sprintf("pcap_%d", pcapID)
	err := s.index.Delete(docID)
	if err != nil {
		log.Printf("删除索引文档失败 (ID: %s): %v", docID, err)
		return err
	}

	log.Printf("成功删除PCAP索引 (ID: %d)", pcapID)
	return nil
}

// GetIndexStats 获取索引统计信息
func (s *SearchService) GetIndexStats() (map[string]interface{}, error) {
	switch s.engine {
	case SearchEngineElasticsearch:
		if s.esService != nil && s.esService.IsAvailable() {
			return s.esService.GetIndexStats()
		}
		// 如果Elasticsearch不可用，回退到Bleve
		fallthrough
	case SearchEngineBleve:
		fallthrough
	default:
		return s.getIndexStatsBleve()
	}
}

// getIndexStatsBleve 使用Bleve获取索引统计信息
func (s *SearchService) getIndexStatsBleve() (map[string]interface{}, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.index == nil {
		return nil, fmt.Errorf("Bleve搜索索引未初始化")
	}

	// 获取索引统计
	stats := make(map[string]interface{})

	// 获取文档数量
	docCount, err := s.index.DocCount()
	if err != nil {
		return nil, fmt.Errorf("获取文档数量失败: %v", err)
	}
	stats["doc_count"] = docCount

	// 获取索引大小
	indexPath := "bleve"
	if stat, err := os.Stat(indexPath); err == nil {
		stats["index_size"] = stat.Size()
	}

	return stats, nil
}

// Engine 获取当前使用的搜索引擎
func (s *SearchService) Engine() SearchEngine {
	return s.engine
}

// Close 关闭搜索服务
func (s *SearchService) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.index != nil {
		log.Println("正在关闭Bleve搜索索引...")
		err := s.index.Close()
		if err != nil {
			log.Printf("关闭Bleve搜索索引失败: %v", err)
			return err
		}
		s.index = nil
		log.Println("Bleve搜索索引已关闭")
	}
	return nil
}

// searchBleveByPcapIDs 在指定的PCAP ID范围内使用Bleve执行搜索
func (s *SearchService) searchBleveByPcapIDs(queryStr string, pcapIDs []int, searchType SearchType) ([]SearchResult, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.index == nil {
		return nil, fmt.Errorf("Bleve搜索索引未初始化")
	}

	// 构建搜索查询
	searchQuery := s.buildSearchQuery(queryStr, searchType)

	// 创建ID范围查询
	var idQueries []query.Query
	for _, pcapID := range pcapIDs {
		idQuery := query.NewTermQuery(fmt.Sprintf("%d", pcapID))
		idQuery.SetField("pcap_id")
		idQueries = append(idQueries, idQuery)
	}

	// 组合查询：flag查询 AND (ID1 OR ID2 OR ...)
	if len(idQueries) > 0 {
		idOrQuery := query.NewBooleanQuery(idQueries, nil, nil)
		combinedQuery := query.NewBooleanQuery([]query.Query{searchQuery, idOrQuery}, nil, nil)
		searchQuery = combinedQuery
	}

	// 执行搜索
	searchRequest := bleve.NewSearchRequest(searchQuery)
	searchRequest.Size = 10000                 // 设置足够大的size以获取所有结果
	searchRequest.SortBy([]string{"-pcap_id"}) // 按pcap_id降序排序
	searchRequest.Fields = []string{"*"}

	searchResult, err := s.index.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("bleve搜索失败: %v", err)
	}

	// 转换结果
	var results []SearchResult
	for _, hit := range searchResult.Hits {
		// 安全地获取字段值，避免nil指针异常
		getString := func(field string) string {
			if val, ok := hit.Fields[field]; ok && val != nil {
				if str, ok := val.(string); ok {
					return str
				}
			}
			return ""
		}

		getInt := func(field string) int {
			if val, ok := hit.Fields[field]; ok && val != nil {
				if i, ok := val.(int); ok {
					return i
				}
				if f, ok := val.(float64); ok {
					return int(f)
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
			Score:      hit.Score,
		}
		results = append(results, result)
	}

	return results, nil
}
