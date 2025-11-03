package database

import (
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"gopkg.in/ini.v1"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Init_database(section *ini.Section) (db *gorm.DB, err error) {
	engine := section.Key("db_engine").String()
	switch engine {
	case "mysql":
		host := section.Key("db_host").String()
		port := section.Key("db_port").String()
		username := section.Key("db_username").String()
		password := section.Key("db_password").String()
		tables := section.Key("db_tables").String()
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local&compress=True",
			username, password, host, port, tables)
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
			SkipDefaultTransaction: true,
			Logger:                 logger.Default.LogMode(logger.Silent),
		})
	default:
		engine = "sqlite"
		dsn := "file:sqlite.db?mode=rwc" +
			"&_journal_mode=WAL" +
			"&_synchronous=NORMAL" + // 改为 NORMAL 提高性能
			"&_cache_size=-4000" + // 4GB 缓存
			"&_auto_vacuum=FULL" +
			"&_page_size=4096" +
			"&_mmap_size=536870912" + // 512MB 内存映射
			"&_temp_store=2" +
			"&_busy_timeout=10000" + // 10秒超时
			"&_foreign_keys=1" +
			"&_secure_delete=OFF" +
			"&_journal_size_limit=67108864" + // 64MB WAL 限制
			"&_wal_autocheckpoint=1000" // WAL 检查点
		db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
			SkipDefaultTransaction: true,
			Logger:                 logger.Default.LogMode(logger.Silent),
		})
		if err == nil {
			// 连接池限制
			sqlDB, _ := db.DB()
			sqlDB.SetMaxOpenConns(1)

			// 确保元信息表一次性创建
			db.Exec("CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value INTEGER)")

			var vacuumFlag = flag.Bool("db_vacuum", false, "run VACUUM on startup (sqlite only)")
			forceVacuum := *vacuumFlag || os.Getenv("DB_VACUUM") == "1" || os.Getenv("DB_VACUUM") == "true"

			shouldVacuum := false
			if forceVacuum {
				shouldVacuum = true
			} else {
				// 读取最近一次 VACUUM 时间
				var lastVacUnix int64
				row := db.Raw("SELECT value FROM meta WHERE key = 'last_vacuum' LIMIT 1").Row()
				if scanErr := row.Scan(&lastVacUnix); scanErr == nil {
					lastVacTime := time.Unix(lastVacUnix, 0)
					if time.Since(lastVacTime) >= 24*time.Hour {
						shouldVacuum = true
					}
				} else {
					// 没有该记录/表：初始化 last_vacuum 为当前时间
					nowUnix := time.Now().Unix()
					db.Exec("INSERT INTO meta(key, value) VALUES('last_vacuum', ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", nowUnix)
				}
			}

			if shouldVacuum {
				db.Exec("VACUUM;")
				// 记录本次 VACUUM 时间
				nowUnix := time.Now().Unix()
				db.Exec("INSERT INTO meta(key, value) VALUES('last_vacuum', ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", nowUnix)
			}
		}
	}

	if err != nil {
		log.Println("Failed to open database: ", err)
		return db, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Println("Failed to get underlying sql.DB:", err)
		return db, err
	}

	err = sqlDB.Ping()
	if err != nil {
		log.Println("Failed to connect to database:", err)
		os.Exit(1)
	}

	log.Println("Connected to database:", engine)

	err = init_database_client(db, engine)
	return db, err
}

