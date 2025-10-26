package client

import (
	"0E7/service/config"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/pcapgo"
)

func moniter_pcap_device() string {
	devices, err := pcap.FindAllDevs()
	if err != nil {
		return "{}"
	}
	type item struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	result := []item{}
	for _, device := range devices {
		result = append(result, item{Name: device.Name, Description: device.Description})
	}
	device, err := json.Marshal(result)
	if err != nil {
		return "{}"
	}
	return string(device)
}

func moniter_pcap(device string, desc string, bpf string, timeout time.Duration) {
	var wg sync.WaitGroup
	if device != "" {
		log.Printf("开始采集设备 %s (%s) 的流量，BPF过滤器: %s，采集时长: %v", device, desc, bpf, timeout)
		wg.Add(1)
		go capture(device, desc, bpf, timeout, &wg)
	} else {
		log.Printf("开始采集所有设备的流量，BPF过滤器: %s，采集时长: %v", bpf, timeout)
		devices, err := pcap.FindAllDevs()
		if err != nil {
			log.Printf("查找网络设备失败: %v", err)
			return
		}
		log.Printf("找到 %d 个网络设备", len(devices))
		for _, device := range devices {
			wg.Add(1)
			go capture(device.Name, device.Description, bpf, timeout, &wg)
		}
	}
	wg.Wait()
	log.Printf("所有设备流量采集完成")
}

func capture(device string, desc string, bpf string, timeout time.Duration, wg *sync.WaitGroup) (err error) {
	defer wg.Done()

	log.Printf("设备 %s 开始初始化流量采集", device)
	handle, err := pcap.OpenLive(device, 65536, true, timeout)
	if err != nil {
		log.Printf("设备 %s 打开失败: %v", device, err)
		return err
	}
	defer handle.Close()
	log.Printf("设备 %s 初始化成功", device)

	buffer := new(bytes.Buffer)
	writer_pcap := pcapgo.NewWriter(buffer)
	if err != nil {
		log.Printf("设备 %s 创建PCAP写入器失败: %v", device, err)
		return err
	}

	err = writer_pcap.WriteFileHeader(65536, handle.LinkType())
	if err != nil {
		log.Printf("设备 %s 写入PCAP文件头失败: %v", device, err)
		return err
	}

	if bpf != "" {
		err = handle.SetBPFFilter(bpf)
		if err != nil {
			log.Printf("设备 %s 设置BPF过滤器 '%s' 失败: %v", device, bpf, err)
			return err
		}
		log.Printf("设备 %s BPF过滤器设置成功: %s", device, bpf)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	packetCount := 0
	startTime := time.Now()

	log.Printf("设备 %s 开始采集数据包，采集时长: %v", device, timeout)

	for {
		select {
		case packet := <-packetSource.Packets():
			packetCount++
			err = writer_pcap.WritePacket(packet.Metadata().CaptureInfo, packet.Data())
			if err != nil {
				log.Printf("设备 %s 写入数据包失败: %v", device, err)
				break
			}
		case <-ctx.Done():
			endTime := time.Now()
			duration := endTime.Sub(startTime)

			// 修复文件名生成
			fileName := fmt.Sprintf("%d_%s_%d.pcap",
				config.Client_id,
				device,
				startTime.Unix())

			log.Printf("设备 %s 采集完成，采集时长: %v，数据包数量: %d，文件名: %s",
				device, duration, packetCount, fileName)

			body := &bytes.Buffer{}
			writer_file := multipart.NewWriter(body)
			fileWriter, err := writer_file.CreateFormFile("file", fileName)
			if err != nil {
				log.Printf("设备 %s 创建文件写入器失败: %v", device, err)
				return err
			}

			_, err = buffer.WriteTo(fileWriter)
			if err != nil {
				log.Printf("设备 %s 写入文件数据失败: %v", device, err)
				return err
			}

			err = writer_file.Close()
			if err != nil {
				log.Printf("设备 %s 关闭文件写入器失败: %v", device, err)
				return err
			}

			log.Printf("设备 %s 开始上传文件 %s，文件大小: %d 字节", device, fileName, buffer.Len())

			request, err := http.NewRequest("POST", config.Server_url+"/webui/pcap_upload", body)
			if err != nil {
				log.Printf("设备 %s 创建上传请求失败: %v", device, err)
				return err
			}
			request.Header.Set("Content-Type", writer_file.FormDataContentType())

			response, err := client.Do(request)
			if err != nil {
				log.Printf("设备 %s 上传文件失败: %v", device, err)
				return err
			}
			defer response.Body.Close()

			if response.StatusCode != 200 {
				log.Printf("设备 %s 上传文件失败，状态码: %d", device, response.StatusCode)
				return errors.New("upload failed")
			}

			log.Printf("设备 %s 文件上传成功", device)
			return nil
		}
	}
}
