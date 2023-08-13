package config

import (
	"0E7/utils/database"
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	"gopkg.in/ini.v1"
	"os"
)

var Global_timeout_http int
var Global_timeout_download int

var Db *sql.DB
var Server_mode bool
var Server_port string
var Server_url string
var Server_flag string
var Client_mode bool
var Client_uuid string
var Client_pypi string
var Client_update bool

func Init_conf() error {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		fmt.Println("Failed to load config file:", err)
		os.Exit(1)
		return err
	}

	section := cfg.Section("global")
	Global_timeout_http, err = section.Key("timeout_http").Int()
	if err != nil {
		Global_timeout_http = 5
	}
	Global_timeout_download, err = section.Key("timeout_download").Int()
	if err != nil {
		Global_timeout_download = 60
	}

	section = cfg.Section("client")
	Client_mode, err = section.Key("enable").Bool()
	if err != nil {
		Client_mode = true
	}
	if Client_mode {
		Server_url = section.Key("server_url").String()
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
	}
	section = cfg.Section("server")
	Server_mode, err = section.Key("enable").Bool()
	if err != nil {
		Server_mode = false
	}
	if Server_mode {
		Db, err = database.Init_database(section)
		Server_port = section.Key("port").String()
		Server_url = section.Key("server_url").String()
		Server_flag = section.Key("flag").String()
	}
	err = cfg.SaveTo("config.ini")
	if err != nil {
		fmt.Println("Failed to save config file:", err)
		return err
	}
	return err
}
