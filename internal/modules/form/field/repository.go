package field

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("form field not found")

type IRepository interface {
	Create(ctx context.Context, f *FormField) error
	GetByID(ctx context.Context, id uuid.UUID) (*FormField, error)
	Update(ctx context.Context, f *FormField) (*FormField, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListByFormVersionID(ctx context.Context, formVersionID uuid.UUID) ([]*FormField, error)
}

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) IRepository {
	return &Repository{db: db}
}

// Create implements [IRepository].
func (r *Repository) Create(ctx context.Context, f *FormField) error {
	query := `
		INSERT INTO tbl_form_field (id, form_version_id, label, section_type, payment_responsibility, tax_type, coa_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
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
	query := `SELECT id, form_version_id, label, section_type, payment_responsibility, tax_type, coa_id, created_at, updated_at
		FROM tbl_form_field WHERE id = $1 AND deleted_at IS NULL`
	var f FormField
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&f.ID, &f.FormVersionID, &f.Label, &f.SectionType, &f.PaymentResponsibility, &f.TaxType, &f.CoaID, &f.CreatedAt, &f.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get form field: %w", err)
	}
	return &f, nil
}

// Update implements [IRepository].
func (r *Repository) Update(ctx context.Context, f *FormField) (*FormField, error) {
	query := `
		UPDATE tbl_form_field
		SET label = $1, section_type = $2, payment_responsibility = $3, tax_type = $4, coa_id = $5, updated_at = now()
		WHERE id = $6 AND deleted_at IS NULL
		RETURNING id, form_version_id, label, section_type, payment_responsibility, tax_type, coa_id, created_at, updated_at
	`
	var out FormField
	if err := r.db.QueryRowContext(ctx, query,
		f.Label, f.SectionType, f.PaymentResponsibility, f.TaxType, f.CoaID, f.ID,
	).Scan(&out.ID, &out.FormVersionID, &out.Label, &out.SectionType, &out.PaymentResponsibility, &out.TaxType, &out.CoaID, &out.CreatedAt, &out.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
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
		return ErrNotFound
	}
	return nil
}

// ListByFormVersionID implements [IRepository].
func (r *Repository) ListByFormVersionID(ctx context.Context, formVersionID uuid.UUID) ([]*FormField, error) {
	query := `SELECT id, form_version_id, label, section_type, payment_responsibility, tax_type, coa_id, created_at, updated_at
		FROM tbl_form_field WHERE form_version_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC`
	var list []*FormField
	if err := r.db.SelectContext(ctx, &list, query, formVersionID); err != nil {
		return nil, fmt.Errorf("list form fields: %w", err)
	}
	return list, nil
}
