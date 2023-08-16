package update

import (
	"0E7/utils/config"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"time"
)

func downloadFile(filepath string) error {
	values := url.Values{}
	values.Set("platform", runtime.GOOS)
	values.Set("arch", runtime.GOARCH)
	requestBody := bytes.NewBufferString(values.Encode())
	request, err := http.NewRequest("POST", config.Server_url+"/api/update", requestBody)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: time.Duration(config.Global_timeout_download) * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	out, err := os.Create("new_" + filepath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, response.Body)
	if err != nil {
		return err
	}
	return nil
}

func Replace() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("Replace Error:", err)
		}
	}()

	var filePath string
	filePath = "0e7_" + runtime.GOOS + "_" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		filePath += ".exe"
	}
	err := downloadFile(filePath)
	if err != nil {
		fmt.Println("File download error", err)
		return
	}
	wdPath, err := os.Getwd()
	if err != nil {
		fmt.Println("exePath read fail", err)
		return
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", "/C", "start", "new_"+filePath)
	} else {
		cmd = exec.Command("nohup", "./"+"new_"+filePath, "&")
	}
	//fmt.Println(wdPath)
	cmd.Dir = wdPath
	err = cmd.Start()
	if err != nil {
		fmt.Println("Replace fail", err)
		return
	}
	os.Exit(0)
}
