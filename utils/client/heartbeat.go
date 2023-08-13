package client

import (
	"0E7/utils/config"
	"0E7/utils/update"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

var heartbeat_delay int

func heartbeat() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("Heartbeat Error:", err)
			go heartbeat()
		}
	}()
	for true {
		cpuInfo, err := cpu.Info()
		if err != nil {
			fmt.Println("Failed to get cpuInfo:", err)
		}
		memInfo, err := mem.VirtualMemory()
		if err != nil {
			fmt.Println("Failed to get memInfo:", err)
		}
		cpuPercent, err := cpu.Percent(time.Second, false)
		if err != nil {
			fmt.Println("Failed to get cpuPercent:", err)
		}
		hostname, err := host.Info()
		if err != nil {
			fmt.Println("Failed to get hostname:", err)
			return
		}
		values := url.Values{}
		values.Set("uuid", config.Client_uuid)
		values.Set("hostname", hostname.Hostname)
		values.Set("cpu", cpuInfo[0].ModelName)
		values.Set("cpu_use", strconv.FormatFloat(cpuPercent[0], 'f', 2, 64))
		values.Set("memory_use", strconv.Itoa(int(memInfo.Used/1024/1024)))
		values.Set("memory_max", strconv.Itoa(int(memInfo.Total/1024/1024)))
		requestBody := bytes.NewBufferString(values.Encode())
		request, err := http.NewRequest("POST", config.Server_url+"/api/heartbeat", requestBody)
		if err != nil {
			fmt.Println(err)
		}
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		client := &http.Client{Timeout: time.Duration(config.Global_timeout_http) * time.Second}
		response, err := client.Do(request)
		if err != nil {
			fmt.Println(err)
		}
		if response.StatusCode == 200 {
			var result map[string]interface{}
			err = json.NewDecoder(response.Body).Decode(&result)
			if err != nil {
				fmt.Println(err)
			}
			found := false
			for _, hash := range result["sha256"].([]interface{}) {
				if hash == update.Sha256_hash[0] {
					found = true
					break
				}
			}
			if found == false && config.Client_update == true {
				fmt.Println("Try to update")
				go update.Replace()
			} else {
				exploit()
			}
		}
		time.Sleep(time.Second * time.Duration(heartbeat_delay))
	}
}
