package webui

import (
	"0E7/service/config"
	"0E7/service/database"
	"math"
	"regexp"
	"strconv"
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
	page_size := c.PostForm("page_size")
	page_num := c.PostForm("page")
	offset := 1
	if page_num != "" {
		offset, err = strconv.Atoi(page_num)
		if err != nil {
			c.JSON(400, gin.H{
				"message":    "fail",
				"error":      err.Error(),
				"page_num":   "",
				"page":       "",
				"page_count": "",
				"result":     []interface{}{},
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
				"message":    "fail",
				"error":      err.Error(),
				"page_num":   "",
				"page":       "",
				"page_count": "",
				"result":     []interface{}{},
			})
			return
		}
		if multi <= 0 {
			multi = 1
		}
	}
	var count int64
	if name == "" {
		err = config.Db.Model(&database.Action{}).Count(&count).Error
	} else {
		err = config.Db.Model(&database.Action{}).Where("name LIKE ?", "%"+name+"%").Count(&count).Error
	}
	if err != nil {
		c.JSON(400, gin.H{
			"message":    "fail",
			"error":      err.Error(),
			"page_num":   "",
			"page":       "",
			"page_count": "",
			"result":     []interface{}{},
		})
		return
	}
	page_count := 1
	if count >= 0 {
		page_count = int(math.Ceil(float64(count) / float64(multi)))
	}
	if page_count < offset {
		if err != nil {
			c.JSON(400, gin.H{
				"message":    "fail",
				"error":      "Page Error",
				"page_num":   "",
				"page":       multi,
				"page_count": page_count,
				"result":     []interface{}{},
			})
			return
		}
	}

	var actions []database.Action
	if id == "" {
		if name == "" {
			err = config.Db.Order("id DESC").Limit(multi).Offset((offset - 1) * multi).Find(&actions).Error
		} else {
			err = config.Db.Where("name LIKE ?", "%"+name+"%").Order("id DESC").Limit(multi).Offset((offset - 1) * multi).Find(&actions).Error
		}
	} else {
		err = config.Db.Where("id = ?", id).Order("id DESC").Limit(multi).Offset((offset - 1) * multi).Find(&actions).Error
	}
	if err != nil {
		c.JSON(400, gin.H{
			"message":    "fail",
			"error":      err.Error(),
			"page_num":   "",
			"page":       "",
			"page_count": "",
			"result":     []interface{}{},
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
		"message":    "success",
		"error":      "",
		"page_num":   "",
		"page":       "",
		"page_count": "",
		"result":     ret,
	})
}
