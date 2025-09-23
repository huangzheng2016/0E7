package client

import (
	"0E7/utils/config"
	"bytes"
	"crypto/tls"
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
			log.Println("Exploit error: ", err)
		}
		jobsMutex.Lock()
		currentJobs--
		jobsMutex.Unlock()
	}()
	values := url.Values{}
	values.Set("uuid", config.Client_uuid)
	requestBody := bytes.NewBufferString(values.Encode())
	request, err := http.NewRequest("POST", config.Server_url+"/api/monitor", requestBody)
	if err != nil {
		log.Println(err)
		return
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: time.Duration(config.Global_timeout_http) * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
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
			if monitor_list[id] != new {
				monitor_list[id] = new
				go monitor_run(id)
			}
		}
		for key := range monitor_list {
			found := false
			for _, id := range id_list {
				if key == id {
					found = true
					break
				}
			}
			if !found {
				delete(monitor_list, key)
			}
		}
	}
}
func monitor_run(id int) {
	old := monitor_list[id]
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
			if old != monitor_list[id] || old.interval == 0 {
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
