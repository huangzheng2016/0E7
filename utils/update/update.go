package update

import (
	"0E7/utils/config"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

var Sha256_hash []string
var conf config.Conf

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
func Init_update(sconf config.Conf) {
	conf = sconf
	exePath, err := os.Executable()
	if err != nil {
		fmt.Println(err)
	}
	err = calaulateSha256(exePath)
	if err != nil {
		fmt.Println(err)
	}
	if conf.Server_mode == true {
		err = calaulateSha256("0e7.exe")
		if err != nil {
			fmt.Println(err)
		}
	}
	fmt.Println(Sha256_hash)
}
