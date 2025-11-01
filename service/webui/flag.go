package webui

import (
	"0E7/service/config"
	"0E7/service/database"
	"0E7/service/flag"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gopkg.in/ini.v1"
)

// GetFlagList 获取flag列表
func GetFlagList(c *gin.Context) {
	// 获取分页参数
	pageStr := c.Query("page")
	pageSizeStr := c.Query("page_size")

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

	// 获取搜索条件
	flag := c.Query("flag")
	team := c.Query("team")
	status := c.Query("status")
	exploitIdStr := c.Query("exploit_id")

	// 构建查询条件
	query := config.Db.Model(&database.Flag{})

	if flag != "" {
		query = query.Where("flag LIKE ?", "%"+flag+"%")
	}
	if team != "" {
		query = query.Where("team LIKE ?", "%"+team+"%")
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if exploitIdStr != "" {
		if exploitId, err := strconv.Atoi(exploitIdStr); err == nil {
			query = query.Where("exploit_id = ?", exploitId)
		}
	}

	// 获取总数
	var total int64
	err := query.Count(&total).Error
	if err != nil {
		c.JSON(500, gin.H{
			"message": "fail",
			"error":   "获取flag总数失败: " + err.Error(),
		})
		return
	}

	// 获取分页数据
	var flags []database.Flag

	// 确保分页参数正确
	if pageSize <= 0 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}

	// 计算offset
	offset := (page - 1) * pageSize

	// 执行查询
	err = query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&flags).Error
	if err != nil {
		c.JSON(500, gin.H{
			"message": "fail",
			"error":   "获取flag列表失败: " + err.Error(),
		})
		return
	}

	// 获取exploit名称
	// 收集唯一的 ExploitId
	exploitIDSet := make(map[int]struct{})
	for i := range flags {
		if flags[i].ExploitId > 0 {
			exploitIDSet[flags[i].ExploitId] = struct{}{}
		}
	}

	// 如果有需要查询的 ExploitId，则批量查询
	if len(exploitIDSet) > 0 {
		ids := make([]int, 0, len(exploitIDSet))
		for id := range exploitIDSet {
			ids = append(ids, id)
		}

		var exploits []database.Exploit
		if err := config.Db.Where("id IN ?", ids).Find(&exploits).Error; err == nil {
			// 构建 id -> name 映射
			idToName := make(map[int]string, len(exploits))
			for i := range exploits {
				idToName[exploits[i].ID] = exploits[i].Name
			}
			// 回填
			for i := range flags {
				if name, ok := idToName[flags[i].ExploitId]; ok {
					flags[i].ExploitName = name
				}
			}
		}
	}

	c.JSON(200, gin.H{
		"message": "success",
		"result": gin.H{
			"flags": flags,
			"total": total,
		},
	})
}

// SubmitFlag 提交flag
func SubmitFlag(c *gin.Context) {
	flagValue := c.PostForm("flag")
	team := c.PostForm("team")
	flagRegex := c.PostForm("flag_regex")

	if flagValue == "" {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "flag不能为空",
		})
		return
	}

	// 确定使用的flag正则表达式
	regexPattern := flagRegex
	if regexPattern == "" {
		regexPattern = config.Server_flag
	}

	if regexPattern == "" {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "未设置flag正则表达式",
		})
		return
	}

	// 编译正则表达式
	regex, err := regexp.Compile(regexPattern)
	if err != nil {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "flag正则表达式无效: " + err.Error(),
		})
		return
	}

	// 使用正则表达式匹配flag
	var flags []string
	matches := regex.FindAllString(flagValue, -1)
	for _, match := range matches {
		match = strings.TrimSpace(match)
		if match != "" {
			flags = append(flags, match)
		}
	}

	// 限制数量
	if len(flags) > 999 {
		flags = flags[:999]
	}

	if len(flags) == 0 {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "没有匹配到有效的flag",
		})
		return
	}

	// 统计结果
	var total, success, skipped, error int

	// 批量处理flag
	for _, flag := range flags {
		total++

		// 检查是否已存在
		var count int64
		err := config.Db.Model(&database.Flag{}).Where("flag = ?", flag).Count(&count).Error
		if err != nil {
			error++
			continue
		}

		// 创建flag记录
		flagRecord := database.Flag{
			Flag:   flag,
			Team:   team,
			Status: "QUEUE",
		}

		if count > 0 {
			flagRecord.Status = "SKIPPED"
			skipped++
		} else {
			success++
		}

		err = config.Db.Create(&flagRecord).Error
		if err != nil {
			error++
			success--
			continue
		}
	}

	c.JSON(200, gin.H{
		"message": "success",
		"result": gin.H{
			"total":   total,
			"success": success,
			"skipped": skipped,
			"error":   error,
		},
	})
}

// DeleteFlag 删除flag
func DeleteFlag(c *gin.Context) {
	idStr := c.PostForm("id")
	if idStr == "" {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "id不能为空",
		})
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "无效的id",
		})
		return
	}

	err = config.Db.Delete(&database.Flag{}, id).Error
	if err != nil {
		c.JSON(500, gin.H{
			"message": "fail",
			"error":   "删除flag失败: " + err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"message": "success",
		"result":  "flag删除成功",
	})
}

// UpdateFlagConfig 更新flag配置
func UpdateFlagConfig(c *gin.Context) {
	newPattern := c.PostForm("pattern")
	if newPattern == "" {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "flag模式不能为空",
		})
		return
	}

	// 检查模式是否发生变化
	if config.Server_flag == newPattern {
		c.JSON(200, gin.H{
			"message": "success",
			"result":  "flag配置未发生变化",
		})
		return
	}

	// 更新config.ini文件
	err := updateFlagConfigInFile(newPattern)
	if err != nil {
		c.JSON(500, gin.H{
			"message": "fail",
			"error":   "更新flag配置失败: " + err.Error(),
		})
		return
	}

	// 更新内存中的配置
	config.Server_flag = newPattern

	// 触发重新索引
	flagDetector := flag.GetFlagDetector()
	flagDetector.TriggerReindex()

	c.JSON(200, gin.H{
		"message": "success",
		"result":  "flag配置更新成功，正在重新索引历史数据",
	})
}

// updateFlagConfigInFile 更新config.ini文件中的flag配置
func updateFlagConfigInFile(newPattern string) error {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		return fmt.Errorf("failed to load config.ini: %v", err)
	}

	// 更新server section中的flag值
	serverSection := cfg.Section("server")
	serverSection.Key("flag").SetValue(newPattern)

	// 保存文件
	err = cfg.SaveTo("config.ini")
	if err != nil {
		return fmt.Errorf("failed to save config.ini: %v", err)
	}

	return nil
}

// GetCurrentFlagConfig 获取当前flag配置
func GetCurrentFlagConfig(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "success",
		"result": gin.H{
			"current_pattern": config.Server_flag,
		},
	})
}
