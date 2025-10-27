package flag

import (
	"0E7/service/config"
	"0E7/service/database"
	"0E7/service/search"
	"encoding/json"
	"log"
	"regexp"
	"runtime"
	"sync"
	"time"
)

// ReindexProgress 重新索引进度
type ReindexProgress struct {
	TotalRecords     int     `json:"total_records"`
	ProcessedRecords int     `json:"processed_records"`
	IsRunning        bool    `json:"is_running"`
	StartTime        int64   `json:"start_time"`
	CurrentPattern   string  `json:"current_pattern"`
	Percentage       float64 `json:"percentage"`
}

// FlagDetector flag检测器
type FlagDetector struct {
	mutex           sync.RWMutex
	stopChan        chan bool
	lastFlagPattern string           // 记录上次的flag模式
	reindexChan     chan bool        // 用于触发重新索引
	progress        *ReindexProgress // 重新索引进度
}

var (
	flagDetector *FlagDetector
	once         sync.Once
)

// GetFlagDetector 获取flag检测器单例
func GetFlagDetector() *FlagDetector {
	once.Do(func() {
		flagDetector = &FlagDetector{
			stopChan:        make(chan bool),
			reindexChan:     make(chan bool, 1),
			lastFlagPattern: config.Server_flag, // 初始化时记录当前flag模式
			progress:        &ReindexProgress{IsRunning: false},
		}
		flagDetector.startDetection()
	})
	return flagDetector
}

// startDetection 开始检测
func (fd *FlagDetector) startDetection() {
	go fd.detectionLoop()
}

// detectionLoop 检测循环
func (fd *FlagDetector) detectionLoop() {
	pendingTicker := time.NewTicker(5 * time.Second)
	defer pendingTicker.Stop()

	// 每2分钟检查一次flag模式是否变化
	configTicker := time.NewTicker(2 * time.Minute)
	defer configTicker.Stop()

	for {
		select {
		case <-pendingTicker.C:
			// 检测PENDING标签的流量
			fd.detectPendingFlags()
		case <-configTicker.C:
			// 检查flag模式是否变化
			if config.Server_flag != fd.lastFlagPattern {
				log.Printf("检测到flag模式变化: %s -> %s", fd.lastFlagPattern, config.Server_flag)
				fd.lastFlagPattern = config.Server_flag
				// 触发重新索引
				fd.TriggerReindex()
			}
		case <-fd.reindexChan:
			// 当flag模式更新时，触发重新索引
			fd.reindexWithNewPattern()
		case <-fd.stopChan:
			return
		}
	}
}

