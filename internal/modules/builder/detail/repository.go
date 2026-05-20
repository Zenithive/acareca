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
	Create(ctx context.Context, tx *sqlx.Tx, d *FormDetail) error
	Update(ctx context.Context, tx *sqlx.Tx, d *FormDetail) (*FormDetail, error)
	Delete(ctx context.Context, tx *sqlx.Tx, formID uuid.UUID) error
	GetByID(ctx context.Context, formID uuid.UUID) (*FormDetail, error)
	ListForm(ctx context.Context, filter common.Filter, actorID uuid.UUID, role string, count bool) ([]FormDetail, int, error)
	UpdateFormStatus(ctx context.Context, tx *sqlx.Tx, formID uuid.UUID, status string) error
}

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) IRepository {
	return &Repository{db: db}

}

// Create implements [IRepository].
func (r *Repository) Create(ctx context.Context, tx *sqlx.Tx, d *FormDetail) error {
	query := `
		INSERT INTO tbl_form (id, clinic_id, name, description, status, method, owner_share, clinic_share, super_component)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at
	`
	if err := tx.QueryRowContext(ctx, query,
		d.ID, d.ClinicID, d.Name, d.Description, d.Status, d.Method, d.OwnerShare, d.ClinicShare, d.SuperComponent,
	).Scan(&d.CreatedAt, &d.UpdatedAt); err != nil {
		return fmt.Errorf("create form detail: %w", err)
	}
	return nil
}

// Delete implements [IRepository].
func (r *Repository) Delete(ctx context.Context, tx *sqlx.Tx, formID uuid.UUID) error {
	query := `UPDATE tbl_form SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`
	res, err := tx.ExecContext(ctx, query, formID)
	if err != nil {
		return fmt.Errorf("delete form: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) ListForm(ctx context.Context, filter common.Filter, actorID uuid.UUID, role string, withCount bool) ([]FormDetail, int, error) {

	var permissionClause string

	if role == "ACCOUNTANT" {
		permissionClause = `
		AND (
			f.clinic_id IN (
				SELECT c.id
				FROM tbl_clinic c
				INNER JOIN tbl_invitation i
					ON i.practitioner_id = c.practitioner_id
				WHERE i.accountant_id = ?
					AND i.status = 'COMPLETED'
					AND c.deleted_at IS NULL
			)
			OR (
				f.clinic_id = '00000000-0000-0000-0000-000000000000'
				AND v.practitioner_id = ?
			)
		)`
	} else {
		permissionClause = `
		AND (
			f.clinic_id IN (
				SELECT id
				FROM tbl_clinic
				WHERE practitioner_id = ?
					AND deleted_at IS NULL
			)
			OR (
				f.clinic_id = '00000000-0000-0000-0000-000000000000'
				AND v.practitioner_id = ?
			)
		)`
	}

	base := `
	FROM tbl_form f
	LEFT JOIN tbl_custom_form_version v
		ON v.form_id = f.id
		AND v.is_active = true
		AND v.deleted_at IS NULL
	LEFT JOIN tbl_clinic c
		ON f.clinic_id = c.id
		AND c.deleted_at IS NULL
	WHERE f.deleted_at IS NULL
		AND f.method != 'EXPENSE_ENTRY'
	` + permissionClause

	args := []any{actorID, actorID}

	allowedColumns := map[string]string{
		"name":        "f.name",
		"status":      "f.status::text",
		"method":      "f.method::text",
		"clinic_ids":  "f.clinic_id",
		"created_at":  "f.created_at",
		"updated_at":  "f.updated_at",
		"clinic_name": "c.name",
	}

	searchCols := []string{
		"f.name",
		"f.description",
		"f.method::text",
		"c.name",
	}

	query, qArgs := common.BuildQuery(
		base,
		filter,
		allowedColumns,
		searchCols,
		false,
	)

	args = append(args, qArgs...)

	total := 0

	if withCount {
		countQuery := `SELECT COUNT(*) ` + base
		countQuery = r.db.Rebind(countQuery)

		if err := r.db.GetContext(
			ctx,
			&total,
			countQuery,
			actorID,
			actorID,
		); err != nil {
			return nil, 0, fmt.Errorf("count forms: %w", err)
		}
	}

	query = `
	SELECT
		f.id,
		f.clinic_id,
		f.name,
		f.description,
		f.status,
		f.method,
		f.owner_share,
		f.clinic_share,
		f.super_component,
		v.id AS active_version_id,
		f.created_at,
		f.updated_at,
		c.name AS clinic_name
	` + query

	query = r.db.Rebind(query)

	var details []FormDetail

	if err := r.db.SelectContext(
		ctx,
		&details,
		query,
		args...,
	); err != nil {
		return nil, 0, fmt.Errorf("list form details: %w", err)
	}

	if !withCount {
		total = len(details)
	}

	return details, total, nil
}

// Update implements [IRepository].
func (r *Repository) Update(ctx context.Context, tx *sqlx.Tx, d *FormDetail) (*FormDetail, error) {
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

// UpdateFormStatus implements [IRepository].
func (r *Repository) UpdateFormStatus(ctx context.Context, tx *sqlx.Tx, formID uuid.UUID, status string) error {
	query := `UPDATE tbl_form SET status = $1, updated_at = now() WHERE id = $2 AND deleted_at IS NULL`
	res, err := tx.ExecContext(ctx, query, status, formID)
	if err != nil {
		return fmt.Errorf("update form status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
