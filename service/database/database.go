package database

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"

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
			"&_synchronous=FULL" +
			"&_cache_size=-2000" +
			"&_auto_vacuum=FULL" +
			"&_page_size=4096" +
			"&_mmap_size=268435456" +
			"&_temp_store=2" +
			"&_busy_timeout=5000" +
			"&_foreign_keys=1" +
			"&_secure_delete=OFF"
		db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
			SkipDefaultTransaction: true,
			Logger:                 logger.Default.LogMode(logger.Silent),
		})
		if err == nil {
			db.Exec("VACUUM;")
			sqlDB, _ := db.DB()
			sqlDB.SetMaxOpenConns(1)
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
		"msg": ""
	})
print(json.dumps(result))`),
				Output:   "{\"type\":\"flag_submiter\",\"num\":20}",
				Interval: 5,
			},
			{
				ID:   2,
				Name: "ipbucket_default",
				Code: CodeToBase64("code/python3",
					`import json
team = []
for i in range(1,254):
    team.append({
        "team": f"Team {i}",
        "value": f"192.168.1.{i}"
    })
print(json.dumps(team))`),
				Interval: 60,
			},
		}
		for _, action := range actions {
			db.Create(&action)
		}
	}

	// exploit
	// import sys
	//import uuid
	//ip = "127.0.0.1"
	//if len(sys.argv) == 2:
	//    ip = sys.argv[1]
	//print(f"ip:{ip} \nflag{{{str(uuid.uuid4())}}}")
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
