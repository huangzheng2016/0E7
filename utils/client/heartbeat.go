package client

import (
	"0E7/utils/config"
	"0E7/utils/update"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"log"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"time"
)

var heartbeat_delay int

func heartbeat() {
	defer func() {
		if err := recover(); err != nil {
			log.Println("Heartbeat Error:", err)
			go heartbeat()
		}
	}()
	for {
		cpuInfo, err := cpu.Info()
		if err != nil {
			log.Println("Failed to get cpuInfo:", err)
		}
		memInfo, err := mem.VirtualMemory()
		if err != nil {
			log.Println("Failed to get memInfo:", err)
		}
		cpuPercent, err := cpu.Percent(time.Second, false)
		if err != nil {
			log.Println("Failed to get cpuPercent:", err)
		}
		hostname, err := host.Info()
		if err != nil {
			log.Println("Failed to get hostname:", err)
			return
		}
		pcap := moniter_pcap_device()
		values := url.Values{}
		values.Set("uuid", config.Client_uuid)
		values.Set("hostname", hostname.Hostname)
		values.Set("platform", runtime.GOOS)
		values.Set("arch", runtime.GOARCH)
		values.Set("cpu", cpuInfo[0].ModelName)
		values.Set("cpu_use", strconv.FormatFloat(cpuPercent[0], 'f', 2, 64))
		values.Set("memory_use", strconv.Itoa(int(memInfo.Used/1024/1024)))
		values.Set("memory_max", strconv.Itoa(int(memInfo.Total/1024/1024)))
		values.Set("pcap", pcap)

		requestBody := bytes.NewBufferString(values.Encode())
		request, err := http.NewRequest("POST", config.Server_url+"/api/heartbeat", requestBody)
		if err != nil {
			log.Println(err)
		}
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		client := &http.Client{Timeout: time.Duration(config.Global_timeout_http) * time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
		response, err := client.Do(request)
		if err != nil {
			log.Println(err)
		}
		if response.StatusCode == 200 || response.StatusCode == 400 {
			if response.StatusCode == 400 {
				log.Println("Try to update manually")
			}
			var result map[string]interface{}
			err = json.NewDecoder(response.Body).Decode(&result)
			if err != nil {
				log.Println(err)
			}
			found := false
			for _, hash := range result["sha256"].([]interface{}) {
				if hash == update.Sha256Hash[0] {
					found = true
					break
				}
			}
			if found == false && config.Client_update == true {
				log.Println("Try to update")
				go update.Replace()
			} else {
				jobsMutex.Lock()
				if currentJobs <= maxWorkers {
					currentJobs++
					go exploit()
				}
				jobsMutex.Unlock()
			}
			go monitor()
		}
		time.Sleep(time.Second * time.Duration(heartbeat_delay))
	}
}
