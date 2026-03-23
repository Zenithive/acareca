package db

import (
	"fmt"
	"sync"
	"time"

	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var (
	instance *sqlx.DB
	initErr  error
	dbOnce   sync.Once
)

func DBConn(cfg *config.Config) (*sqlx.DB, error) {
	dbOnce.Do(func() {
		dsn := fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode,
		)

		instance, initErr = sqlx.Connect("postgres", dsn)
		if initErr != nil {
			return
		}

		instance.SetMaxOpenConns(25)
		instance.SetMaxIdleConns(5)
		instance.SetConnMaxIdleTime(5 * time.Minute)
	})

	if instance == nil {
		return nil, fmt.Errorf("database connection failed: %w", initErr)
	}

	return instance, nil
}
