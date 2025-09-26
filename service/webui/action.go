package webui

import (
	"0E7/service/config"
	"0E7/service/database"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func action(c *gin.Context) {
	var err error
	id := c.PostForm("id")
	name := c.PostForm("name")
	code := c.PostForm("code")
	output := c.PostForm("output")
	interval := c.PostForm("interval")
	timeout := c.PostForm("timeout")
	configStr := c.PostForm("config")

	if code != "" {
		match := regexp.MustCompile(`^data:(code\/(?:python2|python3|golang));base64,(.*)$`).FindStringSubmatch(code)
		if match == nil {
			c.JSON(400, gin.H{
				"message": "fail",
				"error":   "code format error",
			})
			c.Abort()
			return
		}
	}

	intervalInt, _ := strconv.Atoi(interval)
	timeoutInt, _ := strconv.Atoi(timeout)

	// 限制超时时间在 0-60 秒之间
	if timeoutInt < 0 {
		timeoutInt = 0
	} else if timeoutInt > 60 {
		timeoutInt = 60
	}

	actionRecord := database.Action{
		Name:     name,
		Code:     code,
		Output:   output,
		Config:   configStr,
		Status:   "PENDING",
		Interval: intervalInt,
		Timeout:  timeoutInt,
	}

	if id == "" {
		err = config.Db.Create(&actionRecord).Error
	} else {
		idInt, _ := strconv.Atoi(id)
		actionRecord.ID = idInt
		err = config.Db.Save(&actionRecord).Error
	}

	if err != nil {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   err.Error(),
		})
		c.Abort()
		return
	}
	c.JSON(200, gin.H{
		"message": "success",
		"error":   "",
	})
}

func action_show(c *gin.Context) {
	var err error
	id := c.PostForm("id")
	name := c.PostForm("name")
	code := c.PostForm("code")
	output := c.PostForm("output")
	page_size := c.PostForm("page_size")
	page_num := c.PostForm("page")
	offset := 1
	if page_num != "" {
		offset, err = strconv.Atoi(page_num)
		if err != nil {
			c.JSON(400, gin.H{
				"message": "fail",
				"error":   err.Error(),
				"total":   0,
				"result":  []interface{}{},
			})
			return
		}
		if offset <= 0 {
			offset = 1
		}
	}
	multi := 20
	if page_size != "" {
		multi, err = strconv.Atoi(page_size)
		if err != nil {
			c.JSON(400, gin.H{
				"message": "fail",
				"error":   err.Error(),
				"total":   0,
				"result":  []interface{}{},
			})
			return
		}
		if multi <= 0 {
			multi = 1
		}
	}

	// 构建查询条件
	var filter_argv []interface{}
	var filter_sql string

	if name != "" {
		filter_sql = filter_sql + " AND name LIKE ?"
		filter_argv = append(filter_argv, "%"+name+"%")
	}
	if code != "" {
		filter_sql = filter_sql + " AND code LIKE ?"
		filter_argv = append(filter_argv, "%"+code+"%")
	}
	if output != "" {
		filter_sql = filter_sql + " AND output LIKE ?"
		filter_argv = append(filter_argv, "%"+output+"%")
	}

	// 构建基础查询，过滤已删除的记录
	baseQuery := config.Db.Model(&database.Action{}).Where("is_deleted = ?", false)
	if filter_sql != "" {
		// 移除开头的 " AND "
		filter_sql = strings.TrimPrefix(filter_sql, " AND ")
		baseQuery = baseQuery.Where(filter_sql, filter_argv...)
	}

	var count int64
	err = baseQuery.Count(&count).Error
	if err != nil {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   err.Error(),
			"total":   0,
			"result":  []interface{}{},
		})
		return
	}
	page_count := 1
	if count > 0 {
		page_count = int(math.Ceil(float64(count) / float64(multi)))
	}

	// 当没有数据时，直接返回空结果，而不是报错
	if count == 0 {
		c.JSON(200, gin.H{
			"message": "success",
			"error":   "",
			"total":   count,
			"result":  []interface{}{},
		})
		return
	}

	if page_count < offset {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "Page Error",
			"total":   count,
			"result":  []interface{}{},
		})
		return
	}

	var actions []database.Action
	if id == "" {
		query := baseQuery.Order("id DESC").Limit(multi).Offset((offset - 1) * multi)
		err = query.Find(&actions).Error
	} else {
		err = config.Db.Where("id = ?", id).Order("id DESC").Limit(multi).Offset((offset - 1) * multi).Find(&actions).Error
	}
	if err != nil {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   err.Error(),
			"total":   0,
			"result":  []interface{}{},
		})
		return
	}
	var ret []map[string]interface{}
	for _, action := range actions {
		code := action.Code
		output := action.Output
		if len(code) > 10240 {
			code = code[:10240]
		}
		if len(output) > 10240 {
			output = output[:10240]
		}

		element := map[string]interface{}{
			"id":       action.ID,
			"name":     action.Name,
			"code":     code,
			"output":   output,
			"error":    action.Error,
			"config":   action.Config,
			"interval": action.Interval,
			"timeout":  action.Timeout,
			"status":   action.Status,
			"next_run": action.NextRun.Format(time.DateTime),
			"updated":  action.UpdatedAt.Format(time.DateTime),
		}
		ret = append(ret, element)
	}
	c.JSON(200, gin.H{
		"message": "success",
		"error":   "",
		"total":   count,
		"result":  ret,
	})
}

