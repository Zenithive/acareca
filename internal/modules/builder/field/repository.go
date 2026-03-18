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
	Create(ctx context.Context, f *FormField) error
	GetByID(ctx context.Context, id uuid.UUID) (*FormField, error)
	Update(ctx context.Context, f *FormField) (*FormField, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListByFormVersionID(ctx context.Context, formVersionID uuid.UUID) ([]*FormField, error)
	ListRsByFormVersionID(ctx context.Context, formVersionID uuid.UUID) ([]*RsFormField, error)

	// Transaction-based variants
	CreateTx(ctx context.Context, tx *sqlx.Tx, f *FormField) error
	UpdateTx(ctx context.Context, tx *sqlx.Tx, f *FormField) (*FormField, error)
	DeleteTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error
}

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) IRepository {
	return &Repository{db: db}
}

// fieldRow is used to scan field + coa join results.
type fieldRow struct {
	ID                    uuid.UUID `db:"id"`
	FormVersionID         uuid.UUID `db:"form_version_id"`
	Label                 string    `db:"label"`
	SectionType           string    `db:"section_type"`
	PaymentResponsibility *string   `db:"payment_responsibility"`
	TaxType               *string   `db:"tax_type"`
	CoaID                 uuid.UUID `db:"coa_id"`
	CreatedAt             string    `db:"created_at"`
	UpdatedAt             string    `db:"updated_at"`
	CoaCode               *int16    `db:"coa_code"`
	CoaName               *string   `db:"coa_name"`
	CoaAccountTypeID      *int16    `db:"coa_account_type_id"`
	CoaAccountTaxID       *int16    `db:"coa_account_tax_id"`
}

func (r *fieldRow) toFormField() *FormField {
	return &FormField{
		ID:                    r.ID,
		FormVersionID:         r.FormVersionID,
		Label:                 r.Label,
		SectionType:           r.SectionType,
		PaymentResponsibility: r.PaymentResponsibility,
		TaxType:               r.TaxType,
		CoaID:                 r.CoaID,
		CreatedAt:             r.CreatedAt,
		UpdatedAt:             r.UpdatedAt,
	}
}

func (r *fieldRow) toRs() *RsFormField {
	rs := r.toFormField().ToRs()
	if r.CoaCode != nil && r.CoaName != nil {
		rs.Coa = &RsCoaDetail{
			ID:            r.CoaID,
			Code:          *r.CoaCode,
			Name:          *r.CoaName,
			AccountTypeID: *r.CoaAccountTypeID,
			AccountTaxID:  *r.CoaAccountTaxID,
		}
	}
	return rs
}

const fieldWithCoaSelect = `
	SELECT
		ff.id, ff.form_version_id, ff.label, ff.section_type,
		ff.payment_responsibility, ff.tax_type, ff.coa_id,
		ff.created_at, ff.updated_at,
		coa.code  AS coa_code,
		coa.name  AS coa_name,
		coa.account_type_id AS coa_account_type_id,
		coa.account_tax_id  AS coa_account_tax_id
	FROM tbl_form_field ff
	LEFT JOIN tbl_chart_of_accounts coa ON coa.id = ff.coa_id AND coa.deleted_at IS NULL
`

// Create implements [IRepository].
func (r *Repository) Create(ctx context.Context, f *FormField) error {
	query := `
		INSERT INTO tbl_form_field (id, form_version_id, label, section_type, payment_responsibility, tax_type, coa_id)
		VALUES ($1, $2, $3, $4::section_type, $5::payment_responsibility, $6::tax_type, $7)
		RETURNING created_at, updated_at
	`
	if err := r.db.QueryRowContext(ctx, query,
		f.ID, f.FormVersionID, f.Label, f.SectionType, f.PaymentResponsibility, f.TaxType, f.CoaID,
	).Scan(&f.CreatedAt, &f.UpdatedAt); err != nil {
		return fmt.Errorf("create form field: %w", err)
	}
	return nil
}

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

// Update implements [IRepository].
func (r *Repository) Update(ctx context.Context, f *FormField) (*FormField, error) {
	query := `
		UPDATE tbl_form_field
		SET label = $1, section_type = $2::section_type, payment_responsibility = $3::payment_responsibility, tax_type = $4::tax_type, coa_id = $5, updated_at = now()
		WHERE id = $6 AND deleted_at IS NULL
		RETURNING id, form_version_id, label, section_type, payment_responsibility, tax_type, coa_id, created_at, updated_at
	`
	var out FormField
	if err := r.db.QueryRowContext(ctx, query,
		f.Label, f.SectionType, f.PaymentResponsibility, f.TaxType, f.CoaID, f.ID,
	).Scan(&out.ID, &out.FormVersionID, &out.Label, &out.SectionType, &out.PaymentResponsibility, &out.TaxType, &out.CoaID, &out.CreatedAt, &out.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("form field not found")
		}
		return nil, fmt.Errorf("update form field: %w", err)
	}
	return &out, nil
}

// Delete implements [IRepository].
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE tbl_form_field SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete form field: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("form field not found")
	}
	return nil
}

// ListByFormVersionID implements [IRepository].
func (r *Repository) ListByFormVersionID(ctx context.Context, formVersionID uuid.UUID) ([]*FormField, error) {
	query := fieldWithCoaSelect + `WHERE ff.form_version_id = $1 AND ff.deleted_at IS NULL ORDER BY ff.created_at ASC`
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
	query := fieldWithCoaSelect + `WHERE ff.form_version_id = $1 AND ff.deleted_at IS NULL ORDER BY ff.created_at ASC`
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

// CreateTx - Transaction variant of Create
func (r *Repository) CreateTx(ctx context.Context, tx *sqlx.Tx, f *FormField) error {
	query := `
		INSERT INTO tbl_form_field (id, form_version_id, label, section_type, payment_responsibility, tax_type, coa_id)
		VALUES ($1, $2, $3, $4::section_type, $5::payment_responsibility, $6::tax_type, $7)
		RETURNING created_at, updated_at
	`
	if err := tx.QueryRowContext(ctx, query,
		f.ID, f.FormVersionID, f.Label, f.SectionType, f.PaymentResponsibility, f.TaxType, f.CoaID,
	).Scan(&f.CreatedAt, &f.UpdatedAt); err != nil {
		return fmt.Errorf("create form field tx: %w", err)
	}
	return nil
}

// UpdateTx - Transaction variant of Update
func (r *Repository) UpdateTx(ctx context.Context, tx *sqlx.Tx, f *FormField) (*FormField, error) {
	query := `
		UPDATE tbl_form_field
		SET label = $1, section_type = $2::section_type, payment_responsibility = $3::payment_responsibility, tax_type = $4::tax_type, coa_id = $5, updated_at = now()
		WHERE id = $6 AND deleted_at IS NULL
		RETURNING id, form_version_id, label, section_type, payment_responsibility, tax_type, coa_id, created_at, updated_at
	`
	var out FormField
	if err := tx.QueryRowContext(ctx, query,
		f.Label, f.SectionType, f.PaymentResponsibility, f.TaxType, f.CoaID, f.ID,
	).Scan(&out.ID, &out.FormVersionID, &out.Label, &out.SectionType, &out.PaymentResponsibility, &out.TaxType, &out.CoaID, &out.CreatedAt, &out.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("form field not found")
		}
		return nil, fmt.Errorf("update form field tx: %w", err)
	}
	return &out, nil
}

// DeleteTx - Transaction variant of Delete
func (r *Repository) DeleteTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error {
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
