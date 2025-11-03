package update

import (
	"0E7/service/config"
	"bytes"
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

var (
	client = &http.Client{Timeout: time.Duration(config.Global_timeout_http) * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}

	replaceMutex    sync.Mutex
	lastFailureTime time.Time
	failureCount    int
	maxBackoffDelay = time.Hour
	initialBackoff  = time.Minute
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

// calculateBackoffDelay 计算指数退避延时
func calculateBackoffDelay(count int) time.Duration {
	delay := initialBackoff
	for i := 0; i < count; i++ {
		delay *= 2
		if delay > maxBackoffDelay {
			delay = maxBackoffDelay
			break
		}
	}
	return delay
}

func Replace() {
	// 尝试获取锁，如果获取不到（已经有其他 Replace() 在运行）则直接退出
	if !replaceMutex.TryLock() {
		return
	}
	defer replaceMutex.Unlock()

	// 在执行前判断时间，如果还没到重试时间就直接返回
	if !lastFailureTime.IsZero() {
		nextRetryTime := lastFailureTime.Add(calculateBackoffDelay(failureCount))
		if time.Now().Before(nextRetryTime) {
			return
		}
	}

	defer func() {
		if err := recover(); err != nil {
			log.Println("Replace Error:", err)
		}
	}()

	var filePath string
	filePath = "0e7_" + runtime.GOOS + "_" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		filePath += ".exe"
	}
	err := downloadFile(filePath)
	if err != nil {
		log.Println("File download error", err)
		// 下载失败，记录失败时间和失败次数
		lastFailureTime = time.Now()
		failureCount++
		return
	}

	// 下载成功，重置失败计数
	lastFailureTime = time.Time{}
	failureCount = 0

	wdPath, err := os.Getwd()
	if err != nil {
		log.Println("exePath read fail", err)
		return
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", "/C", "start", "new_"+filePath)
	} else {
		cmd = exec.Command("nohup", "./"+"new_"+filePath, "&")
	}
	//log.Println(wdPath)
	cmd.Dir = wdPath
	err = cmd.Start()
	if err != nil {
		log.Println("Replace fail", err)
		return
	}
	os.Exit(0)
}
