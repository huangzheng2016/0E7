package server

import (
	"0E7/utils/config"
	"bytes"
	"encoding/base64"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
	"log"
	"regexp"
	"time"
)

func action() {
	defer func() {
		if err := recover(); err != nil {
			log.Println("Action error: ", err)
		}
		jobsMutex.Lock()
		jobs["action"] = false
		jobsMutex.Unlock()
	}()
	var err error
	var id, interval int
	var name, code, output, updated string
	err = config.Db.QueryRow("SELECT id,name,code,output,interval,updated FROM `0e7_action` WHERE interval>=0 AND code!='' ORDER BY updated LIMIT 1").Scan(&id, &name, &code, &output, &interval, &updated)
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
