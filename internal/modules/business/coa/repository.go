package coa

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

var (
	ErrNotFound               = errors.New("coa not found")
	ErrCodeExists             = errors.New("code already exists")
	ErrSystemAccountProtected = errors.New("system account cannot be updated or deleted")
)

type Repository interface {
	ListAccountTypes(ctx context.Context, f common.Filter) ([]*AccountType, error)
	GetAccountType(ctx context.Context, id int16) (*AccountType, error)
	ListAccountTaxes(ctx context.Context, f common.Filter) ([]*AccountTax, error)
	GetAccountTax(ctx context.Context, id int16) (*AccountTax, error)
	GetAccountTypeByName(ctx context.Context, name string) (int, error)

	ListChartOfAccount(ctx context.Context, actorID uuid.UUID, role string, f common.Filter) ([]*ChartOfAccount, error)
	CountChartOfAccount(ctx context.Context, actorID uuid.UUID, role string, f common.Filter) (int, error)
	GetChartOfAccount(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID) (*ChartOfAccount, error)
	GetChartOfAccountByKey(ctx context.Context, key string, practitionerID uuid.UUID) (*ChartOfAccount, error)
	GetChartByCodeAndPractitionerID(ctx context.Context, code int16, practitionerID uuid.UUID, excludeID *uuid.UUID) (*ChartOfAccount, error)
	CreateChartOfAccount(ctx context.Context, c *ChartOfAccount, tx *sqlx.Tx) (*ChartOfAccount, error)
	BulkCreateChartOfAccounts(ctx context.Context, rows []*ChartOfAccount, tx *sqlx.Tx) error
	UpdateCharOfAccount(ctx context.Context, c *ChartOfAccount) (*ChartOfAccount, error)
	DeleteChartOfAccount(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID) error
	GetByIDInternal(ctx context.Context, id uuid.UUID) (*ChartOfAccount, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) ListAccountTypes(ctx context.Context, f common.Filter) ([]*AccountType, error) {
	base := `
		SELECT id, name, created_at, updated_at
		FROM tbl_account_type
		WHERE 1=1
		`

	searchCols := []string{"name"}
	colMap := map[string]string{"id": "id", "name": "name"}

	query, filterArgs := common.BuildQuery(base, f, colMap, searchCols, false)
	query = r.db.Rebind(query)

	var list []*AccountType
	if err := r.db.SelectContext(ctx, &list, query, filterArgs...); err != nil {
		return nil, fmt.Errorf("list account types: %w", err)
	}
	return list, nil
}

func (r *repository) ListAccountTaxes(ctx context.Context, f common.Filter) ([]*AccountTax, error) {
	base := `
		SELECT id, name, rate, is_taxable, created_at, updated_at
		FROM tbl_account_tax
		WHERE 1=1
	`
	searchCols := []string{"name", "is_taxable"}
	colMap := map[string]string{"id": "id", "name": "name", "rate": "rate"}

	query, filterArgs := common.BuildQuery(base, f, colMap, searchCols, false)
	query = r.db.Rebind(query)

	var list []*AccountTax
	if err := r.db.SelectContext(ctx, &list, query, filterArgs...); err != nil {
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

var chartOfAccountColumns = map[string]string{
	"practitioner_id": "practitioner_id",
	"id":              "coa.id",
	"account_type_id": "coa.account_type_id",
	"account_tax_id":  "coa.account_tax_id",
	"code":            "coa.code",
	"name":            "coa.name",
	"key":             "coa.key",
	"is_system":       "coa.is_system",
	"created_at":      "coa.created_at",
}

var coaSearchColumns = []string{"coa.name", "CAST(coa.code AS TEXT)"}

func (r *repository) ListChartOfAccount(ctx context.Context, actorID uuid.UUID, role string, f common.Filter) ([]*ChartOfAccount, error) {
	base := `
        SELECT 
            coa.id, coa.practitioner_id, coa.account_type_id, coa.account_tax_id,
            coa.code, coa.name, coa.key, coa.is_system, at.is_taxable, coa.created_at, coa.updated_at
        FROM tbl_chart_of_accounts coa
        JOIN tbl_account_tax at ON at.id = coa.account_tax_id
        WHERE coa.deleted_at IS NULL
    `

	// Use "AND" instead of "WHERE" since we started with deleted_at IS NULL
	if role == util.RoleAccountant {
		base += fmt.Sprintf(` AND EXISTS (
                SELECT 1 FROM tbl_invitation inv 
                WHERE inv.practitioner_id = coa.practitioner_id 
                AND inv.accountant_id = '%s' 
                AND inv.status = 'COMPLETED'
            )`, actorID.String())
	} else {
		base += fmt.Sprintf(` AND coa.practitioner_id = '%s'`, actorID.String())
	}

	// Now BuildQuery will correctly append any EXTRA search/filter params from the UI
	query, filterArgs := common.BuildQuery(base, f, chartOfAccountColumns, coaSearchColumns, false)
	query = r.db.Rebind(query)

	var list []*ChartOfAccount
	if err := r.db.SelectContext(ctx, &list, query, filterArgs...); err != nil {
		return nil, fmt.Errorf("list chart of accounts: %w", err)
	}

	return list, nil
}

func (r *repository) CountChartOfAccount(ctx context.Context, actorID uuid.UUID, role string, f common.Filter) (int, error) {
	// Start with the soft-delete filter immediately and JOIN with account_tax table
	base := ` FROM tbl_chart_of_accounts coa 
	          JOIN tbl_account_tax at ON at.id = coa.account_tax_id
	          WHERE coa.deleted_at IS NULL `

	if role == util.RoleAccountant {
		base += fmt.Sprintf(` AND EXISTS (
                SELECT 1 FROM tbl_invitation
                WHERE practitioner_id = coa.practitioner_id 
                AND accountant_id = '%s' 
                AND status = 'COMPLETED'
            )`, actorID.String())
	} else {
		base += fmt.Sprintf(` AND coa.practitioner_id = '%s'`, actorID.String())
	}

	query, filterArgs := common.BuildQuery(base, f, chartOfAccountColumns, coaSearchColumns, true)

	if strings.Contains(query, "ORDER BY") {
		query = strings.Split(query, "ORDER BY")[0]
	}

	query = r.db.Rebind(query)

	var count int
	if err := r.db.GetContext(ctx, &count, query, filterArgs...); err != nil {
		return 0, fmt.Errorf("count chart of accounts: %w", err)
	}

	return count, nil
}

func (r *repository) GetChartOfAccount(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID) (*ChartOfAccount, error) {
	query := `
		SELECT coa.id, coa.practitioner_id, coa.account_type_id, coa.account_tax_id, coa.code, coa.name, coa.key,
		       coa.is_system, at.is_taxable, coa.created_at, coa.updated_at, coa.deleted_at
		FROM tbl_chart_of_accounts coa
		JOIN tbl_account_tax at ON at.id = coa.account_tax_id
		WHERE coa.id = $1 AND coa.practitioner_id = $2 AND coa.deleted_at IS NULL
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

func (r *repository) GetChartOfAccountByKey(ctx context.Context, key string, practitionerID uuid.UUID) (*ChartOfAccount, error) {
	query := `
        SELECT coa.id, coa.practitioner_id, coa.account_type_id, coa.account_tax_id, coa.code, coa.name, coa.key,
               coa.is_system, at.is_taxable, coa.created_at, coa.updated_at, coa.deleted_at
        FROM tbl_chart_of_accounts coa
        JOIN tbl_account_tax at ON at.id = coa.account_tax_id
        WHERE coa.key = $1 
          AND ($2 = '00000000-0000-0000-0000-000000000000'::uuid OR coa.practitioner_id = $2)
          AND coa.deleted_at IS NULL
    `
	var c ChartOfAccount
	if err := r.db.QueryRowxContext(ctx, query, key, practitionerID).StructScan(&c); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get chart by key and practitioner_id: %w", err)
	}
	return &c, nil
}

func (r *repository) GetChartByCodeAndPractitionerID(ctx context.Context, code int16, practitionerID uuid.UUID, excludeID *uuid.UUID) (*ChartOfAccount, error) {
	query := `
		SELECT coa.id, coa.practitioner_id, coa.account_type_id, coa.account_tax_id, coa.code, coa.name, coa.key,
		       coa.is_system, at.is_taxable, coa.created_at, coa.updated_at, coa.deleted_at
		FROM tbl_chart_of_accounts coa
	 JOIN tbl_account_tax at ON at.id = coa.account_tax_id
		WHERE coa.code = $1 AND coa.practitioner_id = $2 AND coa.deleted_at IS NULL
	`
	args := []interface{}{code, practitionerID}
	if excludeID != nil {
		query += ` AND coa.id != $3`
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

func (r *repository) CreateChartOfAccount(ctx context.Context, c *ChartOfAccount, tx *sqlx.Tx) (*ChartOfAccount, error) {
	query := `
		INSERT INTO tbl_chart_of_accounts (practitioner_id, account_type_id, account_tax_id, code, name, key, is_system)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`
	var id uuid.UUID
	err := tx.QueryRowxContext(ctx, query,
		c.PractitionerID, c.AccountTypeID, c.AccountTaxID, c.Code, c.Name, c.Key, c.IsSystem,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("create chart of account: %w", err)
	}
	return r.getChartByID(ctx, tx, id)
}

// BulkCreateChartOfAccounts inserts all rows in a single query — used during practitioner onboarding.
func (r *repository) BulkCreateChartOfAccounts(ctx context.Context, rows []*ChartOfAccount, tx *sqlx.Tx) error {
	if len(rows) == 0 {
		return nil
	}

	query := `INSERT INTO tbl_chart_of_accounts (practitioner_id, account_type_id, account_tax_id, code, name, key, is_system) VALUES `
	args := make([]interface{}, 0, len(rows)*7)
	for i, row := range rows {
		if i > 0 {
			query += ", "
		}
		base := i * 7
		query += fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)", base+1, base+2, base+3, base+4, base+5, base+6, base+7)
		args = append(args, row.PractitionerID, row.AccountTypeID, row.AccountTaxID, row.Code, row.Name, row.Key, row.IsSystem)
	}

	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("bulk create chart of accounts: %w", err)
	}
	return nil
}

func (r *repository) UpdateCharOfAccount(ctx context.Context, c *ChartOfAccount) (*ChartOfAccount, error) {
	query := `
		UPDATE tbl_chart_of_accounts
		SET account_type_id = $2, account_tax_id = $3, code = $4, name = $5,  updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id
	`
	var id uuid.UUID
	fmt.Println("name--------->", c.Name)
	err := r.db.QueryRowxContext(ctx, query,
		c.ID, c.AccountTypeID, c.AccountTaxID, c.Code, c.Name,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update chart of account: %w", err)
	}
	return r.getChartByID(ctx, r.db, id)
}

// getChartByID fetches a ChartOfAccount by id, joining tbl_account_tax for is_taxable.
// querier accepts either *sqlx.DB or *sqlx.Tx.
func (r *repository) getChartByID(ctx context.Context, querier interface {
	QueryRowxContext(ctx context.Context, query string, args ...interface{}) *sqlx.Row
}, id uuid.UUID) (*ChartOfAccount, error) {
	query := `
		SELECT coa.id, coa.practitioner_id, coa.account_type_id, coa.account_tax_id, coa.code, coa.name, coa.key,
		       coa.is_system, at.is_taxable, coa.created_at, coa.updated_at, coa.deleted_at
		FROM tbl_chart_of_accounts coa
		JOIN tbl_account_tax at ON at.id = coa.account_tax_id
		WHERE coa.id = $1
	`
	var c ChartOfAccount
	if err := querier.QueryRowxContext(ctx, query, id).StructScan(&c); err != nil {
		return nil, fmt.Errorf("get chart by id: %w", err)
	}
	return &c, nil
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

// GetAccountTypeByName implements [Repository].
func (r *repository) GetAccountTypeByName(ctx context.Context, name string) (int, error) {
	query := `
		SELECT id, name, created_at, updated_at
		FROM tbl_account_type
		WHERE name = $1
	`
	var a AccountType
	if err := r.db.QueryRowxContext(ctx, query, name).StructScan(&a); err != nil {
		return 0, fmt.Errorf("get account type: %w", err)
	}
	return int(a.ID), nil
}

// GetByIDInternal ignores the practitionerID check because it's only used for internal ID resolution
func (r *repository) GetByIDInternal(ctx context.Context, id uuid.UUID) (*ChartOfAccount, error) {
	query := `
		SELECT coa.id, coa.practitioner_id, coa.account_type_id, coa.account_tax_id, coa.code, coa.name, coa.key,
		       coa.is_system, at.is_taxable, coa.created_at, coa.updated_at, coa.deleted_at
		FROM tbl_chart_of_accounts coa
		JOIN tbl_account_tax at ON at.id = coa.account_tax_id
		WHERE coa.id = $1 AND coa.deleted_at IS NULL
	`
	var c ChartOfAccount
	if err := r.db.GetContext(ctx, &c, query, id); err != nil {
		return nil, err
	}
	return &c, nil
}
