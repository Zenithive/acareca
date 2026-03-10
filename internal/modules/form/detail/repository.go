package detail

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("form not found")

type IRepository interface {
	Create(ctx context.Context, d *FormDetail) error
	Update(ctx context.Context, d *FormDetail) (*FormDetail, error)
	Delete(ctx context.Context, formID uuid.UUID) error
	GetByID(ctx context.Context, formID uuid.UUID) (*FormDetail, error)
	ListForm(ctx context.Context, filter Filter) ([]*FormDetail, error)
}

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) IRepository {
	return &Repository{db: db}
}

// Create implements [IRepository].
func (r *Repository) Create(ctx context.Context, d *FormDetail) error {
	query := `
		INSERT INTO tbl_form (clinic_id, name, description, status, method, owner_share, clinic_share)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	if _, err := r.db.ExecContext(ctx, query,
		d.ClinicID, d.Name, d.Description, d.Status, d.Method, d.OwnerShare, d.ClinicShare,
	); err != nil {
		return fmt.Errorf("create form detail: %w", err)
	}
	return nil
}

// Delete implements [IRepository].
func (r *Repository) Delete(ctx context.Context, formID uuid.UUID) error {
	query := `UPDATE tbl_form SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`
	res, err := r.db.ExecContext(ctx, query, formID)
	if err != nil {
		return fmt.Errorf("delete form: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListForm implements [IRepository].
func (r *Repository) ListForm(ctx context.Context, filter Filter) ([]*FormDetail, error) {
	query := `SELECT id, clinic_id, name, description, status, method, owner_share, clinic_share, created_at, updated_at FROM tbl_form WHERE deleted_at IS NULL`
	var args []interface{}
	if filter.Status != nil {
		query += ` AND status = $1`
		args = append(args, *filter.Status)
	}
	if filter.Method != nil {
		query += ` AND method = $2`
		args = append(args, *filter.Method)
	}
	var details []*FormDetail
	if err := r.db.SelectContext(ctx, &details, query, args...); err != nil {
		return nil, fmt.Errorf("list form details: %w", err)
	}
	return details, nil
}

// Update implements [IRepository].
func (r *Repository) Update(ctx context.Context, d *FormDetail) (*FormDetail, error) {
	query := `
		UPDATE tbl_form
		SET name = $1, description = $2, status = $3, method = $4, owner_share = $5, clinic_share = $6, updated_at = now()
		WHERE id = $7 AND deleted_at IS NULL
		RETURNING id, clinic_id, name, description, status, method, owner_share, clinic_share, created_at, updated_at
	`
	var out FormDetail
	if err := r.db.QueryRowContext(ctx, query,
		d.Name, d.Description, d.Status, d.Method, d.OwnerShare, d.ClinicShare, d.ID,
	).Scan(&out.ID, &out.ClinicID, &out.Name, &out.Description, &out.Status, &out.Method, &out.OwnerShare, &out.ClinicShare, &out.CreatedAt, &out.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update form detail: %w", err)
	}
	return &out, nil
}

// GetByID implements [IRepository].
func (r *Repository) GetByID(ctx context.Context, formID uuid.UUID) (*FormDetail, error) {
	query := `SELECT id, clinic_id, name, description, status, method, owner_share, clinic_share, created_at, updated_at FROM tbl_form WHERE id = $1 AND deleted_at IS NULL`
	var d FormDetail
	if err := r.db.QueryRowContext(ctx, query, formID).Scan(
		&d.ID, &d.ClinicID, &d.Name, &d.Description, &d.Status, &d.Method, &d.OwnerShare, &d.ClinicShare, &d.CreatedAt, &d.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get form detail by id: %w", err)
	}
	return &d, nil
}
