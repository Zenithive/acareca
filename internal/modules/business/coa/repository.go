package coa

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var (
	ErrNotFound               = errors.New("coa not found")
	ErrCodeExists             = errors.New("code already exists")
	ErrSystemAccountProtected = errors.New("system account cannot be updated or deleted")
)

type Repository interface {
	ListAccountTypes(ctx context.Context) ([]*AccountType, error)
	GetAccountType(ctx context.Context, id int16) (*AccountType, error)
	ListAccountTaxes(ctx context.Context) ([]*AccountTax, error)
	GetAccountTax(ctx context.Context, id int16) (*AccountTax, error)

	ListChartOfAccount(ctx context.Context, practitionerID uuid.UUID) ([]*ChartOfAccount, error)
	GetChartOfAccount(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID) (*ChartOfAccount, error)
	GetChartByCodeAndPractitionerID(ctx context.Context, code int16, practitionerID uuid.UUID, excludeID *uuid.UUID) (*ChartOfAccount, error)
	CreateChartOfAccount(ctx context.Context, c *ChartOfAccount) (*ChartOfAccount, error)
	UpdateCharOfAccount(ctx context.Context, c *ChartOfAccount) (*ChartOfAccount, error)
	DeleteChartOfAccount(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID) error
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) ListAccountTypes(ctx context.Context) ([]*AccountType, error) {
	query := `
		SELECT id, name, created_at, updated_at
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
		SELECT id, name, rate, is_taxable, created_at, updated_at
		FROM tbl_account_tax
		ORDER BY id
	`
	var list []*AccountTax
	if err := r.db.SelectContext(ctx, &list, query); err != nil {
		return nil, fmt.Errorf("list account taxes: %w", err)
	}
	return list, nil
}

func (r *repository) GetAccountType(ctx context.Context, id int16) (*AccountType, error) {
	query := `
		SELECT id, name, created_at, updated_at
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

func (r *repository) GetAccountTax(ctx context.Context, id int16) (*AccountTax, error) {
	query := `
		SELECT id, name, rate, is_taxable, created_at, updated_at
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

func (r *repository) ListChartOfAccount(ctx context.Context, practitionerID uuid.UUID) ([]*ChartOfAccount, error) {
	query := `
		SELECT id, practitioner_id, account_type_id, account_tax_id, code, name,
		       is_system, created_at, updated_at, deleted_at
		FROM tbl_chart_of_accounts
		WHERE practitioner_id = $1 AND deleted_at IS NULL
		ORDER BY code
	`
	var list []*ChartOfAccount
	if err := r.db.SelectContext(ctx, &list, query, practitionerID); err != nil {
		return nil, fmt.Errorf("list chart of accounts by practitioner_id: %w", err)
	}
	return list, nil
}

func (r *repository) GetChartOfAccount(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID) (*ChartOfAccount, error) {
	query := `
		SELECT id, practitioner_id, account_type_id, account_tax_id, code, name,
		       is_system, created_at, updated_at, deleted_at
		FROM tbl_chart_of_accounts
		WHERE id = $1 AND practitioner_id = $2 AND deleted_at IS NULL
	`
	var c ChartOfAccount
	if err := r.db.QueryRowxContext(ctx, query, id, practitionerID).StructScan(&c); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get chart by id and practitioner_id: %w", err)
	}
	return &c, nil
}

func (r *repository) GetChartByCodeAndPractitionerID(ctx context.Context, code int16, practitionerID uuid.UUID, excludeID *uuid.UUID) (*ChartOfAccount, error) {
	query := `
		SELECT id, practitioner_id, account_type_id, account_tax_id, code, name,
		       is_system, created_at, updated_at, deleted_at
		FROM tbl_chart_of_accounts
		WHERE code = $1 AND practitioner_id = $2 AND deleted_at IS NULL
	`
	args := []interface{}{code, practitionerID}
	if excludeID != nil {
		query += ` AND id != $3`
		args = append(args, *excludeID)
	}
	query += ` LIMIT 1`
	var c ChartOfAccount
	if err := r.db.QueryRowxContext(ctx, query, args...).StructScan(&c); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get chart by code and practitioner_id: %w", err)
	}
	return &c, nil
}

func (r *repository) CreateChartOfAccount(ctx context.Context, c *ChartOfAccount) (*ChartOfAccount, error) {
	query := `
		INSERT INTO tbl_chart_of_accounts (practitioner_id, account_type_id, account_tax_id, code, name, is_system)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, practitioner_id, account_type_id, account_tax_id, code, name, is_system, created_at, updated_at, deleted_at
	`
	var out ChartOfAccount
	err := r.db.QueryRowxContext(ctx, query,
		c.PractitionerID, c.AccountTypeID, c.AccountTaxID, c.Code, c.Name, c.IsSystem,
	).StructScan(&out)
	if err != nil {
		return nil, fmt.Errorf("create chart of account: %w", err)
	}
	return &out, nil
}

func (r *repository) UpdateCharOfAccount(ctx context.Context, c *ChartOfAccount) (*ChartOfAccount, error) {
	query := `
		UPDATE tbl_chart_of_accounts
		SET account_type_id = $2, account_tax_id = $3, code = $4, name = $5, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, practitioner_id, account_type_id, account_tax_id, code, name, is_system, created_at, updated_at, deleted_at
	`
	var out ChartOfAccount
	err := r.db.QueryRowxContext(ctx, query,
		c.ID, c.AccountTypeID, c.AccountTaxID, c.Code, c.Name,
	).StructScan(&out)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update chart of account: %w", err)
	}
	return &out, nil
}

func (r *repository) DeleteChartOfAccount(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID) error {
	query := `UPDATE tbl_chart_of_accounts SET deleted_at = now(), updated_at = now() WHERE id = $1 AND practitioner_id = $2 AND deleted_at IS NULL`
	res, err := r.db.ExecContext(ctx, query, id, practitionerID)
	if err != nil {
		return fmt.Errorf("delete chart of account: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
