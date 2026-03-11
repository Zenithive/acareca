package coa

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("coa not found")

type Repository interface {
	ListAccountTypes(ctx context.Context) ([]*AccountType, error)
	GetAccountTypeByID(ctx context.Context, id int16) (*AccountType, error)
	ListAccountTaxes(ctx context.Context) ([]*AccountTax, error)
	GetAccountTaxByID(ctx context.Context, id int16) (*AccountTax, error)

	ListCharts(ctx context.Context) ([]*ChartOfAccount, error)
	GetChartByID(ctx context.Context, id uuid.UUID) (*ChartOfAccount, error)
	CreateChart(ctx context.Context, c *ChartOfAccount) (*ChartOfAccount, error)
	UpdateChart(ctx context.Context, c *ChartOfAccount) (*ChartOfAccount, error)
	DeleteChart(ctx context.Context, id uuid.UUID) error
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

func (r *repository) ListCharts(ctx context.Context) ([]*ChartOfAccount, error) {
	query := `
		SELECT id, created_by, account_type_id, account_tax_id, code, name, description,
		       is_system, is_active, created_at, updated_at, deleted_at
		FROM tbl_chart_of_accounts
		WHERE deleted_at IS NULL
		ORDER BY code
	`
	var list []*ChartOfAccount
	if err := r.db.SelectContext(ctx, &list, query); err != nil {
		return nil, fmt.Errorf("list chart of accounts: %w", err)
	}
	return list, nil
}

func (r *repository) GetChartByID(ctx context.Context, id uuid.UUID) (*ChartOfAccount, error) {
	query := `
		SELECT id, created_by, account_type_id, account_tax_id, code, name, description,
		       is_system, is_active, created_at, updated_at, deleted_at
		FROM tbl_chart_of_accounts
		WHERE id = $1 AND deleted_at IS NULL
	`
	var c ChartOfAccount
	if err := r.db.QueryRowxContext(ctx, query, id).StructScan(&c); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get chart of account: %w", err)
	}
	return &c, nil
}

func (r *repository) CreateChart(ctx context.Context, c *ChartOfAccount) (*ChartOfAccount, error) {
	query := `
		INSERT INTO tbl_chart_of_accounts (created_by, account_type_id, account_tax_id, code, name, description, is_system, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_by, account_type_id, account_tax_id, code, name, description, is_system, is_active, created_at, updated_at, deleted_at
	`
	var out ChartOfAccount
	err := r.db.QueryRowxContext(ctx, query,
		c.CreatedBy, c.AccountTypeID, c.AccountTaxID, c.Code, c.Name, c.Description, c.IsSystem, c.IsActive,
	).StructScan(&out)
	if err != nil {
		return nil, fmt.Errorf("create chart of account: %w", err)
	}
	return &out, nil
}

func (r *repository) UpdateChart(ctx context.Context, c *ChartOfAccount) (*ChartOfAccount, error) {
	query := `
		UPDATE tbl_chart_of_accounts
		SET account_type_id = $2, account_tax_id = $3, code = $4, name = $5, description = $6, is_active = $7, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, created_by, account_type_id, account_tax_id, code, name, description, is_system, is_active, created_at, updated_at, deleted_at
	`
	var out ChartOfAccount
	err := r.db.QueryRowxContext(ctx, query,
		c.ID, c.AccountTypeID, c.AccountTaxID, c.Code, c.Name, c.Description, c.IsActive,
	).StructScan(&out)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update chart of account: %w", err)
	}
	return &out, nil
}

func (r *repository) DeleteChart(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE tbl_chart_of_accounts SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete chart of account: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
