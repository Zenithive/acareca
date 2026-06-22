package coa

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type IRepo interface {
	Create(ctx context.Context, account AccountTemplate) error
	Update(ctx context.Context, account AccountTemplate) error
	Delete(ctx context.Context, id uuid.UUID, adminID uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (AccountTemplate, error)
	GetByCode(ctx context.Context, code int16) (AccountTemplate, error)
	List(ctx context.Context) ([]AccountTemplate, error)
	SeedNewTemplateToAllPractitioners(ctx context.Context, templateID uuid.UUID) error
}

type repo struct {
	db *sqlx.DB
}

func NewRepo(db *sqlx.DB) IRepo {
	return &repo{
		db: db,
	}
}

func (r *repo) Create(ctx context.Context, account AccountTemplate) error {
	const query = `
		INSERT INTO tbl_chart_of_accounts_template (
			account_type_id,
			account_tax_id,
			code,
			key,
			name,
			is_system,
			is_cos,
			is_capital,
			created_by
		)
		VALUES (
			:account_type_id,
			:account_tax_id,
			:code,
			:key,
			:name,
			:is_system,
			:is_cos,
			:is_capital,
			:created_by
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, account)
	return err
}

func (r *repo) Update(ctx context.Context, account AccountTemplate) error {
	const query = `
		UPDATE tbl_chart_of_accounts_template
		SET
			account_type_id = :account_type_id,
			account_tax_id = :account_tax_id,
			code = :code,
			key = :key,
			name = :name,
			is_system = :is_system,
			is_cos = :is_cos,
			is_capital = :is_capital,
			updated_by = :updated_by,
			updated_at = NOW()
		WHERE id = :id
		  AND deleted_at IS NULL
	`

	_, err := r.db.NamedExecContext(ctx, query, account)
	return err
}

func (r *repo) Delete(ctx context.Context, id uuid.UUID, adminID uuid.UUID) error {
	const query = `
		UPDATE tbl_chart_of_accounts_template
		SET
			deleted_at = NOW(),
			updated_by = $2,
			updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	_, err := r.db.ExecContext(ctx, query, id, adminID)
	return err
}

func (r *repo) GetByID(ctx context.Context, id uuid.UUID) (AccountTemplate, error) {
	const query = `
		SELECT
			id,
			account_type_id,
			account_tax_id,
			code,
			key,
			name,
			is_system,
			is_cos,
			is_capital,
			created_by,
			updated_by,
			created_at,
			updated_at,
			deleted_at
		FROM tbl_chart_of_accounts_template
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	var account AccountTemplate
	err := r.db.GetContext(ctx, &account, query, id)
	if err != nil {
		return AccountTemplate{}, err
	}

	return account, nil
}

func (r *repo) GetByCode(ctx context.Context, code int16) (AccountTemplate, error) {
	const query = `
		SELECT
			id,
			account_type_id,
			account_tax_id,
			code,
			key,
			name,
			is_system,
			is_cos,
			is_capital,
			created_by,
			updated_by,
			created_at,
			updated_at,
			deleted_at
		FROM tbl_chart_of_accounts_template
		WHERE code = $1
		  AND deleted_at IS NULL
	`

	var account AccountTemplate
	err := r.db.GetContext(ctx, &account, query, code)
	if err != nil {
		return AccountTemplate{}, err
	}

	return account, nil
}

func (r *repo) List(ctx context.Context) ([]AccountTemplate, error) {
	const query = `
		SELECT
			id,
			account_type_id,
			account_tax_id,
			code,
			key,
			name,
			is_system,
			is_cos,
			is_capital,
			created_by,
			updated_by,
			created_at,
			updated_at,
			deleted_at
		FROM tbl_chart_of_accounts_template
		WHERE deleted_at IS NULL
		ORDER BY code
	`

	var accounts []AccountTemplate
	err := r.db.SelectContext(ctx, &accounts, query)
	if err != nil {
		return nil, err
	}

	return accounts, nil
}

func (r *repo) SeedNewTemplateToAllPractitioners(ctx context.Context, templateID uuid.UUID) error {
	const query = `
		INSERT INTO tbl_chart_of_accounts (id, practitioner_id, template_id, is_custom, created_at)
		SELECT 
			gen_random_uuid(), 
			p.id, 
			$1, 
			false, 
			NOW()
		FROM tbl_practitioners p
		ON CONFLICT (practitioner_id, template_id) DO NOTHING
	`
	_, err := r.db.ExecContext(ctx, query, templateID)
	return err
}
