package webui

import (
	"0E7/utils/config"
	"bytes"
	"database/sql"
	"encoding/base64"
	"github.com/gin-gonic/gin"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
	"log"
	"math"
	"regexp"
	"strconv"
	"time"
)

func action(c *gin.Context) {
	var err error
	id := c.PostForm("id")
	name := c.PostForm("name")
	code := c.PostForm("code")
	output := c.PostForm("output")
	interval := c.PostForm("interval")
	updated := time.Now().Format(time.DateTime)
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
	if id == "" {
		_, err = config.Db.Exec("INSERT INTO `0e7_action` (name,code,output,interval,updated) VALUES (?,?,?,?,?)", name, code, output, interval, updated)
	} else {
		_, err = config.Db.Exec("UPDATE `0e7_action` SET name=?,code=?,output=?,interval=?,updated=? WHERE uuid=?", name, code, output, interval, updated, id)
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
	page_show := c.PostForm("page_show")
	page_num := c.PostForm("page_num")
	offset := 1
	if page_num != "" {
		offset, err = strconv.Atoi(page_num)
		if err != nil {
			c.JSON(400, gin.H{
				"message":    "fail",
				"error":      err.Error(),
				"page_num":   "",
				"page_show":  "",
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
	if page_show != "" {
		multi, err = strconv.Atoi(page_show)
		if err != nil {
			c.JSON(400, gin.H{
				"message":    "fail",
				"error":      err.Error(),
				"page_num":   "",
				"page_show":  "",
				"page_count": "",
				"result":     []interface{}{},
			})
			return
		}
		if multi <= 0 {
			multi = 1
		}
	}
	var count int
	if name == "" {
		err = config.Db.QueryRow("SELECT COUNT(*) FROM `0e7_action` WHERE 1").Scan(&count)
	} else {
		err = config.Db.QueryRow("SELECT COUNT(*) FROM `0e7_action` WHERE name LIKE ?", "%"+name+"%").Scan(&count)
	}
	if err != nil {
		c.JSON(400, gin.H{
			"message":    "fail",
			"error":      err.Error(),
			"page_num":   "",
			"page_show":  "",
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
				"page_show":  multi,
				"page_count": page_count,
				"result":     []interface{}{},
			})
			return
		}
	}

	var rows *sql.Rows
	if id == "" {
		if name == "" {
			rows, err = config.Db.Query("SELECT name,substr(code,10240),substr(output,10240),output,interval,updated FROM `0e7_action` WHERE 1 ORDER BY id DESC LIMIT ? OFFSET ?", multi, (offset-1)*multi)
		} else {
			rows, err = config.Db.Query("SELECT name,substr(code,10240),substr(output,10240),output,interval,updated FROM `0e7_action` WHERE name LIKE ? ORDER BY id DESC LIMIT ? OFFSET ?", "%"+name+"%", multi, (offset-1)*multi)
		}
	} else {
		rows, err = config.Db.Query("SELECT name,code,output,interval,updated FROM `0e7_action` WHERE id=? ORDER BY id DESC LIMIT ? OFFSET ?", id, multi, (offset-1)*multi)
	}
	if err != nil {
		c.JSON(400, gin.H{
			"message":    "fail",
			"error":      err.Error(),
			"page_num":   "",
			"page_show":  "",
			"page_count": "",
			"result":     []interface{}{},
		})
		return
	}
	var ret []map[string]interface{}
	for rows.Next() {
		var name, code, output, interval, updated string
		err := rows.Scan(&name, &code, &output, &interval, &updated)
		if err != nil {
			c.JSON(400, gin.H{
				"message":    "fail",
				"error":      err.Error(),
				"page_num":   "",
				"page_show":  "",
				"page_count": "",
				"result":     []interface{}{},
			})
			return
		}
		element := map[string]interface{}{
			"id":       id,
			"name":     name,
			"code":     code,
			"output":   output,
			"interval": interval,
			"updated":  updated,
		}
		ret = append(ret, element)
	}
	c.JSON(200, gin.H{
		"message":    "success",
		"error":      "",
		"page_num":   "",
		"page_show":  "",
		"page_count": "",
		"result":     ret,
	})
}
func update_action() {
	var err error
	var id, interval int
	var name, code, output, updated string
	err = config.Db.QueryRow("SELECT id,name,code,output,interval,updated FROM `0e7_action` WHERE interval>=0 AND code!='' ORDER BY updated DESC LIMIT 1").Scan(&id, &name, &code, &output, &interval, &updated)
	if err == nil {
		updatedTime, err := time.ParseInLocation(time.DateTime, updated, time.Now().Location())
		if err != nil {
			log.Println(err)
			return
		}
		if updatedTime.Before(time.Now()) {
			match := regexp.MustCompile(`^data:(code\/(?:python2|python3|golang));base64,(.*)$`).FindStringSubmatch(code)
			if match != nil {
				fileType := match[1]
				data := match[2]
				code_decode, err := base64.StdEncoding.DecodeString(data)
				if err != nil {
					log.Println("Base64 decode error:", err)
					return
				}
				code = string(code_decode)
				var new_output string
				if fileType == "code/python2" || fileType == "code/python3" {
					log.Println("Python support in future")
					return
				} else if fileType == "code/golang" {
					var goibuf bytes.Buffer
					goi := interp.New(interp.Options{Stdout: &goibuf})
					goi.Use(stdlib.Symbols)
					programs[id], err = goi.Compile(code)
					if err != nil {
						log.Println("Compile error:", err.Error())
						return
					}
					_, err = goi.Execute(programs[id])
					if err != nil {
						log.Println("Runtime error:", err.Error())
						return
					}
					new_output = goibuf.String()
				}
				if new_output != output {
					_, err = config.Db.Exec("UPDATE `0e7_action` SET output=?,updated=? WHERE id=?", new_output, time.Now().Add(time.Duration(interval)*time.Second).Format(time.DateTime), id)
					if err != nil {
						log.Println("Update output error:", err.Error())
						return
					}
					log.Printf("Action %s update: %s\n", name, new_output)
				}
				programs[id] = nil
			} else {
				log.Println("Code format error")
				return
			}
		}
	}
}
