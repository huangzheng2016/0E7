package client

import (
	"0E7/service/config"
	"bytes"
	"encoding/json"
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
	values.Set("uuid", config.Client_uuid)
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
	}
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
			interval, err := strconv.Atoi(itemMap["interval"].(string))
			if err != nil {
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
			log.Println(err)
			return
		}
		for {
			currentValue, exists := monitor_list.Load(id)
			if !exists || currentValue.(Tmonitor) != old || old.interval == 0 {
				break
			}
			now := time.Now()
			moniter_pcap(device.Name, device.Description, device.Bpf, time.Duration(old.interval)*time.Second)
			if time.Since(now) < time.Duration(old.interval)*time.Second {
				time.Sleep(time.Duration(old.interval)*time.Second - time.Since(now))
			}
		}
	}
}
