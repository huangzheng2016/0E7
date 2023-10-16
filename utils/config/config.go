package config

import (
	"0E7/utils/database"
	"0E7/utils/udpcast"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"github.com/google/uuid"
	"gopkg.in/ini.v1"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"
)

var Global_timeout_http int
var Global_timeout_download int
var Global_debug bool

var Db *sql.DB
var Server_mode bool
var Server_tls bool
var Server_port string
var Server_url string
var Server_flag string
var Client_mode bool
var Client_uuid string
var Client_pypi string
var Client_update bool
var Client_worker int

func Init_conf() error {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		file, err := os.Create("config.ini")
		if err != nil {
			log.Println("Create error", err)
			os.Exit(1)
		}
		defer file.Close()
		cfg, err = ini.Load("config.ini")
		if err != nil {
			log.Println("Failed to load config file:", err)
			os.Exit(1)
		}
	}
	Server_url = ""
	section := cfg.Section("global")
	Global_timeout_http, err = section.Key("timeout_http").Int()
	if err != nil {
		Global_timeout_http = 5
	}
	Global_timeout_download, err = section.Key("timeout_download").Int()
	if err != nil {
		Global_timeout_download = 60
	}
	Global_debug, err = section.Key("debug").Bool()
	if err != nil {
		Global_debug = false
	}

	section = cfg.Section("client")
	Client_mode, err = section.Key("enable").Bool()
	if err != nil {
		Client_mode = true
	}

	section = cfg.Section("server")
	Server_mode, err = section.Key("enable").Bool()
	if err != nil {
		Server_mode = false
	}
	if Server_mode {
		Server_port = section.Key("port").String()
		if Server_port == "" {
			Server_port = "6102"
		}
		Server_url = section.Key("server_url").String()
		Server_flag = section.Key("flag").String()
		Server_tls, err = section.Key("tls").Bool()
		if err != nil {
			Server_tls = true
		}
		if Server_tls == true {
			generator_key()
			Server_url = strings.Replace(Server_url, "http://", "https://", 1)
		} else {
			Server_url = strings.Replace(Server_url, "https://", "http://", 1)
		}
		Db, err = database.Init_database(section)
	}

	section = cfg.Section("client")
	var wg sync.WaitGroup
	if Client_mode {
		if Server_url == "" {
			Server_url = section.Key("server_url").String()
		}
		if Server_url == "" {
			args := os.Args
			if len(args) == 2 {
				Server_url = args[1]
			} else if Server_mode == false {
				wg.Add(1)
				go udpcast.Udp_receive(&wg, &Server_url)
			} else {
				if Server_tls == true {
					Server_url = "https://localhost:" + Server_port
				} else {
					Server_url = "http://localhost:" + Server_port
				}
			}
			if Server_url != "" {
				section.Key("server_url").SetValue(Server_url)
			}
		}
		Client_uuid = section.Key("uuid").String()
		if Client_uuid == "" {
			Client_uuid = uuid.New().String()
			section.Key("uuid").SetValue(Client_uuid)
		}

		Client_pypi = section.Key("pypi").String()
		if Client_pypi == "" {
			Client_pypi = "https://pypi.tuna.tsinghua.edu.cn/simple"
		}

		Client_update, err = section.Key("update").Bool()
		if err != nil {
			Client_update = true
		}

		Client_worker, err = section.Key("worker").Int()
		if err != nil {
			Client_worker = 5
		}
	}
	wg.Wait()
	if Client_mode && Server_url == "" {
		log.Println("Server not found")
		os.Exit(1)
	}

	err = cfg.SaveTo("config.ini")
	if err != nil {
		log.Println("Failed to save config file:", err)
		return err
	}

	return nil
}

func generator_key() {
	if _, err := os.Stat("cert"); os.IsNotExist(err) {
		err := os.Mkdir("cert", 0660)
		if err != nil {
			log.Println("Error to create cert folder:", err)
		}
	}
	_, err1 := os.Stat("cert/private.key")
	_, err2 := os.Stat("cert/certificate.crt")
	if os.IsNotExist(err1) || os.IsNotExist(err2) {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			log.Fatal(err)
		}
		template := x509.Certificate{
			SerialNumber:          big.NewInt(1),
			NotBefore:             time.Now(),
			NotAfter:              time.Now().AddDate(10, 0, 0), // 有效期为十年
			BasicConstraintsValid: true,
		}
		derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
		if err != nil {
			log.Fatal(err)
		}
		err = ioutil.WriteFile("cert/private.key", encodePrivateKeyToPEM(privateKey), 0600)
		if err != nil {
			log.Fatal(err)
		}
		err = ioutil.WriteFile("cert/certificate.crt", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}), 0644)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func encodePrivateKeyToPEM(privateKey *rsa.PrivateKey) []byte {
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	return pem.EncodeToMemory(privateKeyPEM)
}
