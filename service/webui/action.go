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
)

func action(c *gin.Context) {
	var err error
	id := c.PostForm("id")
	name := c.PostForm("name")
	code := c.PostForm("code")
	output := c.PostForm("output")
	interval := c.PostForm("interval")

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
	actionRecord := database.Action{
		Name:     name,
		Code:     code,
		Output:   output,
		Interval: intervalInt,
	}

	if id == "" {
		err = config.Db.Create(&actionRecord).Error
	} else {
		idInt, _ := strconv.Atoi(id)
		actionRecord.ID = uint(idInt)
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
			"interval": action.Interval,
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
