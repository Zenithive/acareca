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
	ListChartOfAccount(ctx context.Context, actorID *uuid.UUID, role string, f common.Filter) ([]*ChartOfAccount, error)
	CountChartOfAccount(ctx context.Context, actorID *uuid.UUID, role string, f common.Filter) (int, error)
	GetChartOfAccount(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID) (*ChartOfAccount, error)
	GetChartOfAccountByKey(ctx context.Context, key string, practitionerID uuid.UUID) (*ChartOfAccount, error)
	GetChartByCodeAndPractitionerID(ctx context.Context, code int16, practitionerID uuid.UUID, excludeID *uuid.UUID) (*ChartOfAccount, error)
	CreateChartOfAccount(ctx context.Context, c *ChartOfAccount, tx *sqlx.Tx) (*ChartOfAccount, error)
	BulkCreateChartOfAccounts(ctx context.Context, rows []*ChartOfAccount, tx *sqlx.Tx) error
	UpdateCharOfAccount(ctx context.Context, c *ChartOfAccount) (*ChartOfAccount, error)
	DeleteChartOfAccount(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID) error
	GetByIDInternal(ctx context.Context, id uuid.UUID) (*ChartOfAccount, error)
	ListTemplates(ctx context.Context) ([]COATemplate, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

var chartOfAccountColumns = map[string]string{
	"practitioner_id": "coa.practitioner_id",
	"id":              "coa.id",
	"account_type_id": "COALESCE(coa.account_type_id, tpl.account_type_id)",
	"account_tax_id":  "COALESCE(coa.account_tax_id, tpl.account_tax_id)",
	"code":            "COALESCE(coa.code, tpl.code)",
	"name":            "COALESCE(coa.name, tpl.name)",
	"key":             "COALESCE(coa.key, tpl.key)",
	"is_system":       "COALESCE(coa.is_system, tpl.is_system)",
	"created_at":      "coa.created_at",
}

var coaSearchColumns = []string{"COALESCE(coa.name, tpl.name)", "CAST(COALESCE(coa.code, tpl.code) AS TEXT)"}

const coaBaseSelectQuery = `
	SELECT 
		coa.id, 
		coa.practitioner_id, 
		coa.template_id, 
		coa.is_custom, 
		COALESCE(coa.key, tpl.key, '') AS "key", 
		coa.created_at, 
		coa.updated_at, 
		coa.deleted_at,
		COALESCE(coa.account_type_id, tpl.account_type_id, 0) AS account_type_id, 
		COALESCE(coa.account_tax_id, tpl.account_tax_id, 0) AS account_tax_id, 
		COALESCE(coa.code, tpl.code, 0) AS code, 
		COALESCE(coa.name, tpl.name, '') AS name, 
		COALESCE(coa.is_system, tpl.is_system, false) AS is_system, 
		COALESCE(coa.is_cos, tpl.is_cos, false) AS is_cos, 
		COALESCE(coa.is_capital, tpl.is_capital, false) AS is_capital,
		COALESCE(atyp.name, '') AS account_type_name,
		COALESCE(tax.is_taxable, false) AS is_taxable, 
		tpl.id AS template_uuid,
		COALESCE(tpl.account_type_id, 0) AS template_account_type_id,
		COALESCE(tpl.account_tax_id, 0) AS template_account_tax_id,
		COALESCE(tpl.code, 0) AS template_code,
		COALESCE(tpl.name, '') AS template_name,
		COALESCE(tpl.is_system, false) AS template_is_system,
		COALESCE(tpl.is_cos, false) AS template_is_cos,
		COALESCE(tpl.is_capital, false) AS template_is_capital
	FROM tbl_chart_of_accounts coa
	LEFT JOIN tbl_chart_of_accounts_template tpl ON tpl.id = coa.template_id
	LEFT JOIN tbl_account_type atyp ON atyp.id = COALESCE(coa.account_type_id, tpl.account_type_id)
	LEFT JOIN tbl_account_tax tax ON tax.id = COALESCE(coa.account_tax_id, tpl.account_tax_id)
`

const coaTemplateSoftDeleteFilter = ` AND coa.deleted_at IS NULL AND (coa.is_custom = true OR tpl.deleted_at IS NULL)`

func (r *repository) ListAccountTypes(ctx context.Context, f common.Filter) ([]*AccountType, error) {
	base := `SELECT id, name, created_at, updated_at FROM tbl_account_type WHERE 1=1`
	query, filterArgs := common.BuildQuery(base, f, map[string]string{"id": "id", "name": "name"}, []string{"name"}, false)

	var list []*AccountType
	if err := r.db.SelectContext(ctx, &list, r.db.Rebind(query), filterArgs...); err != nil {
		return nil, fmt.Errorf("list account types: %w", err)
	}
	return list, nil
}

func (r *repository) ListAccountTaxes(ctx context.Context, f common.Filter) ([]*AccountTax, error) {
	base := `SELECT id, name, rate, is_taxable, created_at, updated_at FROM tbl_account_tax WHERE 1=1`
	query, filterArgs := common.BuildQuery(base, f, map[string]string{"id": "id", "name": "name", "rate": "rate"}, []string{"name", "is_taxable"}, false)

	var list []*AccountTax
	if err := r.db.SelectContext(ctx, &list, r.db.Rebind(query), filterArgs...); err != nil {
		return nil, fmt.Errorf("list account taxes: %w", err)
	}
	return list, nil
}

func (r *repository) GetAccountType(ctx context.Context, id int16) (*AccountType, error) {
	query := `SELECT id, name, created_at, updated_at FROM tbl_account_type WHERE id = $1`
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
	query := `SELECT id, name, rate, is_taxable, created_at, updated_at FROM tbl_account_tax WHERE id = $1`
	var a AccountTax
	if err := r.db.QueryRowxContext(ctx, query, id).StructScan(&a); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get account tax: %w", err)
	}
	return &a, nil
}