// detectPendingFlags 检测PENDING标签的流量
func (fd *FlagDetector) detectPendingFlags() {
	// 获取所有PENDING标签的流量，每5秒处理5000条，优化内存使用
	// 使用预加载减少数据库查询次数
	var pcaps []database.Pcap
	err := config.Db.Where("tags LIKE ?", "%PENDING%").Limit(5000).Find(&pcaps).Error
	if err != nil {
		log.Printf("获取PENDING流量失败: %v", err)
		return
	}

	if len(pcaps) == 0 {
		return // 没有PENDING流量
	}

	log.Printf("检测到 %d 条PENDING流量，开始批量处理", len(pcaps))

	// 检查是否有flag正则表达式
	var flagRegex *regexp.Regexp
	if config.Server_flag != "" {
		flagRegex, err = regexp.Compile(config.Server_flag)
		if err != nil {
			log.Printf("编译flag正则表达式失败: %v", err)
			flagRegex = nil // 设置为nil，只进行索引不检测flag
		}
	}

	// 准备批量索引的记录和批量更新的数据
	var recordsToIndex []database.Pcap
	var updateData []map[string]interface{} // 批量更新数据

	// 处理每个PENDING记录
	for _, pcap := range pcaps {
		var hasClientFlag, hasServerFlag bool

		// 如果有flag正则表达式，进行flag检测
		if flagRegex != nil {
			hasClientFlag = flagRegex.MatchString(pcap.ClientContent)
			hasServerFlag = flagRegex.MatchString(pcap.ServerContent)
		}

		// 更新标签
		var tags []string
		if err := json.Unmarshal([]byte(pcap.Tags), &tags); err != nil {
			log.Printf("解析标签失败 (ID: %d): %v", pcap.ID, err)
			continue
		}

		// 移除PENDING标签
		newTags := make([]string, 0)
		for _, tag := range tags {
			if tag != "PENDING" && tag != "FLAG-IN" && tag != "FLAG-OUT" {
				newTags = append(newTags, tag)
			}
		}

		// 如果有flag正则表达式且检测到flag，添加方向标签
		if flagRegex != nil {
			if hasClientFlag {
				newTags = append(newTags, "FLAG-IN")
			}
			if hasServerFlag {
				newTags = append(newTags, "FLAG-OUT")
			}
		}

		// 只有在检测到flag时才显示日志
		if flagRegex != nil && (hasClientFlag || hasServerFlag) {
			log.Printf("在流量ID %d 中检测到flag (客户端: %v, 服务器端: %v)",
				pcap.ID, hasClientFlag, hasServerFlag)
		}

		// 准备批量更新数据
		newTagsJSON, err := json.Marshal(newTags)
		if err != nil {
			log.Printf("序列化标签失败 (ID: %d): %v", pcap.ID, err)
			continue
		}

		// 添加到批量更新列表
		updateData = append(updateData, map[string]interface{}{
			"id":   pcap.ID,
			"tags": string(newTagsJSON),
		})

		// 更新pcap记录的tags字段
		pcap.Tags = string(newTagsJSON)
		recordsToIndex = append(recordsToIndex, pcap)
	}

	// 执行批量更新标签，使用 GORM 优雅的批量更新方式
	if len(updateData) > 0 {
		// 使用事务确保批量更新的原子性
		tx := config.Db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		// 分批处理批量更新，使用 GORM 的 Save 方法进行批量更新
		batchUpdateSize := 200
		successCount := 0

		for i := 0; i < len(updateData); i += batchUpdateSize {
			end := i + batchUpdateSize
			if end > len(updateData) {
				end = len(updateData)
			}

			batch := updateData[i:end]

			// 构建批量更新的 Pcap 对象
			var pcapUpdates []database.Pcap
			for _, data := range batch {
				pcapUpdates = append(pcapUpdates, database.Pcap{
					ID:   data["id"].(int),
					Tags: data["tags"].(string),
				})
			}

			// 使用 GORM 的 Save 方法进行批量更新
			err := tx.Save(&pcapUpdates).Error
			if err != nil {
				tx.Rollback()
				log.Printf("批量更新标签失败 (批次 %d-%d): %v", i, end-1, err)
				return
			}

			successCount += len(pcapUpdates)
			log.Printf("成功更新批次 %d-%d，共 %d 条记录", i, end-1, len(pcapUpdates))
		}

		// 最终提交
		if err := tx.Commit().Error; err != nil {
			log.Printf("最终提交批量更新事务失败: %v", err)
		} else {
			log.Printf("成功批量更新 %d 条记录的标签", successCount)
		}
	}

	// 批量索引所有处理完的记录，优化内存使用
	if len(recordsToIndex) > 0 {
		searchService := search.GetSearchService()

		// 分批处理，避免内存占用过大，减少磁盘 I/O
		batchSize := 2000 // 增大批次，减少数据库查询次数
		for i := 0; i < len(recordsToIndex); i += batchSize {
			end := i + batchSize
			if end > len(recordsToIndex) {
				end = len(recordsToIndex)
			}

			batch := recordsToIndex[i:end]
			err := searchService.BatchIndexPcaps(batch)
			if err != nil {
				log.Printf("批量索引失败 (批次 %d-%d): %v", i, end-1, err)
			} else {
				log.Printf("成功批量索引 %d 条记录到 bleve (批次 %d-%d)", len(batch), i, end-1)
			}

			// 强制垃圾回收，释放内存，减少内存压力
			if i%4000 == 0 { // 减少 GC 频率，提高性能
				runtime.GC()
			}
		}
	}
}

