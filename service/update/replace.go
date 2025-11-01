package update

import (
	"0E7/service/config"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

var client = &http.Client{Timeout: time.Duration(config.Global_timeout_http) * time.Second,
	Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}

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

	// 检查响应状态码
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status code %d", response.StatusCode)
	}

	newFilePath := "new_" + filepath
	out, err := os.Create(newFilePath)
	if err != nil {
		return err
	}
	defer out.Close()

	copied, err := io.Copy(out, response.Body)
	if err != nil {
		os.Remove(newFilePath) // 下载失败时清理不完整的文件
		return err
	}

	// 验证文件大小（至少应该有内容）
	if copied == 0 {
		os.Remove(newFilePath)
		return errors.New("downloaded file is empty")
	}

	// 在非 Windows 系统上设置执行权限
	if runtime.GOOS != "windows" {
		err = os.Chmod(newFilePath, 0755)
		if err != nil {
			log.Printf("Warning: Failed to set executable permission: %v", err)
		}
	}

	log.Printf("Downloaded %s successfully, size: %d bytes", newFilePath, copied)
	return nil
}

func Replace() {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("Replace Error: %v", err)
		}
	}()

	var filePath string
	filePath = "0e7_" + runtime.GOOS + "_" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		filePath += ".exe"
	}

	log.Printf("Starting update process, downloading file: %s", filePath)
	err := downloadFile(filePath)
	if err != nil {
		log.Printf("File download error: %v", err)
		return
	}

	// 获取可执行文件的目录作为工作目录，而不是当前工作目录
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("Failed to get executable path: %v", err)
		// 回退到使用当前工作目录
		execPath, _ = os.Getwd()
	}
	wdPath := filepath.Dir(execPath)

	newFilePath := filepath.Join(wdPath, "new_"+filePath)
	// 等待文件写入完成（下载可能刚完成，文件可能还在写入）
	time.Sleep(100 * time.Millisecond)

	// 验证新文件是否存在且可执行
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		info, err := os.Stat(newFilePath)
		if err != nil {
			if i < maxRetries-1 {
				log.Printf("New file not found (attempt %d/%d): %v, retrying...", i+1, maxRetries, err)
				time.Sleep(200 * time.Millisecond)
				continue
			}
			log.Printf("New file not found after %d attempts: %v", maxRetries, err)
			return
		}

		// 检查文件模式是否有执行权限（仅非Windows系统）
		if runtime.GOOS != "windows" {
			if info.Mode()&0111 == 0 {
				log.Println("Warning: File does not have execute permission, attempting to fix...")
				err = os.Chmod(newFilePath, 0755)
				if err != nil {
					log.Printf("Failed to set execute permission: %v", err)
					return
				}
				// 重新检查权限
				info, _ = os.Stat(newFilePath)
				if info.Mode()&0111 == 0 {
					log.Printf("Failed to set execute permission after retry")
					return
				}
			}
		}

		// 验证文件大小不为0
		if info.Size() == 0 {
			log.Printf("New file is empty, download may have failed")
			return
		}
		break
	}

	// 使用绝对路径确保能找到文件
	absNewFilePath, err := filepath.Abs(newFilePath)
	if err != nil {
		log.Printf("Failed to get absolute path: %v", err)
		return
	}

	// 验证文件确实存在
	if _, err := os.Stat(absNewFilePath); err != nil {
		log.Printf("New file does not exist at %s: %v", absNewFilePath, err)
		return
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Windows: 使用绝对路径启动，cmd.Dir已设置工作目录
		// start /b 在后台启动程序，不等待其退出
		cmd = exec.Command("cmd.exe", "/C", "start", "/b", absNewFilePath)
	} else {
		// Unix-like: 直接执行程序并设置后台运行
		cmd = exec.Command(absNewFilePath)
		// 设置进程组，使新进程独立于当前进程组
		setUnixProcAttr(cmd)
	}

	cmd.Dir = wdPath
	// 分离标准输入输出，避免继承
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	log.Printf("Starting new process: %s (working dir: %s)", absNewFilePath, wdPath)
	err = cmd.Start()
	if err != nil {
		log.Printf("Failed to start new process: %v", err)
		return
	}

	// 延长等待时间并改进启动验证
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	// 使用更长的等待时间（2秒），给程序足够的初始化时间
	select {
	case <-time.After(2 * time.Second):
		// 进程仍在运行，说明启动成功
		log.Println("New process started successfully")
		cmd.Process.Release() // 释放资源，不再等待进程退出
	case err := <-done:
		if err != nil {
			log.Printf("New process exited immediately with error: %v", err)
			// 检查进程是否真的启动失败
			// 在某些情况下，程序可能快速启动并退出，这也是正常的
			log.Println("Warning: Process exited quickly, but update may still succeed")
			// 不直接返回，继续执行退出流程
		} else {
			log.Println("New process exited normally")
		}
	}

	// 给新进程一些时间来处理更新（复制文件等操作）
	// 这样新进程在尝试替换旧文件时，旧进程可能已经退出了
	log.Println("Waiting a moment for new process to process the update...")
	time.Sleep(1 * time.Second)

	log.Println("Exiting current process to complete update...")
	os.Exit(0)
}
