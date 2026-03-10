package coa

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("coa not found")

type Repository interface {
	ListAccountTypes(ctx context.Context) ([]*AccountType, error)
	GetAccountTypeByID(ctx context.Context, id int16) (*AccountType, error)
	ListAccountTaxes(ctx context.Context) ([]*AccountTax, error)
	GetAccountTaxByID(ctx context.Context, id int16) (*AccountTax, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) ListAccountTypes(ctx context.Context) ([]*AccountType, error) {
	query := `
		SELECT id, name, description, created_at, updated_at
		FROM tbl_account_type
		ORDER BY id
	`
	var list []*AccountType
	if err := r.db.SelectContext(ctx, &list, query); err != nil {
		return nil, fmt.Errorf("list account types: %w", err)
	}
	return list, nil
}

func (r *repository) ListAccountTaxes(ctx context.Context) ([]*AccountTax, error) {
	query := `
		SELECT id, name, rate, bas_field, is_taxable, description, created_at, updated_at
		FROM tbl_account_tax
		ORDER BY id
	`
	var list []*AccountTax
	if err := r.db.SelectContext(ctx, &list, query); err != nil {
		return nil, fmt.Errorf("list account taxes: %w", err)
	}
	return list, nil
}

func (r *repository) GetAccountTypeByID(ctx context.Context, id int16) (*AccountType, error) {
	query := `
		SELECT id, name, description, created_at, updated_at
		FROM tbl_account_type
		WHERE id = $1
	`
	var a AccountType
	if err := r.db.QueryRowxContext(ctx, query, id).StructScan(&a); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get account type: %w", err)
	}
	return &a, nil
}

func (r *repository) GetAccountTaxByID(ctx context.Context, id int16) (*AccountTax, error) {
	query := `
		SELECT id, name, rate, bas_field, is_taxable, description, created_at, updated_at
		FROM tbl_account_tax
		WHERE id = $1
	`
	var a AccountTax
	if err := r.db.QueryRowxContext(ctx, query, id).StructScan(&a); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get account tax: %w", err)
	}
	return &a, nil
}
