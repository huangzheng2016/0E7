package update

import (
	"0E7/utils/config"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"time"
)

var Sha256Hash []string

func calculateSha256(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	sha256Hash := sha256.New()
	if _, err := io.Copy(sha256Hash, file); err != nil {
		return err
	}
	sha256Sum := sha256Hash.Sum(nil)
	sha256 := hex.EncodeToString(sha256Sum[:])
	Sha256Hash = append(Sha256Hash, sha256)
	fmt.Println(filePath, "\tSHA256:", sha256)
	return nil
}
func InitUpdate() {
	if config.Server_mode == true {
		root := "."
		pattern := regexp.MustCompile(`^0e7_[^_]+_[^_]+$`)
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				fmt.Printf("Folder Walk Error: %v\n", err)
				return nil
			}
			if !info.IsDir() {
				if pattern.MatchString(info.Name()) {
					err = calculateSha256(info.Name())
					if err != nil {
						fmt.Println(err)
					}
				}
			}
			return nil
		})
		if err != nil {
			fmt.Println(err)
		}
	} else {
		absPath, err := filepath.Abs(os.Args[0])
		if err != nil {
			fmt.Printf("OS PATH ERROR: %v\n", err)
			return
		}
		appName := filepath.Base(absPath)
		err = calculateSha256(appName)
		if err != nil {
			fmt.Println(err)
		}
	}
	//fmt.Println("HASH:", Sha256Hash)
}
func CheckStatus() {
	fmt.Println("CheckStatus: ", time.Now().Format(time.DateTime))
	filename := "0e7_" + runtime.GOOS + "_" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		filename += ".exe"
	}
	newFilename := "new_" + filename

	absPath, err := filepath.Abs(os.Args[0])
	if err != nil {
		fmt.Printf("OS PATH ERROR: %v\n", err)
		return
	}
	appName := filepath.Base(absPath)
	if appName == newFilename {
		removeUpdate(filename)
		copy_update(newFilename, filename)
		wdPath, err := os.Getwd()
		if err != nil {
			fmt.Println("exePath read fail", err)
			return
		}
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd.exe", "/C", "start", filename)
		} else {
			cmd = exec.Command("nohup", "./"+filename, "&")
		}
		cmd.Dir = wdPath
		err = cmd.Start()
		if err != nil {
			fmt.Println("Update fail", err)
			return
		}
		os.Exit(0)
	} else if appName == filename {
		removeUpdate(newFilename)
	}
}
func removeUpdate(filename string) {
	fmt.Println("Remove update cache")
	for {
		err := os.Remove(filename)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return
			}
			fmt.Println("Remove fail. retrying...")
		} else {
			return
		}
	}
}
func copy_update(sourceFile string, destinationFile string) {
	source, err := os.Open(sourceFile)
	if err != nil {
		fmt.Println("Update Error:", err)
		return
	}
	defer source.Close()
	destination, err := os.Create(destinationFile)
	if err != nil {
		fmt.Println("Update Error:", err)
		return
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	if err != nil {
		fmt.Println("Update Error:", err)
		return
	}
	fmt.Println("Copy Success")
}
