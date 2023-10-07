package client

import (
	"0E7/utils/config"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/pcapgo"
	"log"
	"mime/multipart"
	"net/http"
	"strconv"
	"sync"
	"time"
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
		wg.Add(1)
		go capture(device, desc, bpf, timeout, &wg)
	} else {
		devices, err := pcap.FindAllDevs()
		if err != nil {
			log.Println("Error finding devices:", err)
			return
		}
		for _, device := range devices {
			wg.Add(1)
			go capture(device.Name, device.Description, bpf, timeout, &wg)
		}
	}
	wg.Wait()
}
func capture(device string, desc string, bpf string, timeout time.Duration, wg *sync.WaitGroup) (err error) {
	defer wg.Done()
	handle, err := pcap.OpenLive(device, 65536, true, timeout)
	if err != nil {
		log.Println(err.Error())
		return err
	}
	defer handle.Close()

	buffer := new(bytes.Buffer)

	writer_pcap := pcapgo.NewWriter(buffer)
	if err != nil {
		return err
	}
	err = writer_pcap.WriteFileHeader(65536, handle.LinkType())
	if err != nil {
		return err
	}
	err = handle.SetBPFFilter(bpf)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for {
		select {
		case packet := <-packetSource.Packets():
			err = writer_pcap.WritePacket(packet.Metadata().CaptureInfo, packet.Data())
			if err != nil {
				log.Println(err.Error())
				break
			}
		case <-ctx.Done():
			body := &bytes.Buffer{}
			writer_file := multipart.NewWriter(body)
			fileWriter, err := writer_file.CreateFormFile("file", config.Client_uuid+"_"+desc+"_"+strconv.Itoa(int(time.Now().Unix()))+".pcap")
			if err != nil {
				log.Println(err)
				return err
			}
			_, err = buffer.WriteTo(fileWriter)
			if err != nil {
				log.Println(err)
				return err
			}
			err = writer_file.Close()
			if err != nil {
				log.Println(err)
				return err
			}
			request, err := http.NewRequest("POST", config.Server_url+"/webui/pcap_upload", body)
			if err != nil {
				log.Println(err)
				return err
			}
			request.Header.Set("Content-Type", writer_file.FormDataContentType())
			client := &http.Client{Timeout: time.Duration(config.Global_timeout_http) * time.Second,
				Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
			response, err := client.Do(request)
			if err != nil {
				log.Println(err)
				return err
			}
			defer response.Body.Close()

			if response.StatusCode != 200 {
				log.Println("Upload failed")
				return errors.New("Upload failed")
			}
			return nil
		}
	}
}