func init_database_client(db *gorm.DB, engine string) error {
	// 自动迁移所有表结构
	err := db.AutoMigrate(
		&Client{},
		&Exploit{},
		&Flag{},
		&ExploitOutput{},
		&Action{},
		&PcapFile{},
		&Monitor{},
		&Pcap{},
	)
	if err != nil {
		log.Println("Failed to migrate database tables:", err)
		return err
	}

	// 插入默认的 action 数据
	var count int64
	db.Model(&Action{}).Count(&count)
	if count == 0 {
		actions := []Action{
			{
				ID:   1,
				Name: "flag_submiter",
				Code: CodeToBase64("code/python3",
					`import sys
import json
if len(sys.argv) != 2:
	print(json.dumps([]))
	sys.exit(0)
	
data = json.loads(sys.argv[1])
result = []
for item in data:
	result.append({
		"flag": item,
		"status": "SUCCESS",
		"msg": "",
		"score": 100.0
	})
print(json.dumps(result))`),
				Config:   "{\"type\":\"flag_submiter\",\"num\":20}",
				Interval: 5,
			},
			{
				ID:   2,
				Name: "ipbucket_default",
				Code: CodeToBase64("code/python3",
					`import json
team = []
for i in range(1,10):
    team.append({
        "team": f"Team {i}",
        "value": f"192.168.1.{i}"
    })
print(json.dumps(team))`),
				Interval: -1,
				NextRun:  time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			{
				ID:       3,
				Name:     "run_exploit_1",
				Code:     "",
				Config:   "{\"type\":\"exec_script\",\"num\":1,\"script_id\":1}",
				Interval: -1, // 默认不启用
				NextRun:  time.Date(2037, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		}
		for _, action := range actions {
			db.Create(&action)
		}

		// 插入代码生成模板
		codeTemplates := []Action{
			{
				ID:   4,
				Name: "requests_template",
				Code: CodeToBase64("code/python3",
					`import requests
import base64

# 禁用SSL验证和保持连接
session = requests.Session()
session.verify = False
session.headers.update({
    'Connection': 'close'  # 不保持连接
})

# 请求数据
url = "{{.URL}}"
headers = {{.Headers}}

# 使用base64解码的数据（默认）
data = base64.b64decode("{{.Data}}").decode('utf-8', errors='ignore')

# 或者直接使用原始数据（取消注释下面一行，注释上面一行）
# data = "{{.RawData}}"

# 发送请求
response = session.post(url, headers=headers, data=data)
print(f"Status: {response.status_code}")
print(f"Response: {response.text}")`),
				Config:   "{\"type\": \"template\"}",
				Interval: -1, // 默认不启用
				NextRun:  time.Date(2037, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			{
				ID:   5,
				Name: "pwntools_template",
				Code: CodeToBase64("code/python3",
					`from pwn import *

# 连接设置
context.log_level = 'debug'

# 连接信息
host = "{{.Host}}"
port = {{.Port}}

# 原始数据 - 用户可以直接修改这里的数据
raw_data = "{{.RawData}}"

# 建立连接
conn = remote(host, port)

# 发送原始数据
conn.send(raw_data.encode())

# 接收响应
response = conn.recvall()
print(response.decode('utf-8', errors='ignore'))

conn.close()`),
				Config:   "{\"type\": \"template\"}",
				Interval: -1, // 默认不启用
				NextRun:  time.Date(2037, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			{
				ID:   6,
				Name: "curl_template",
				Code: CodeToBase64("code/bash",
					`#!/bin/bash

# 请求数据
URL="{{.URL}}"
DATA="{{.Data}}"

# 解码base64数据（默认）
DECODED_DATA=$(echo "$DATA" | base64 -d)

# 或者直接使用原始数据（取消注释下面一行，注释上面一行）
# DECODED_DATA="{{.RawData}}"

# 构建curl命令
curl -X POST \\
  --insecure \\
  --no-keepalive \\
  {{.HeadersCurl}}  --data "$DECODED_DATA" \\
  "$URL"`),
				Config:   "{\"type\": \"template\"}",
				Interval: -1, // 默认不启用
				NextRun:  time.Date(2037, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		}
		for _, template := range codeTemplates {
			db.Create(&template)
		}
	}
	db.Model(&Exploit{}).Count(&count)
	if count == 0 {
		exploits := []Exploit{
			{
				ID:   1,
				Name: "rand_flag",
				Filename: CodeToBase64("code/python3",
					`import sys
import uuid
ip = "127.0.0.1"
if len(sys.argv) == 2:
    ip = sys.argv[1]
print(f"ip:{ip} \nflag{{{str(uuid.uuid4())}}}")`),
				Timeout:   "15",
				Times:     "0",
				Flag:      "flag{.*}",
				Argv:      "{ipbucket_default}",
				IsDeleted: false,
			},
		}
		for _, exploit := range exploits {
			db.Create(&exploit)
		}
	}

	log.Println("Database tables migrated successfully.")
	return nil
}

func CodeToBase64(codeType string, code string) string {
	return fmt.Sprintf("data:%s;base64,%s", codeType, base64.StdEncoding.EncodeToString([]byte(code)))
}
