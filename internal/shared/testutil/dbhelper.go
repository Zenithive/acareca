package testutil

import (
	"context"
	"database/sql"
	"testing"

	"github.com/iamarpitzala/acareca/internal/shared/db"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
)

func OpenTestDB(t *testing.T) *sqlx.DB {
	t.Helper()

	cfg := config.NewConfig()
	dbConn, err := db.DBConn(cfg)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}

	if err := dbConn.Ping(); err != nil {
		t.Fatalf("ping test database: %v", err)
	}
	return dbConn
}

func BeginTx(t *testing.T, dbConn *sqlx.DB) *sqlx.Tx {
	t.Helper()

	tx, err := dbConn.BeginTxx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	return tx
}

func CommitTx(t *testing.T, tx *sqlx.Tx) {
	t.Helper()

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}
}

func RollbackTx(t *testing.T, tx *sqlx.Tx) {
	t.Helper()

	if tx == nil {
		return
	}
	if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
		t.Fatalf("rollback tx: %v", err)
	}
}
