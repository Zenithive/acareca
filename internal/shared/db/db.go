package db

import (
	"fmt"

	"sync"

	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var (
	db     *sqlx.DB
	dbOnce sync.Once
)

func DBConn(cfg *config.Config) (*sqlx.DB, error) {
	var err error
	dbOnce.Do(func() {
		dsn := fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode,
		)

		db, err = sqlx.Connect("postgres", dsn)
		if err != nil {
			return
		}

		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)
	})

	if db == nil {
		return nil, fmt.Errorf("database connection failed: %w", err)
	}

	return db, nil
}
