package version

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("form version not found")
var ErrForbidden = errors.New("form does not belong to clinic")

type IRepository interface {
	Create(ctx context.Context, v *FormVersion) error
	Get(ctx context.Context, id uuid.UUID) (*FormVersion, error)
	Update(ctx context.Context, v *FormVersion) (*FormVersion, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListByFormID(ctx context.Context, formID uuid.UUID) ([]*FormVersion, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) IRepository {
	return &repository{db: db}
}

// Create implements [IRepository].
func (r *repository) Create(ctx context.Context, v *FormVersion) error {
	query := `
		INSERT INTO tbl_custom_form_version (id, form_id, version, is_active, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at, updated_at
	`
	if err := r.db.QueryRowContext(ctx, query,
		v.ID, v.FormId, v.Version, v.IsActive, v.CreatedBy,
	).Scan(&v.CreatedAt, &v.UpdatedAt); err != nil {
		return fmt.Errorf("create form version: %w", err)
	}
	return nil
}

// Get implements [IRepository].
func (r *repository) Get(ctx context.Context, id uuid.UUID) (*FormVersion, error) {
	query := `SELECT id, form_id, version, is_active, created_by, created_at, updated_at
		FROM tbl_custom_form_version WHERE id = $1 AND deleted_at IS NULL`
	var v FormVersion
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&v.ID, &v.FormId, &v.Version, &v.IsActive, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get form version: %w", err)
	}
	return &v, nil
}

// Update implements [IRepository].
func (r *repository) Update(ctx context.Context, v *FormVersion) (*FormVersion, error) {
	query := `
		UPDATE tbl_custom_form_version
		SET version = $1, is_active = $2, updated_at = now()
		WHERE id = $3 AND deleted_at IS NULL
		RETURNING id, form_id, version, is_active, created_by, created_at, updated_at
	`
	var out FormVersion
	if err := r.db.QueryRowContext(ctx, query, v.Version, v.IsActive, v.ID).Scan(
		&out.ID, &out.FormId, &out.Version, &out.IsActive, &out.CreatedBy, &out.CreatedAt, &out.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update form version: %w", err)
	}
	return &out, nil
}

// Delete implements [IRepository].
func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE tbl_custom_form_version SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete form version: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByFormID implements [IRepository].
func (r *repository) ListByFormID(ctx context.Context, formID uuid.UUID) ([]*FormVersion, error) {
	query := `SELECT id, form_id, version, is_active, created_by, created_at, updated_at
		FROM tbl_custom_form_version WHERE form_id = $1 AND deleted_at IS NULL
		ORDER BY version ASC`
	var list []*FormVersion
	if err := r.db.SelectContext(ctx, &list, query, formID); err != nil {
		return nil, fmt.Errorf("list form versions: %w", err)
	}
	return list, nil
}
