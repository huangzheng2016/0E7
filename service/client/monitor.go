package client

import (
	"0E7/service/config"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func monitor() {
	defer func() {
		if err := recover(); err != nil {
			log.Println("Monitor error: ", err)
		}
	}()
	if !config.Client_monitor {
		return
	}
	values := url.Values{}
	values.Set("client_id", fmt.Sprintf("%d", config.Client_id))
	requestBody := bytes.NewBufferString(values.Encode())
	request, err := http.NewRequest("POST", config.Server_url+"/api/monitor", requestBody)
	if err != nil {
		log.Println(err)
		return
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.Do(request)
	if err != nil {
		log.Println(err)
		return
	}
	defer response.Body.Close() // 确保关闭响应体
	if response.StatusCode == 200 {
		var result map[string]interface{}
		err = json.NewDecoder(response.Body).Decode(&result)
		if err != nil {
			log.Println(err)
		}
		id_list := []int{}
		for _, item := range result["result"].([]interface{}) {
			itemMap := item.(map[string]interface{})
			id := int(itemMap["id"].(float64))
			id_list = append(id_list, id)
			types := itemMap["types"].(string)
			data := itemMap["data"].(string)

			// 处理interval字段，可能是string或float64
			var interval int
			switch v := itemMap["interval"].(type) {
			case string:
				interval, err = strconv.Atoi(v)
				if err != nil {
					log.Printf("解析interval字符串失败: %v，使用默认值60", err)
					interval = 60
				}
			case float64:
				interval = int(v)
			case int:
				interval = v
			default:
				log.Printf("未知的interval类型: %T，使用默认值60", v)
				interval = 60
			}

			new := Tmonitor{types: types, data: data, interval: interval}
			if oldValue, exists := monitor_list.Load(id); !exists || oldValue.(Tmonitor) != new {
				monitor_list.Store(id, new)
				go monitor_run(id)
			}
		}
		monitor_list.Range(func(key, value interface{}) bool {
			id := key.(int)
			found := false
			for _, listId := range id_list {
				if id == listId {
					found = true
					break
				}
			}
			if !found {
				monitor_list.Delete(id)
			}
			return true
		})
	}
}

func monitor_run(id int) {
	value, exists := monitor_list.Load(id)
	if !exists {
		return
	}
	old := value.(Tmonitor)
	if old.types == "pcap" {
		type item struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Bpf         string `json:"bpf"`
		}
		var device item
		err := json.Unmarshal([]byte(old.data), &device)
		if err != nil {
			log.Printf("监控任务ID %d 解析设备配置失败: %v", id, err)
			return
		}

		log.Printf("监控任务ID %d 开始执行，设备: %s (%s), BPF过滤器: %s, 采集间隔: %d秒",
			id, device.Name, device.Description, device.Bpf, old.interval)

		for {
			currentValue, exists := monitor_list.Load(id)
			if !exists || currentValue.(Tmonitor) != old || old.interval == 0 {
				log.Printf("监控任务ID %d 已停止或配置已更改", id)
				break
			}

			startTime := time.Now()
			log.Printf("监控任务ID %d 开始采集流量，开始时间: %s", id, startTime.Format("2006-01-02 15:04:05"))

			moniter_pcap(device.Name, device.Description, device.Bpf, time.Duration(old.interval)*time.Second)

			endTime := time.Now()
			duration := endTime.Sub(startTime)
			log.Printf("监控任务ID %d 采集完成，结束时间: %s，采集耗时: %v",
				id, endTime.Format("2006-01-02 15:04:05"), duration)

			// 如果采集时间小于间隔时间，则等待剩余时间
			if duration < time.Duration(old.interval)*time.Second {
				sleepTime := time.Duration(old.interval)*time.Second - duration
				log.Printf("监控任务ID %d 等待 %v 后进行下次采集", id, sleepTime)
				time.Sleep(sleepTime)
			}
		}
	}
}
