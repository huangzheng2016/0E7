package config

import (
	"0E7/utils/database"
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	"gopkg.in/ini.v1"
)

type Conf struct {
	Db          *sql.DB
	Server_mode bool
	Server_port string
	Server_url  string
	Client_mode bool
	Client_uuid string
}

func Init_conf() (conf Conf, err error) {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		fmt.Println("Failed to load config file:", err)
		return conf, err
	}
	section := cfg.Section("client")
	conf.Client_mode, _ = section.Key("enable").Bool()
	if err != nil {
		conf.Client_mode = false
	}
	if conf.Client_mode {
		conf.Server_url = section.Key("server_url").String()
		conf.Client_uuid = section.Key("uuid").String()
		if conf.Client_uuid == "" {
			conf.Client_uuid = uuid.New().String()
			section.Key("uuid").SetValue(conf.Client_uuid)
		}
	}
	section = cfg.Section("server")
	conf.Server_mode, _ = section.Key("enable").Bool()
	if err != nil {
		conf.Server_mode = false
	}
	if conf.Server_mode {
		conf.Db, err = database.Init_database(section)
		conf.Server_port = section.Key("port").String()
		conf.Server_url = section.Key("server_url").String()
	}
	err = cfg.SaveTo("config.ini")
	if err != nil {
		fmt.Println("Failed to save config file:", err)
		return conf, err
	}
	return conf, err
}
