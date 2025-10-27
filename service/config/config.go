package config

import (
	"0E7/service/database"
	"0E7/service/udpcast"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gopkg.in/ini.v1"
	"gorm.io/gorm"
)

var (
	Global_timeout_http           int
	Global_timeout_download       int
	Global_debug                  bool
	Db                            *gorm.DB
	Server_mode                   bool
	Server_tls                    bool
	Server_port                   string
	Server_url                    string
	Server_flag                   string
	Server_pcap_zip               bool
	Server_pcap_workers           int
	Client_mode                   bool
	Client_name                   string
	Client_id                     int
	Client_pypi                   string
	Client_update                 bool
	Client_worker                 int
	Client_monitor                bool
	Client_only_monitor           bool
	Search_engine                 string
	Search_elasticsearch_url      string
	Search_elasticsearch_username string
	Search_elasticsearch_password string
	Db_engine                     string
	Db_host                       string
	Db_port                       string
	Db_username                   string
	Db_password                   string
	Db_tables                     string
)

func Init_conf(configFile string) error {
	cfg, err := ini.Load(configFile)
	if err != nil {
		file, err := os.Create(configFile)
		if err != nil {
			log.Println("Create error", err)
			os.Exit(1)
		}
		defer file.Close()
		cfg, err = ini.Load(configFile)
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
		if Server_tls {
			generator_key()
			Server_url = strings.Replace(Server_url, "http://", "https://", 1)
		} else {
			Server_url = strings.Replace(Server_url, "https://", "http://", 1)
		}
		Server_pcap_zip, err = section.Key("pcap_zip").Bool()
		if err != nil {
			Server_pcap_zip = true
		}
		Server_pcap_workers, err = section.Key("pcap_workers").Int()
		if err != nil || Server_pcap_workers <= 0 {
			Server_pcap_workers = 0 // 0 表示使用 CPU 核心数
		}

		// 读取数据库配置
		Db_engine = section.Key("db_engine").String()
		if Db_engine == "" {
			Db_engine = "sqlite3" // 默认使用sqlite3
		}
		Db_host = section.Key("db_host").String()
		if Db_host == "" {
			Db_host = "localhost"
		}
		Db_port = section.Key("db_port").String()
		if Db_port == "" {
			Db_port = "3306"
		}
		Db_username = section.Key("db_username").String()
		Db_password = section.Key("db_password").String()
		Db_tables = section.Key("db_tables").String()

		Db, err = database.Init_database(section)
		if err != nil {
			log.Println("Failed to init database:", err)
			os.Exit(1)
		}
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
		Client_id, err = section.Key("id").Int()
		if err != nil {
			Client_id = 0
		}
		Client_name = section.Key("name").String()
		if Client_name == "" {
			Client_name = uuid.New().String()
			section.Key("name").SetValue(Client_name)
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

		Client_monitor, err = section.Key("monitor").Bool()
		if err != nil {
			Client_monitor = true
		}

		Client_only_monitor, err = section.Key("only_monitor").Bool()
		if err != nil {
			Client_only_monitor = false
		}
	}

	// 读取搜索引擎配置
	section = cfg.Section("search")
	Search_engine = section.Key("search_engine").String()
	if Search_engine == "" {
		Search_engine = "bleve" // 默认使用bleve
	}
	Search_elasticsearch_url = section.Key("search_elasticsearch_url").String()
	if Search_elasticsearch_url == "" {
		Search_elasticsearch_url = "http://localhost:9200" // 默认Elasticsearch地址
	}
	Search_elasticsearch_username = section.Key("search_elasticsearch_username").String()
	Search_elasticsearch_password = section.Key("search_elasticsearch_password").String()

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
		err := os.Mkdir("cert", os.ModePerm)
		if err != nil {
			log.Println("Error to create cert folder:", err)
		}
	}
	_, err1 := os.Stat("cert/private.key")
	_, err2 := os.Stat("cert/certificate.crt")
	if err1 != nil || err2 != nil {
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
		err = ioutil.WriteFile("cert/private.key", encodePrivateKeyToPEM(privateKey), os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}
		err = ioutil.WriteFile("cert/certificate.crt", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}), os.ModePerm)
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

// UpdateConfigClientId 更新config.ini文件中的client_id，如果ID发生变化则更新
func UpdateConfigClientId(clientId int) error {
	// 检查ID是否发生变化
	if Client_id == clientId {
		return nil // ID没有变化，不需要更新
	}
	log.Printf("更新客户端 ID: %d", clientId)
	cfg, err := ini.Load("config.ini")
	if err != nil {
		return fmt.Errorf("failed to load config.ini: %v", err)
	}

	// 更新client section中的id值
	clientSection := cfg.Section("client")
	clientSection.Key("id").SetValue(fmt.Sprintf("%d", clientId))

	// 保存文件
	err = cfg.SaveTo("config.ini")
	if err != nil {
		return fmt.Errorf("failed to save config.ini: %v", err)
	}

	// 更新内存中的Client_id
	Client_id = clientId

	return nil
}
