package db

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/pressly/goose/v3"
)

func RunMigrations(db *sql.DB) error {
	log.Println("Running database migrations with goose...")

	migrationsDir := "../../migrations"
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	if err := goose.Up(db, migrationsDir); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("Database migrations completed successfully")
	return nil
}
