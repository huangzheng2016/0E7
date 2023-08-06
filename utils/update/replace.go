package update

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
)

func downloadFile(filepath string) error {
	values := url.Values{}
	values.Set("platform", runtime.GOOS)
	requestBody := bytes.NewBufferString(values.Encode())
	request, err := http.NewRequest("POST", conf.Server_url+"/api/update", requestBody)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	out, err := os.Create(filepath + ".new")
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
	var filePath string
	if runtime.GOOS == "windows" {
		filePath = "0e7.exe"
	} else if runtime.GOOS == "linux" {
		filePath = "0e7"
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
		cmd = exec.Command("cmd.exe", "/C", "start", "upgrade.bat")
	} else {
		cmd = exec.Command("nohup ", "upgrade.sh")
	}
	fmt.Println(wdPath)
	cmd.Dir = wdPath
	err = cmd.Start()
	if err != nil {
		fmt.Println("Replace fail", err)
		return
	}
	os.Exit(0)
}
