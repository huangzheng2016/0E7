package server

import (
	"0E7/utils/config"
	"0E7/utils/database"
	"bytes"
	"encoding/base64"
	"log"
	"os/exec"
	"regexp"
	"time"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
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
				switch fileType {
				case "code/python2", "code/python3":
					cmd := exec.Command("python", "-c", string(code))
					var stdout, stdeer bytes.Buffer
					cmd.Stderr = &stdeer
					cmd.Stdout = &stdout

					err = cmd.Start()
					if err != nil {
						log.Println(err)
						return
					}
					err := cmd.Wait()
					if err != nil {
						log.Println(err)
						log.Println("Runtime error:", stdeer.String())
					}
					new_output = stdout.String()
				case "code/golang":
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
				default:
					log.Println("Unknown file type:", fileType)
					return
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
