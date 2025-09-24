package webui

import (
	"0E7/utils/config"
	"0E7/utils/database"
	"bytes"
	"encoding/base64"
	"log"
	"math"
	"regexp"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
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
func update_action() {
	var err error
	var actionRecord database.Action
	err = config.Db.Where("interval >= 0 AND code != ''").Order("updated_at").First(&actionRecord).Error
	if err == nil {
		if actionRecord.UpdatedAt.Before(time.Now()) {
			match := regexp.MustCompile(`^data:(code\/(?:python2|python3|golang));base64,(.*)$`).FindStringSubmatch(actionRecord.Code)
			if match != nil {
				fileType := match[1]
				data := match[2]
				code_decode, err := base64.StdEncoding.DecodeString(data)
				if err != nil {
					log.Println("Base64 decode error:", err)
					return
				}
				code := string(code_decode)
				var new_output string
				if fileType == "code/python2" || fileType == "code/python3" {
					log.Println("Python support in future")
					return
				} else if fileType == "code/golang" {
					var goibuf bytes.Buffer
					goi := interp.New(interp.Options{Stdout: &goibuf})
					goi.Use(stdlib.Symbols)
					programs[int(actionRecord.ID)], err = goi.Compile(code)
					if err != nil {
						log.Println("Compile error:", err.Error())
						return
					}
					_, err = goi.Execute(programs[int(actionRecord.ID)])
					if err != nil {
						log.Println("Runtime error:", err.Error())
						return
					}
					new_output = goibuf.String()
				}
				if new_output != actionRecord.Output {
					actionRecord.Output = new_output
					actionRecord.UpdatedAt = time.Now().Add(time.Duration(actionRecord.Interval) * time.Second)
					err = config.Db.Save(&actionRecord).Error
					if err != nil {
						log.Println("Update output error:", err.Error())
						return
					}
					log.Printf("Action %s update: %s\n", actionRecord.Name, new_output)
				}
				programs[int(actionRecord.ID)] = nil
			} else {
				log.Println("Code format error")
				return
			}
		}
	}
}
