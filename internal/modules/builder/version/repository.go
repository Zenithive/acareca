package version

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
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
	ListVersionByFormID(ctx context.Context, formID uuid.UUID) (*FormVersion, error)
	DeactivateByFormID(ctx context.Context, formID uuid.UUID) error
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) IRepository {
	return &repository{db: db}
}

// Create implements [IRepository].
// If v.IsActive is true, deactivates all existing active versions for the same form_id atomically.
func (r *repository) Create(ctx context.Context, v *FormVersion) error {
	insertQuery := `
		INSERT INTO tbl_custom_form_version (id, form_id, version, is_active, practitioner_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at, updated_at
	`
	err := util.RunInTransaction(ctx, r.db, func(ctx context.Context, tx *sqlx.Tx) error {
		if v.IsActive {
			if _, err := tx.ExecContext(ctx,
				`UPDATE tbl_custom_form_version SET is_active = false WHERE form_id = $1 AND is_active = true AND deleted_at IS NULL`,
				v.FormId,
			); err != nil {
				return fmt.Errorf("deactivate existing versions: %w", err)
			}
		}
		return tx.QueryRowxContext(ctx, insertQuery,
			v.ID, v.FormId, v.Version, v.IsActive, v.PractitionerID,
		).StructScan(v)
	})
	if err != nil {
		return fmt.Errorf("create form version: %w", err)
	}
	return nil
}

// DeactivateByFormID implements [IRepository].
func (r *repository) DeactivateByFormID(ctx context.Context, formID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE tbl_custom_form_version SET is_active = false WHERE form_id = $1 AND is_active = true AND deleted_at IS NULL`,
		formID,
	)
	if err != nil {
		return fmt.Errorf("deactivate form versions: %w", err)
	}
	return nil
}

// Get implements [IRepository].
func (r *repository) Get(ctx context.Context, id uuid.UUID) (*FormVersion, error) {
	query := `SELECT id, form_id, version, is_active, practitioner_id, created_at, updated_at
		FROM tbl_custom_form_version WHERE id = $1 AND deleted_at IS NULL`
	var v FormVersion
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&v.ID, &v.FormId, &v.Version, &v.IsActive, &v.PractitionerID, &v.CreatedAt, &v.UpdatedAt,
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
		RETURNING id, form_id, version, is_active, practitioner_id, created_at, updated_at
	`
	var out FormVersion
	if err := r.db.QueryRowContext(ctx, query, v.Version, v.IsActive, v.ID).Scan(
		&out.ID, &out.FormId, &out.Version, &out.IsActive, &out.PractitionerID, &out.CreatedAt, &out.UpdatedAt,
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
	query := `SELECT id, form_id, version, is_active, practitioner_id, created_at, updated_at
		FROM tbl_custom_form_version WHERE form_id = $1 AND deleted_at IS NULL
		ORDER BY version ASC`
	var list []*FormVersion
	if err := r.db.SelectContext(ctx, &list, query, formID); err != nil {
		return nil, fmt.Errorf("list form versions: %w", err)
	}
	return list, nil
}

// ListVersionByFormID implements [IRepository].
func (r *repository) ListVersionByFormID(ctx context.Context, formID uuid.UUID) (*FormVersion, error) {
	query := `SELECT id, form_id, version, is_active, practitioner_id, created_at, updated_at
		FROM tbl_custom_form_version WHERE form_id = $1 AND deleted_at IS NULL
		ORDER BY version ASC LIMIT 1`
	var v FormVersion
	if err := r.db.QueryRowContext(ctx, query, formID).Scan(
		&v.ID, &v.FormId, &v.Version, &v.IsActive, &v.PractitionerID, &v.CreatedAt, &v.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get form version by form id: %w", err)
	}
	return &v, nil
}
