package practitioner

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var errNotFound = errors.New("practitioner not found")

type Repository interface {
	CreatePractitioner(ctx context.Context, req *RqCreatePractitioner) (*RsPractitioner, error)
	GetPractitioner(ctx context.Context, id uuid.UUID) (*RsPractitioner, error)
	DeletePractitioner(ctx context.Context, id uuid.UUID) error
	ListPractitioners(ctx context.Context) ([]*RsPractitioner, error)
	GetPractitionerByUserID(ctx context.Context, userID string) (*RsPractitioner, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

// CreatePractitioner implements [Repository].
func (r *repository) CreatePractitioner(ctx context.Context, req *RqCreatePractitioner) (*RsPractitioner, error) {
	query := `
		INSERT INTO tbl_practitioner (user_id)
		VALUES ($1)
		RETURNING id, user_id, abn, verified, created_at, updated_at, deleted_at
	`
	var p Practitioner
	if err := r.db.QueryRowxContext(ctx, query, req.UserID).StructScan(&p); err != nil {
		return nil, err
	}
	return p.ToRs(), nil
}

// DeletePractitioner implements [Repository].
func (r *repository) DeletePractitioner(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE tbl_practitioner SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errNotFound
	}
	return nil
}

// GetPractitioner implements [Repository].
func (r *repository) GetPractitioner(ctx context.Context, id uuid.UUID) (*RsPractitioner, error) {
	query := `
		SELECT id, user_id, abn, verified, created_at, updated_at, deleted_at FROM tbl_practitioner WHERE id = $1 AND deleted_at IS NULL
	`
	var p Practitioner
	if err := r.db.QueryRowxContext(ctx, query, id).StructScan(&p); err != nil {
		return nil, err
	}
	return p.ToRs(), nil
}

// GetPractitionerByUserID implements [Repository].
func (r *repository) GetPractitionerByUserID(ctx context.Context, userID string) (*RsPractitioner, error) {
	query := `
		SELECT id, user_id, abn, verified, created_at, updated_at, deleted_at FROM tbl_practitioner WHERE user_id = $1 AND deleted_at IS NULL
	`
	var p Practitioner
	if err := r.db.QueryRowxContext(ctx, query, userID).StructScan(&p); err != nil {
		return nil, err
	}
	return p.ToRs(), nil
}

// ListPractitioners implements [Repository].
func (r *repository) ListPractitioners(ctx context.Context) ([]*RsPractitioner, error) {
	query := `
		SELECT id, user_id, abn, verified, created_at, updated_at, deleted_at FROM tbl_practitioner WHERE deleted_at IS NULL
	`
	var list []*Practitioner
	if err := r.db.SelectContext(ctx, &list, query); err != nil {
		return nil, err
	}
	out := make([]*RsPractitioner, 0, len(list))
	for _, p := range list {
		out = append(out, p.ToRs())
	}
	return out, nil
}