// reindexWithNewPattern 使用新flag模式全量重新索引
func (fd *FlagDetector) reindexWithNewPattern() {
	fd.mutex.Lock()
	fd.progress.IsRunning = true
	fd.progress.StartTime = time.Now().Unix()
	fd.progress.CurrentPattern = config.Server_flag
	fd.progress.ProcessedRecords = 0
	fd.progress.Percentage = 0
	fd.mutex.Unlock()

	log.Printf("开始使用新flag模式全量重新索引: %s", config.Server_flag)

	// 1. 获取总记录数（所有PCAP记录，不仅仅是已标记包含flag的）
	var totalCount int64
	err := config.Db.Model(&database.Pcap{}).Count(&totalCount).Error
	if err != nil {
		log.Printf("获取总记录数失败: %v", err)
		fd.mutex.Lock()
		fd.progress.IsRunning = false
		fd.mutex.Unlock()
		return
	}

	fd.mutex.Lock()
	fd.progress.TotalRecords = int(totalCount)
	fd.mutex.Unlock()

	// 2. 批量将所有PCAP记录状态改为PENDING，让worker异步处理
	// 这样新的flag模式可以重新扫描所有数据，发现之前没扫到的flag
	batchSize := 1000
	offset := 0
	processedCount := 0

	for {
		// 分批更新标签为PENDING（所有记录，不仅仅是已标记包含flag的）
		// 查找不包含PENDING标签的记录，并添加PENDING标签
		var pcaps []database.Pcap
		err := config.Db.Where("tags NOT LIKE ?", "%PENDING%").
			Offset(offset).Limit(batchSize).Find(&pcaps).Error

		if err != nil {
			log.Printf("查询PCAP记录失败: %v", err)
			break
		}

		if len(pcaps) == 0 {
			break
		}

		// 为每条记录添加PENDING标签
		for _, pcap := range pcaps {
			var tags []string
			if pcap.Tags != "" && pcap.Tags != "[]" {
				json.Unmarshal([]byte(pcap.Tags), &tags)
			}

			// 检查是否已经有PENDING标签
			hasPending := false
			for _, tag := range tags {
				if tag == "PENDING" {
					hasPending = true
					break
				}
			}

			// 如果没有PENDING标签，则添加
			if !hasPending {
				tags = append(tags, "PENDING")
				tagsJSON, _ := json.Marshal(tags)
				config.Db.Model(&pcap).Update("tags", string(tagsJSON))
			}
		}

		updatedCount := len(pcaps)
		processedCount += updatedCount

		// 更新进度
		fd.mutex.Lock()
		fd.progress.ProcessedRecords = processedCount
		if fd.progress.TotalRecords > 0 {
			fd.progress.Percentage = float64(fd.progress.ProcessedRecords) / float64(fd.progress.TotalRecords) * 100
		}
		fd.mutex.Unlock()

		log.Printf("已标记 %d/%d 条记录为PENDING状态 (%.1f%%)",
			processedCount, totalCount, fd.progress.Percentage)

		if updatedCount < batchSize {
			break // 没有更多需要更新的记录
		}
		offset += batchSize
	}

	// 3. 完成状态更新，worker会自动处理PENDING状态的记录
	fd.mutex.Lock()
	fd.progress.IsRunning = false
	fd.progress.Percentage = 100
	fd.mutex.Unlock()

	log.Printf("flag模式重新索引初始化完成，共标记 %d 条记录为PENDING状态，worker将异步重新扫描所有数据", processedCount)
}

// TriggerReindex 触发重新索引（当flag模式更新时调用）
func (fd *FlagDetector) TriggerReindex() {
	select {
	case fd.reindexChan <- true:
		// 成功发送重新索引信号
	default:
		// 如果通道已满，忽略
	}
}

// GetReindexProgress 获取重新索引进度
func (fd *FlagDetector) GetReindexProgress() *ReindexProgress {
	fd.mutex.RLock()
	defer fd.mutex.RUnlock()

	// 返回进度的副本
	return &ReindexProgress{
		TotalRecords:     fd.progress.TotalRecords,
		ProcessedRecords: fd.progress.ProcessedRecords,
		IsRunning:        fd.progress.IsRunning,
		StartTime:        fd.progress.StartTime,
		CurrentPattern:   fd.progress.CurrentPattern,
		Percentage:       fd.progress.Percentage,
	}
}

// Stop 停止检测
func (fd *FlagDetector) Stop() {
	close(fd.stopChan)
	log.Println("FlagDetector已停止")
}
