package database

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/ini.v1"
	"os"
)

func Init_database(section *ini.Section) (db *sql.DB, err error) {
	engine := section.Key("db_engine").String()
	switch engine {
	case "mysql":
		host := section.Key("db_host").String()
		port := section.Key("db_port").String()
		username := section.Key("db_username").String()
		password := section.Key("db_password").String()
		tables := section.Key("db_tables").String()
		db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", username, password, host, port, tables))
	case "sqlite3":
		db, err = sql.Open("sqlite3", "sqlite.db")
	default:
		fmt.Println("Unknown database engine:", engine)
		return db, err
	}

	if err != nil {
		fmt.Println("Failed to open database:", err)
		return db, err
	}

	err = db.Ping()
	if err != nil {
		fmt.Println("Failed to connect to database:", err)
		os.Exit(1)
	}

	fmt.Println("Connected to database:", engine)

	init_database_client(db, engine)
	return db, err
}

func init_database_client(db *sql.DB, engine string) error {
	var stmt *sql.Stmt
	var err error
	switch engine {
	case "mysql":
		stmt, err = db.Prepare(`
		CREATE TABLE IF NOT EXISTS '0e7_client' (
			id INTEGER PRIMARY KEY AUTO_INCREMENT,
			uuid TEXT NOT NULL,
			hostname TEXT NOT NULL,
			cpu TEXT NOT NULL,
            cpu_use TEXT NOT NULL,
            memory_use TEXT NOT NULL,
            memory_max TEXT NOT NULL,
			updated TEXT NOT NULL
		)
    `)
	case "sqlite3":
		stmt, err = db.Prepare(`
		CREATE TABLE IF NOT EXISTS '0e7_client' (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
			uuid TEXT NOT NULL,
			hostname TEXT NOT NULL,
            cpu TEXT NOT NULL,
            cpu_use TEXT NOT NULL,
            memory_use TEXT NOT NULL,
            memory_max TEXT NOT NULL,
            updated TEXT NOT NULL
        )
	`)
	}
	defer stmt.Close()
	_, err = stmt.Exec()
	if err != nil {
		fmt.Println("Table '0e7_client' create failed", err)
		return err
	}
	fmt.Println("Table '0e7_client' is created successfully.")
	return err
}
