package database

import (
	"database/sql"
	"fmt"
	_ "github.com/glebarez/sqlite"
	"gopkg.in/ini.v1"
	"os"
)

func Init_database(section *ini.Section) (db *sql.DB, err error) {
	engine := section.Key("db_engine").String()
	switch engine {
	/*
		case "mysql":
			host := section.Key("db_host").String()
			port := section.Key("db_port").String()
			username := section.Key("db_username").String()
			password := section.Key("db_password").String()
			tables := section.Key("db_tables").String()
			db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", username, password, host, port, tables))
		//case "sqlite3":
	*/
	default:
		engine = "sqlite3"
		db, err = sql.Open("sqlite", "sqlite.db")
		/*
			default:
				fmt.Println("Unknown database engine:", engine)
				return db, err
		*/
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
	case "sqlite3":
		stmt, err = db.Prepare(`
		CREATE TABLE IF NOT EXISTS '0e7_client' (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
			uuid TEXT NOT NULL,
			hostname TEXT NOT NULL,
            platform TEXT NOT NULL,
            arch TEXT NOT NULL,
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

	switch engine {
	case "sqlite3":
		stmt, err = db.Prepare(`
		CREATE TABLE IF NOT EXISTS '0e7_exploit' (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
			uuid TEXT NOT NULL,
			filename TEXT NOT NULL,
			environment TEXT,
            command TEXT,
            argv TEXT,
            platform TEXT,
            arch TEXT,
            filter TEXT,
            times TEXT NOT NULL
        )
	`)
	}
	defer stmt.Close()
	_, err = stmt.Exec()
	if err != nil {
		fmt.Println("Table '0e7_exploit' create failed", err)
		return err
	}
	fmt.Println("Table '0e7_exploit' is created successfully.")

	switch engine {
	case "sqlite3":
		stmt, err = db.Prepare(`
		CREATE TABLE IF NOT EXISTS '0e7_flag' (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
			uuid TEXT NOT NULL,
			flag TEXT NOT NULL,
			status TEXT,
			updated TEXT                 
        )
	`)
	}
	defer stmt.Close()
	_, err = stmt.Exec()
	if err != nil {
		fmt.Println("Table '0e7_flag' create failed", err)
		return err
	}
	fmt.Println("Table '0e7_flag' is created successfully.")

	switch engine {
	case "sqlite3":
		stmt, err = db.Prepare(`
		CREATE TABLE IF NOT EXISTS '0e7_exploit_output' (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
			uuid TEXT NOT NULL,
			client TEXT NOT NULL,
			output TEXT,
			status TEXT,
			updated TEXT          
        )
	`)
	}
	defer stmt.Close()
	_, err = stmt.Exec()
	if err != nil {
		fmt.Println("Table '0e7_exploit_output' create failed", err)
		return err
	}
	fmt.Println("Table '0e7_exploit_output' is created successfully.")

	return err
}
