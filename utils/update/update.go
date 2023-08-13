package update

import (
	"0E7/utils/config"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
)

var Sha256_hash []string

func calaulateSha256(filePath string) error {
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
	Sha256_hash = append(Sha256_hash, hex.EncodeToString(sha256Sum[:]))
	return nil
}
func Init_update() {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Println(err)
	}
	err = calaulateSha256(exePath)
	if err != nil {
		fmt.Println(err)
	}
	if config.Server_mode == true {
		err = calaulateSha256("0e7.exe")
		if err != nil {
			fmt.Println(err)
		}
	}
	fmt.Println("HASH:", Sha256_hash)
}
func CheckStatus() {
	args := os.Args
	if args[0] == "new_0e7.exe" {
		remove_update("0e7.exe")
		copy_update("new_0e7.exe", "0e7.exe")
		wdPath, err := os.Getwd()
		if err != nil {
			fmt.Println("exePath read fail", err)
			return
		}
		cmd := exec.Command("cmd.exe", "/C", "start", "0e7.exe")
		cmd.Dir = wdPath
		err = cmd.Start()
		if err != nil {
			fmt.Println("Update fail", err)
			return
		}
		os.Exit(0)
	} else if args[0] == "0e7.exe" {
		remove_update("new_0e7.exe")
	}
}
func remove_update(filename string) {
	fmt.Println("Remove update cache")
	for true {
		err := os.Remove(filename)
		if err != nil {
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
