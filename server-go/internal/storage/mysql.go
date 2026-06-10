package storage

import (
	"database/sql"
	"time"

	"auction-system/server-go/internal/config"

	_ "github.com/go-sql-driver/mysql"
)

func OpenMySQL(cfg config.Config) (*sql.DB, error) {
	db, err := sql.Open("mysql", cfg.MySQLDSN)
	if err != nil {
		return nil, err
	}
	if cfg.MySQLMaxOpen > 0 {
		db.SetMaxOpenConns(cfg.MySQLMaxOpen)
	}
	if cfg.MySQLMaxIdle > 0 {
		db.SetMaxIdleConns(cfg.MySQLMaxIdle)
	}
	db.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}
