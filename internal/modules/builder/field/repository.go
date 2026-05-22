package field

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type IRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*FormField, error)
	ListByFormVersionID(ctx context.Context, formVersionID uuid.UUID) ([]*FormField, error)
	ListRsByFormVersionID(ctx context.Context, formVersionID uuid.UUID) ([]*RsFormField, error)

	// Transaction-based variants
	Create(ctx context.Context, tx *sqlx.Tx, f *FormField) error
	Update(ctx context.Context, tx *sqlx.Tx, f *FormField) (*FormField, error)
	Delete(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error
}

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) IRepository {
	return &Repository{db: db}
}

const fieldWithCoaSelect = `
	SELECT
		ff.id, ff.form_version_id, ff.field_key, ff.slug, ff.label, ff.is_computed,
		ff.section_type, ff.payment_responsibility, ff.tax_type, ff.coa_id,
		ff.sort_order, ff.created_at, ff.updated_at, ff.is_formula,
		ff.is_highlighted, ff.business_use, ff.amount,
		coa.code  AS coa_code,
		coa.name  AS coa_name,
		coa.account_type_id AS coa_account_type_id,
		coa.account_tax_id  AS coa_account_tax_id
	FROM tbl_form_field ff
	LEFT JOIN tbl_chart_of_accounts coa ON coa.id = ff.coa_id AND coa.deleted_at IS NULL
`

// GetByID implements [IRepository].
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*FormField, error) {
	query := fieldWithCoaSelect + `WHERE ff.id = $1 AND ff.deleted_at IS NULL`
	var row fieldRow
	if err := r.db.QueryRowxContext(ctx, query, id).StructScan(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("form field not found")
		}
		return nil, fmt.Errorf("get form field: %w", err)
	}
	return row.toFormField(), nil
}

// ListByFormVersionID implements [IRepository].
func (r *Repository) ListByFormVersionID(ctx context.Context, formVersionID uuid.UUID) ([]*FormField, error) {
	query := fieldWithCoaSelect + `WHERE ff.form_version_id = $1 AND ff.deleted_at IS NULL ORDER BY ff.sort_order ASC, ff.created_at ASC`
	var rows []fieldRow
	if err := r.db.SelectContext(ctx, &rows, query, formVersionID); err != nil {
		return nil, fmt.Errorf("list form fields: %w", err)
	}
	list := make([]*FormField, 0, len(rows))
	for i := range rows {
		list = append(list, rows[i].toFormField())
	}
	return list, nil
}

// ListRsByFormVersionID implements [IRepository] — returns fields with COA detail populated.
func (r *Repository) ListRsByFormVersionID(ctx context.Context, formVersionID uuid.UUID) ([]*RsFormField, error) {
	query := fieldWithCoaSelect + `WHERE ff.form_version_id = $1 AND ff.deleted_at IS NULL ORDER BY ff.sort_order ASC, ff.created_at ASC`
	var rows []fieldRow
	if err := r.db.SelectContext(ctx, &rows, query, formVersionID); err != nil {
		return nil, fmt.Errorf("list form fields with coa: %w", err)
	}
	list := make([]*RsFormField, 0, len(rows))
	for i := range rows {
		list = append(list, rows[i].toRs())
	}
	return list, nil
}

// Create - Transaction variant of Create
func (r *Repository) Create(ctx context.Context, tx *sqlx.Tx, f *FormField) error {
	query := `
		INSERT INTO tbl_form_field (id, form_version_id, field_key, slug, label, is_computed, section_type, payment_responsibility, tax_type, coa_id, sort_order, is_formula, is_highlighted, business_use, amount)
		VALUES ($1, $2, $3, $4, $5, $6, $7::section_type, $8::payment_responsibility, $9::tax_type, $10, $11, $12, $13, $14, $15)
		RETURNING created_at, updated_at
	`
	if err := tx.QueryRowContext(ctx, query,
		f.ID, f.FormVersionID, f.FieldKey, f.Slug, f.Label, f.IsComputed, f.SectionType, f.PaymentResponsibility, f.TaxType, f.CoaID, f.SortOrder, f.IsFormula, f.IsHighlighted, f.BusinessUse, f.Amount,
	).Scan(&f.CreatedAt, &f.UpdatedAt); err != nil {
		return fmt.Errorf("create form field tx: %w", err)
	}
	return nil
}

// Update - Transaction variant of Update
func (r *Repository) Update(ctx context.Context, tx *sqlx.Tx, f *FormField) (*FormField, error) {
	query := `
		UPDATE tbl_form_field
		SET label = $1, section_type = $2::section_type, payment_responsibility = $3::payment_responsibility, tax_type =NULLIF($4, '')::tax_type, coa_id = $5, sort_order = $6, is_formula = $7, is_highlighted = $8, business_use = $9, amount = $10, updated_at = now()
		WHERE id = $11 AND deleted_at IS NULL
		RETURNING id, form_version_id, field_key, slug, label, is_computed, section_type, payment_responsibility, tax_type, coa_id, sort_order, created_at, updated_at, is_formula, is_highlighted, business_use, amount
	`
	var row fieldRow
	if err := tx.QueryRowxContext(ctx, query,
		f.Label, f.SectionType, f.PaymentResponsibility, f.TaxType, f.CoaID, f.SortOrder, f.IsFormula, f.IsHighlighted, f.BusinessUse, f.Amount, f.ID,
	).StructScan(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("form field not found")
		}
		return nil, fmt.Errorf("update form field tx: %w", err)
	}
	return row.toFormField(), nil
}

// Delete - Transaction variant of Delete
func (r *Repository) Delete(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error {
	query := `UPDATE tbl_form_field SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`
	res, err := tx.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete form field tx: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("form field not found")
	}
	return nil
}