func (r *repository) ListChartOfAccount(ctx context.Context, actorID *uuid.UUID, role string, f common.Filter) ([]*ChartOfAccount, error) {
	base := coaBaseSelectQuery + ` WHERE 1=1` + coaTemplateSoftDeleteFilter
	var baseArgs []interface{}

	if role == util.RoleAccountant {
		base += ` AND EXISTS (
				SELECT 1 FROM tbl_invitation inv 
				WHERE inv.practitioner_id = coa.practitioner_id 
				AND inv.accountant_id = ? 
				AND inv.status = 'COMPLETED'
			)`
	} else {
		base += ` AND coa.practitioner_id = ?`
	}
	baseArgs = append(baseArgs, actorID)

	query, filterArgs := common.BuildQuery(base, f, chartOfAccountColumns, coaSearchColumns, false)
	finalArgs := append(baseArgs, filterArgs...)

	var list []*ChartOfAccount
	if err := r.db.SelectContext(ctx, &list, r.db.Rebind(query), finalArgs...); err != nil {
		return nil, fmt.Errorf("list chart of accounts: %w", err)
	}
	return list, nil
}

func (r *repository) CountChartOfAccount(ctx context.Context, actorID *uuid.UUID, role string, f common.Filter) (int, error) {
	base := ` 
		FROM tbl_chart_of_accounts coa
		LEFT JOIN tbl_chart_of_accounts_template tpl ON tpl.id = coa.template_id
		LEFT JOIN tbl_account_type atyp ON atyp.id = COALESCE(coa.account_type_id, tpl.account_type_id)
		LEFT JOIN tbl_account_tax tax ON tax.id = COALESCE(coa.account_tax_id, tpl.account_tax_id)
		WHERE 1=1 ` + coaTemplateSoftDeleteFilter
	var baseArgs []interface{}

	if role == util.RoleAccountant {
		base += ` AND EXISTS (
				SELECT 1 FROM tbl_invitation
				WHERE practitioner_id = coa.practitioner_id 
				AND accountant_id = ? 
				AND status = 'COMPLETED'
			)`
	} else {
		base += ` AND coa.practitioner_id = ?`
	}
	baseArgs = append(baseArgs, actorID)

	query, filterArgs := common.BuildQuery(base, f, chartOfAccountColumns, coaSearchColumns, true)
	if idx := strings.Index(query, "ORDER BY"); idx != -1 {
		query = query[:idx]
	}

	finalArgs := append(baseArgs, filterArgs...)
	var count int
	if err := r.db.GetContext(ctx, &count, r.db.Rebind(query), finalArgs...); err != nil {
		return 0, fmt.Errorf("count chart of accounts: %w", err)
	}
	return count, nil
}

func (r *repository) GetChartOfAccount(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID) (*ChartOfAccount, error) {
	query := coaBaseSelectQuery + ` WHERE coa.id = $1 AND coa.practitioner_id = $2` + coaTemplateSoftDeleteFilter
	return r.scanChart(r.db.QueryRowxContext(ctx, query, id, practitionerID))
}

func (r *repository) GetChartByCodeAndPractitionerID(ctx context.Context, code int16, practitionerID uuid.UUID, excludeID *uuid.UUID) (*ChartOfAccount, error) {
	query := coaBaseSelectQuery + ` WHERE COALESCE(coa.code, tpl.code) = $1 AND coa.practitioner_id = $2` + coaTemplateSoftDeleteFilter
	args := []interface{}{code, practitionerID}
	if excludeID != nil {
		query += ` AND coa.id != $3`
		args = append(args, *excludeID)
	}
	query += ` LIMIT 1`
	return r.scanChart(r.db.QueryRowxContext(ctx, query, args...))
}