func action_delete(c *gin.Context) {
	action_id := c.PostForm("id")
	if action_id == "" {
		c.JSON(400, gin.H{"message": "fail", "error": "id is required"})
		return
	}

	// 软删除：将is_deleted设置为true
	result := config.Db.Model(&database.Action{}).Where("id = ?", action_id).Update("is_deleted", true)
	if result.Error != nil {
		c.JSON(500, gin.H{"message": "fail", "error": result.Error.Error()})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(404, gin.H{"message": "fail", "error": "action not found"})
		return
	}

	c.JSON(200, gin.H{
		"message": "success",
		"error":   "",
	})
}

func action_execute(c *gin.Context) {
	action_id := c.PostForm("id")
	if action_id == "" {
		c.JSON(400, gin.H{"message": "fail", "error": "id is required"})
		return
	}

	// 查找Action
	var action database.Action
	err := config.Db.Where("id = ? AND is_deleted = ?", action_id, false).First(&action).Error
	if err != nil {
		c.JSON(404, gin.H{"message": "fail", "error": "action not found"})
		return
	}

	// 检查是否有代码
	if action.Code == "" {
		c.JSON(400, gin.H{"message": "fail", "error": "action has no code"})
		return
	}

	// 设置next_run为1999年1月1日，这样会被立即执行
	action.NextRun = time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)

	// 更新数据库
	err = config.Db.Save(&action).Error
	if err != nil {
		c.JSON(500, gin.H{"message": "fail", "error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"message": "success",
		"error":   "",
	})
}

// action_get_by_id 根据ID获取Action详情
func action_get_by_id(c *gin.Context) {
	action_id := c.PostForm("id")
	if action_id == "" {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "ID参数不能为空",
		})
		return
	}

	var action database.Action
	err := config.Db.Where("id = ? AND is_deleted = ?", action_id, false).First(&action).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(404, gin.H{
				"message": "fail",
				"error":   "定时计划不存在",
			})
		} else {
			c.JSON(500, gin.H{
				"message": "fail",
				"error":   "查询失败: " + err.Error(),
			})
		}
		return
	}

	// 格式化next_run时间
	var nextRunStr string
	if !action.NextRun.IsZero() {
		nextRunStr = action.NextRun.Format("2006-01-02 15:04:05")
	}

	element := map[string]interface{}{
		"id":         action.ID,
		"name":       action.Name,
		"code":       action.Code,
		"output":     action.Output,
		"error":      action.Error,
		"config":     action.Config,
		"interval":   action.Interval,
		"timeout":    action.Timeout,
		"status":     action.Status,
		"next_run":   nextRunStr,
		"created_at": action.CreatedAt.Format("2006-01-02 15:04:05"),
		"updated_at": action.UpdatedAt.Format("2006-01-02 15:04:05"),
	}

	c.JSON(200, gin.H{
		"message": "success",
		"result":  element,
	})
}
