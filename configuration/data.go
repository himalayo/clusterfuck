package main

import (
	"database/sql"
	"os"
	"log"

	"github.com/go-sql-driver/mysql"
)

type Database struct {
	db *sql.DB
}

func ConfigDatabaseFromEnv() *mysql.Config {
	cfg := mysql.NewConfig()
	cfg.User = os.Getenv("DB_USER")
	cfg.Passwd = os.Getenv("DB_PASS")
	cfg.Net = "tcp"
	cfg.Addr = os.Getenv("DB_ADDR")
	cfg.DBName = os.Getenv("DB_NAME")

	return cfg
}

func ConfigDatabase(user string, password string, address string, name string) *mysql.Config {
	cfg := mysql.NewConfig()
	cfg.User = user
	cfg.Passwd = password
	cfg.Net = "tcp"
	cfg.Addr = address
	cfg.DBName = name

	return cfg
}

func NewDatabase(cfg *mysql.Config) *Database {
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		log.Fatal(err)
	}

	return &Database{db: db}
}

func (data *Database) GetInt(key string) (int, error) {
	var out int
	row := data.db.QueryRow("SELECT value FROM emulator_settings WHERE `key` = ?", key)
	if err := row.Scan(&out); err != nil {
		return out, err
	}
	return out, nil
}


func (data *Database) GetDouble(key string) (float64, error) {
	var out float64
	row := data.db.QueryRow("SELECT value FROM emulator_settings WHERE `key` = ?", key)
	if err := row.Scan(&out); err != nil {
		return out, err
	}
	return out, nil
}

func (data *Database) GetString(key string) (string, error) {
	var out string
	row := data.db.QueryRow("SELECT value FROM emulator_settings WHERE `key` = ?", key)
	if err := row.Scan(&out); err != nil {
		return out, err
	}
	return out, nil
}

func (data *Database) GetBoolean(key string) (bool, error) {
	rawValue, err := data.GetString(key)
	if err != nil {
		return false, err
	}
	return rawValue ==  "1", nil
}