func (r *repository) GetChartOfAccountByKey(ctx context.Context, key string, practitionerID uuid.UUID) (*ChartOfAccount, error) {
	query := coaBaseSelectQuery + `
        WHERE COALESCE(coa.key, tpl.key) = $1 
          AND ($2 = '00000000-0000-0000-0000-000000000000'::uuid OR coa.practitioner_id = $2)
          AND coa.deleted_at IS NULL
    `
	return r.scanChart(r.db.QueryRowxContext(ctx, query, key, practitionerID))
}

func (r *repository) CreateChartOfAccount(ctx context.Context, c *ChartOfAccount, tx *sqlx.Tx) (*ChartOfAccount, error) {
	query := `
		INSERT INTO tbl_chart_of_accounts (practitioner_id, account_type_id, account_tax_id, code, name, key, is_system, is_cos, is_capital, is_custom)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`
	var id uuid.UUID
	err := tx.QueryRowxContext(ctx, query,
		c.PractitionerID, c.AccountTypeID, c.AccountTaxID, c.Code, c.Name, c.Key, c.IsSystem, c.IsCos, c.IsCapital, c.IsCustom,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("create chart of account: %w", err)
	}
	return r.getChartByID(ctx, tx, id)
}

func (r *repository) BulkCreateChartOfAccounts(ctx context.Context, rows []*ChartOfAccount, tx *sqlx.Tx) error {
	if len(rows) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("INSERT INTO tbl_chart_of_accounts (practitioner_id, template_id) VALUES ")
	args := make([]interface{}, 0, len(rows)*2)

	for i, row := range rows {
		if i > 0 {
			sb.WriteString(", ")
		}
		base := i * 2
		fmt.Fprintf(&sb, "($%d, $%d)", base+1, base+2)
		args = append(args, row.PractitionerID, row.TemplateID)
	}

	sb.WriteString(" ON CONFLICT (practitioner_id, template_id) DO NOTHING")

	if _, err := tx.ExecContext(ctx, sb.String(), args...); err != nil {
		return fmt.Errorf("bulk create template-linked chart of accounts: %w", err)
	}
	return nil
}

func (r *repository) UpdateCharOfAccount(ctx context.Context, c *ChartOfAccount) (*ChartOfAccount, error) {
	query := `
		UPDATE tbl_chart_of_accounts coa
		SET account_type_id = $2, account_tax_id = $3, code = $4, name = $5, key = $6, is_system = $7, is_cos = $8, is_capital = $9, is_custom = $10, template_id = $11, updated_at = now()
		FROM tbl_chart_of_accounts_template tpl
		WHERE coa.id = $1 AND coa.template_id = tpl.id` + coaTemplateSoftDeleteFilter + `
		RETURNING coa.id
	`
	var id uuid.UUID
	err := r.db.QueryRowxContext(ctx, query, c.ID, c.AccountTypeID, c.AccountTaxID, c.Code, c.Name, c.Key, c.IsSystem, c.IsCos, c.IsCapital, c.IsCustom, c.TemplateID).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update chart of account: %w", err)
	}
	return r.getChartByID(ctx, r.db, id)
}

func (r *repository) DeleteChartOfAccount(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID) error {
	query := `
		UPDATE tbl_chart_of_accounts coa 
		SET deleted_at = now(), updated_at = now() 
		FROM tbl_chart_of_accounts_template tpl
		WHERE coa.id = $1 AND coa.practitioner_id = $2 AND coa.template_id = tpl.id` + coaTemplateSoftDeleteFilter
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

func (r *repository) GetAccountTypeByName(ctx context.Context, name string) (int, error) {
	query := `SELECT id FROM tbl_account_type WHERE name = $1`
	var id int16
	if err := r.db.GetContext(ctx, &id, query, name); err != nil {
		return 0, fmt.Errorf("get account type by name: %w", err)
	}
	return int(id), nil
}

func (r *repository) GetByIDInternal(ctx context.Context, id uuid.UUID) (*ChartOfAccount, error) {
	query := coaBaseSelectQuery + ` WHERE coa.id = $1` + coaTemplateSoftDeleteFilter
	return r.scanChart(r.db.QueryRowxContext(ctx, query, id))
}

func (r *repository) getChartByID(ctx context.Context, querier interface {
	QueryRowxContext(ctx context.Context, query string, args ...interface{}) *sqlx.Row
}, id uuid.UUID) (*ChartOfAccount, error) {
	query := coaBaseSelectQuery + ` WHERE coa.id = $1` + coaTemplateSoftDeleteFilter
	return r.scanChart(querier.QueryRowxContext(ctx, query, id))
}

func (r *repository) scanChart(row *sqlx.Row) (*ChartOfAccount, error) {
	var c ChartOfAccount
	if err := row.StructScan(&c); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *repository) ListTemplates(ctx context.Context) ([]COATemplate, error) {
	const q = `SELECT id, account_type_id, account_tax_id, code, name, key, is_system, is_cos, is_capital FROM tbl_chart_of_accounts_template WHERE deleted_at IS NULL`
	var items []COATemplate
	if err := r.db.SelectContext(ctx, &items, q); err != nil {
		return nil, err
	}
	return items, nil
}
