package webui

import (
	"0E7/service/config"
	"0E7/service/database"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// 获取flag列表
func GetFlagList(c *gin.Context) {
	// 获取查询参数
	page, _ := strconv.Atoi(c.DefaultPostForm("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultPostForm("page_size", "20"))
	flag := c.PostForm("flag")
	team := c.PostForm("team")
	status := c.PostForm("status")
	exploitId := c.PostForm("exploit_id")

	// 构建查询条件 - 使用JOIN获取exploit_name
	query := config.Db.Table("`0e7_flag` f").
		Select("f.*, e.name as exploit_name").
		Joins("LEFT JOIN `0e7_exploit` e ON f.exploit_id = e.id")

	// 添加搜索条件
	if flag != "" {
		query = query.Where("f.flag LIKE ?", "%"+flag+"%")
	}
	if team != "" {
		query = query.Where("f.team LIKE ?", "%"+team+"%")
	}
	if status != "" {
		query = query.Where("f.status = ?", status)
	}
	if exploitId != "" {
		query = query.Where("f.exploit_id = ?", exploitId)
	}

	// 获取总数
	var total int64
	err := query.Count(&total).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "fail",
			"error":   "获取flag总数失败: " + err.Error(),
		})
		return
	}

	// 分页查询
	type FlagWithExploitName struct {
		database.Flag
		ExploitName *string `json:"exploit_name" gorm:"column:exploit_name"`
	}

	var flags []FlagWithExploitName
	offset := (page - 1) * pageSize
	err = query.Order("f.created_at DESC").Offset(offset).Limit(pageSize).Find(&flags).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "fail",
			"error":   "获取flag列表失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"result": gin.H{
			"flags":     flags,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// 手动提交flag
func SubmitFlag(c *gin.Context) {
	flagText := strings.TrimSpace(c.PostForm("flag"))
	team := strings.TrimSpace(c.PostForm("team"))
	flagRegex := strings.TrimSpace(c.PostForm("flag_regex"))

	if flagText == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "fail",
			"error":   "flag不能为空",
		})
		return
	}

	// 如果没有提供team，使用默认值
	if team == "" {
		team = "manual"
	}

	// 如果没有提供flag正则，使用服务器默认的
	if flagRegex == "" {
		flagRegex = config.Server_flag
	}

	// 解析flag文本，支持多个flag（每行一个或逗号分隔）
	var flags []string

	// 首先按行分割
	lines := strings.Split(flagText, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 检查是否包含逗号，如果包含则按逗号分割
		if strings.Contains(line, ",") {
			commaFlags := strings.Split(line, ",")
			for _, flag := range commaFlags {
				flag = strings.TrimSpace(flag)
				if flag != "" {
					flags = append(flags, flag)
				}
			}
		} else {
			// 不包含逗号，直接作为单个flag
			flags = append(flags, line)
		}
	}

	// 限制最多999条
	if len(flags) > 999 {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "fail",
			"error":   "最多只能提交999个flag",
		})
		return
	}

	var results []gin.H
	var successCount, skippedCount int

	for _, flag := range flags {
		// 检查flag是否已存在
		var count int64
		err := config.Db.Model(&database.Flag{}).Where("flag = ?", flag).Count(&count).Error
		if err != nil {
			results = append(results, gin.H{
				"flag":   flag,
				"status": "ERROR",
				"msg":    "检查flag失败: " + err.Error(),
			})
			continue
		}

		// 创建flag记录
		flagRecord := database.Flag{
			ExploitId: 0, // 手动提交的flag，exploit_id设为0
			Flag:      flag,
			Status:    "QUEUE",
			Team:      team,
			Msg:       "",
		}

		if count == 0 {
			// flag不存在，状态设为QUEUE
			flagRecord.Status = "QUEUE"
			successCount++
		} else {
			// flag已存在，状态设为SKIPPED
			flagRecord.Status = "SKIPPED"
			flagRecord.Msg = "Flag已存在"
			skippedCount++
		}

		err = config.Db.Create(&flagRecord).Error
		if err != nil {
			results = append(results, gin.H{
				"flag":   flag,
				"status": "ERROR",
				"msg":    "创建flag记录失败: " + err.Error(),
			})
		} else {
			results = append(results, gin.H{
				"id":     flagRecord.ID,
				"flag":   flagRecord.Flag,
				"status": flagRecord.Status,
				"msg":    flagRecord.Msg,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"result": gin.H{
			"total":   len(flags),
			"success": successCount,
			"skipped": skippedCount,
			"error":   len(flags) - successCount - skippedCount,
			"details": results,
		},
	})
}

// 删除flag
func DeleteFlag(c *gin.Context) {
	idStr := c.PostForm("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "fail",
			"error":   "无效的flag ID",
		})
		return
	}

	// 查找flag
	var flag database.Flag
	err = config.Db.First(&flag, id).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "fail",
			"error":   "flag不存在",
		})
		return
	}

	// 删除flag
	err = config.Db.Delete(&flag).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "fail",
			"error":   "删除flag失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"result": gin.H{
			"id": flag.ID,
		},
	})
}

// 获取flag统计信息
func GetFlagStats(c *gin.Context) {
	var stats struct {
		Total   int64 `json:"total"`
		Queue   int64 `json:"queue"`
		Success int64 `json:"success"`
		Failed  int64 `json:"failed"`
		Skipped int64 `json:"skipped"`
	}

	// 获取各种状态的统计
	config.Db.Model(&database.Flag{}).Count(&stats.Total)
	config.Db.Model(&database.Flag{}).Where("status = ?", "QUEUE").Count(&stats.Queue)
	config.Db.Model(&database.Flag{}).Where("status = ?", "SUCCESS").Count(&stats.Success)
	config.Db.Model(&database.Flag{}).Where("status = ?", "FAILED").Count(&stats.Failed)
	config.Db.Model(&database.Flag{}).Where("status = ?", "SKIPPED").Count(&stats.Skipped)

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"result":  stats,
	})
}

// 批量更新flag状态
func BatchUpdateFlagStatus(c *gin.Context) {
	var request struct {
		IDs    []int  `json:"ids" binding:"required"`
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "fail",
			"error":   "请求参数错误: " + err.Error(),
		})
		return
	}

	// 验证状态值
	validStatuses := []string{"QUEUE", "SUCCESS", "FAILED", "SKIPPED"}
	validStatus := false
	for _, status := range validStatuses {
		if request.Status == status {
			validStatus = true
			break
		}
	}
	if !validStatus {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "fail",
			"error":   "无效的状态值",
		})
		return
	}

	// 批量更新
	err := config.Db.Model(&database.Flag{}).Where("id IN ?", request.IDs).Update("status", request.Status).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "fail",
			"error":   "批量更新失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"result": gin.H{
			"updated_count": len(request.IDs),
		},
	})
}
