package detail

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("form not found")

type IRepository interface {
	Create(ctx context.Context, d *FormDetail) error
	Update(ctx context.Context, d *FormDetail) (*FormDetail, error)
	Delete(ctx context.Context, formID uuid.UUID) error
	GetByID(ctx context.Context, formID uuid.UUID) (*FormDetail, error)
	ListForm(ctx context.Context, filter common.Filter, practitionerID uuid.UUID) ([]*FormDetail, error)
	CountForm(ctx context.Context, filter common.Filter, practitionerID uuid.UUID) (int, error)
	CreateTx(ctx context.Context, tx *sqlx.Tx, d *FormDetail) error
	UpdateTx(ctx context.Context, tx *sqlx.Tx, d *FormDetail) (*FormDetail, error)
	DeleteTx(ctx context.Context, tx *sqlx.Tx, formID uuid.UUID) error
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
		INSERT INTO tbl_form (id, clinic_id, name, description, status, method, owner_share, clinic_share, super_component)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at
	`
	if err := r.db.QueryRowContext(ctx, query,
		d.ID, d.ClinicID, d.Name, d.Description, d.Status, d.Method, d.OwnerShare, d.ClinicShare, d.SuperComponent,
	).Scan(&d.CreatedAt, &d.UpdatedAt); err != nil {
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
func (r *Repository) ListForm(ctx context.Context, filter common.Filter, practitionerID uuid.UUID) ([]*FormDetail, error) {

	base := `
	FROM tbl_form f
	LEFT JOIN tbl_custom_form_version v ON v.form_id = f.id AND v.is_active = true AND v.deleted_at IS NULL
	WHERE f.deleted_at IS NULL
	AND f.clinic_id IN (
		SELECT id FROM tbl_clinic 
		WHERE practitioner_id = ? AND deleted_at IS NULL
	)
	`

	args := []any{practitionerID}

	allowedColumns := map[string]string{
		"status":      "f.status",
		"method":      "f.method",
		"clinic_id":   "f.clinic_id",
		"created_at":  "f.created_at",
		"clinic_name": "f.name",
	}

	searchCols := []string{
		"f.name",
		"f.description",
	}

	query, qArgs := common.BuildQuery(
		base,
		filter,
		allowedColumns,
		searchCols,
		false,
	)

	args = append(args, qArgs...)

	query = `
	SELECT f.id, f.clinic_id, f.name, f.description, f.status, f.method,
	       f.owner_share, f.clinic_share, f.super_component, v.id AS active_version_id, f.created_at, f.updated_at
	` + query

	query = r.db.Rebind(query)

	var details []*FormDetail
	if err := r.db.SelectContext(ctx, &details, query, args...); err != nil {
		return nil, fmt.Errorf("list form details: %w", err)
	}

	return details, nil
}

func (r *Repository) CountForm(ctx context.Context, filter common.Filter, practitionerID uuid.UUID) (int, error) {
	base := `
    FROM tbl_form f
    WHERE f.deleted_at IS NULL
    AND f.clinic_id IN (
        SELECT id FROM tbl_clinic 
        WHERE practitioner_id = ? AND deleted_at IS NULL
    )
    `
	args := []any{practitionerID}

	// Use the same column mappings as ListForm
	allowedColumns := map[string]string{
		"status": "f.status", "method": "f.method", "clinic_id": "f.clinic_id",
	}
	searchCols := []string{"f.name", "f.description"}

	// Pass 'true' to BuildQuery for count mode
	query, qArgs := common.BuildQuery(base, filter, allowedColumns, searchCols, true)

	args = append(args, qArgs...)
	query = r.db.Rebind(query)

	var count int
	if err := r.db.GetContext(ctx, &count, query, args...); err != nil {
		return 0, fmt.Errorf("count forms: %w", err)
	}

	return count, nil
}

// Update implements [IRepository].
func (r *Repository) Update(ctx context.Context, d *FormDetail) (*FormDetail, error) {
	query := `
		UPDATE tbl_form
		SET name = $1, description = $2, status = $3, method = $4, owner_share = $5, clinic_share = $6, super_component = $7, updated_at = now()
		WHERE id = $8 AND deleted_at IS NULL
		RETURNING id, clinic_id, name, description, status, method, owner_share, clinic_share, super_component, created_at, updated_at
	`
	var out FormDetail
	if err := r.db.QueryRowContext(ctx, query,
		d.Name, d.Description, d.Status, d.Method, d.OwnerShare, d.ClinicShare, d.SuperComponent, d.ID,
	).Scan(&out.ID, &out.ClinicID, &out.Name, &out.Description, &out.Status, &out.Method, &out.OwnerShare, &out.ClinicShare, &out.SuperComponent, &out.CreatedAt, &out.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update form detail: %w", err)
	}
	return &out, nil
}

// GetByID implements [IRepository].
func (r *Repository) GetByID(ctx context.Context, formID uuid.UUID) (*FormDetail, error) {
	query := `SELECT id, clinic_id, name, description, status, method, owner_share, clinic_share, super_component, created_at, updated_at FROM tbl_form WHERE id = $1 AND deleted_at IS NULL`
	var d FormDetail
	if err := r.db.QueryRowContext(ctx, query, formID).Scan(
		&d.ID, &d.ClinicID, &d.Name, &d.Description, &d.Status, &d.Method, &d.OwnerShare, &d.ClinicShare, &d.SuperComponent, &d.CreatedAt, &d.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get form detail by id: %w", err)
	}
	return &d, nil
}

// CreateTx creates a form detail within a transaction.
func (r *Repository) CreateTx(ctx context.Context, tx *sqlx.Tx, d *FormDetail) error {
	query := `
		INSERT INTO tbl_form (id, clinic_id, name, description, status, method, owner_share, clinic_share, super_component)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at
	`
	if err := tx.QueryRowContext(ctx, query,
		d.ID, d.ClinicID, d.Name, d.Description, d.Status, d.Method, d.OwnerShare, d.ClinicShare, d.SuperComponent,
	).Scan(&d.CreatedAt, &d.UpdatedAt); err != nil {
		return fmt.Errorf("create form detail in transaction: %w", err)
	}
	return nil
}

// UpdateTx updates a form detail within a transaction.
func (r *Repository) UpdateTx(ctx context.Context, tx *sqlx.Tx, d *FormDetail) (*FormDetail, error) {
	query := `
		UPDATE tbl_form
		SET name = $1, description = $2, status = $3, method = $4, owner_share = $5, clinic_share = $6, super_component = $7, updated_at = now()
		WHERE id = $8 AND deleted_at IS NULL
		RETURNING id, clinic_id, name, description, status, method, owner_share, clinic_share, super_component, created_at, updated_at
	`
	var out FormDetail
	if err := tx.QueryRowContext(ctx, query,
		d.Name, d.Description, d.Status, d.Method, d.OwnerShare, d.ClinicShare, d.SuperComponent, d.ID,
	).Scan(&out.ID, &out.ClinicID, &out.Name, &out.Description, &out.Status, &out.Method, &out.OwnerShare, &out.ClinicShare, &out.SuperComponent, &out.CreatedAt, &out.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update form detail in transaction: %w", err)
	}
	return &out, nil
}

// DeleteTx deletes a form detail within a transaction.
func (r *Repository) DeleteTx(ctx context.Context, tx *sqlx.Tx, formID uuid.UUID) error {
	query := `UPDATE tbl_form SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`
	res, err := tx.ExecContext(ctx, query, formID)
	if err != nil {
		return fmt.Errorf("delete form in transaction: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
