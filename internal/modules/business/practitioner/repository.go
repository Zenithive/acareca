package practitioner

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/jmoiron/sqlx"
)

var errNotFound = errors.New("practitioner not found")

type Repository interface {
	CreatePractitioner(ctx context.Context, req *RqCreatePractitioner, tx *sqlx.Tx) (*RsPractitioner, error)
	GetPractitioner(ctx context.Context, id uuid.UUID) (*RsPractitioner, error)
	DeletePractitioner(ctx context.Context, id uuid.UUID) error
	ListPractitioners(ctx context.Context, f common.Filter) ([]*PractitionerWithUser, error)
	GetPractitionerByUserID(ctx context.Context, userID string) (*RsPractitioner, error)
	CountPractitioners(ctx context.Context, f common.Filter) (int, error)

	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	UpdateABN(ctx context.Context, userID uuid.UUID, abn *string) error
	UpdateStripeCustomerID(ctx context.Context, practitionerID uuid.UUID, customerID string) error
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

// CreatePractitioner implements [Repository].
func (r *repository) CreatePractitioner(ctx context.Context, req *RqCreatePractitioner, tx *sqlx.Tx) (*RsPractitioner, error) {
	query := `
		INSERT INTO tbl_practitioner (user_id)
		VALUES ($1)
		RETURNING id, user_id, abn, verified, stripe_customer_id, created_at, updated_at, deleted_at
	`
	var p Practitioner
	if err := tx.QueryRowxContext(ctx, query, req.UserID).StructScan(&p); err != nil {
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
		SELECT p.id, p.user_id, p.abn, p.verified, p.stripe_customer_id, p.created_at, p.updated_at, p.deleted_at,
		       u.email, u.first_name, u.last_name, u.phone
		FROM tbl_practitioner p
		JOIN tbl_user u ON u.id = p.user_id AND u.deleted_at IS NULL
		WHERE p.id = $1 AND p.deleted_at IS NULL
	`
	var p PractitionerWithUser
	if err := r.db.QueryRowxContext(ctx, query, id).StructScan(&p); err != nil {
		return nil, err
	}
	return p.ToRs(), nil
}

// GetPractitionerByUserID implements [Repository].
func (r *repository) GetPractitionerByUserID(ctx context.Context, userID string) (*RsPractitioner, error) {
	query := `
	SELECT id, user_id, abn, verified, stripe_customer_id, created_at, updated_at, deleted_at FROM tbl_practitioner WHERE user_id = $1 AND deleted_at IS NULL
	`
	var p Practitioner
	if err := r.db.QueryRowxContext(ctx, query, userID).StructScan(&p); err != nil {
		return nil, err
	}
	return p.ToRs(), nil
}

var practitionerColumns = map[string]string{
	"id":         "p.id",
	"first_name": "u.first_name",
	"last_name":  "u.last_name",
	"email":      "u.email",
	"phone":      "u.phone",
	"abn":        "p.abn",
}

var practitionerSearchCols = []string{"u.first_name", "u.last_name", "u.email", "u.phone"}

// ListPractitioners implements [Repository].
func (r *repository) ListPractitioners(ctx context.Context, f common.Filter) ([]*PractitionerWithUser, error) {
	base := `
		SELECT p.id, p.user_id, p.abn, p.verified, p.stripe_customer_id, p.created_at, p.updated_at, p.deleted_at,
		       u.email, u.first_name, u.last_name, u.phone
		FROM tbl_practitioner p
		JOIN tbl_user u ON u.id = p.user_id AND u.deleted_at IS NULL
		WHERE p.deleted_at IS NULL
	`
	query, filterArgs := common.BuildQuery(base, f, practitionerColumns, practitionerSearchCols, false)

	var list []*PractitionerWithUser
	if err := r.db.SelectContext(ctx, &list, r.db.Rebind(query), filterArgs...); err != nil {
		return nil, fmt.Errorf("list practitioners repo: %w", err)
	}
	return list, nil
}

func (r *repository) CountPractitioners(ctx context.Context, f common.Filter) (int, error) {
	base := `
        FROM tbl_practitioner p
        JOIN tbl_user u ON u.id = p.user_id AND u.deleted_at IS NULL
        WHERE p.deleted_at IS NULL
    `
	query, filterArgs := common.BuildQuery(base, f, practitionerColumns, practitionerSearchCols, true)

	var count int
	if err := r.db.GetContext(ctx, &count, r.db.Rebind(query), filterArgs...); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *repository) UpdateABN(ctx context.Context, userID uuid.UUID, abn *string) error {
	query := `UPDATE tbl_practitioner SET abn = $1, updated_at = NOW() WHERE user_id = $2 AND deleted_at IS NULL`
	_, err := r.db.ExecContext(ctx, query, abn, userID)
	return err
}

func (r *repository) UpdateStripeCustomerID(ctx context.Context, practitionerID uuid.UUID, customerID string) error {
	query := `UPDATE tbl_practitioner SET stripe_customer_id = $2, updated_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, practitionerID, customerID)
	return err
}

/*
func (r *repository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	query := `UPDATE tbl_practitioner SET deleted_at = now() WHERE user_id = $1 AND deleted_at IS NULL`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}
*/

func (r *repository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	// 1. Soft Delete the Practitioner Profile
	// Note: No 'status' column here based on your schema check
	profileQuery := `
        UPDATE tbl_practitioner 
        SET 
            deleted_at = now(), 
            updated_at = now()
        WHERE user_id = $1 AND deleted_at IS NULL
    `
	if _, err := r.db.ExecContext(ctx, profileQuery, userID); err != nil {
		return fmt.Errorf("failed to soft-delete practitioner profile: %w", err)
	}

	// 2. Soft Delete and Deactivate the Subscriptions
	// Here we DO update the 'status' column
	subQuery := `
        UPDATE tbl_practitioner_subscription 
        SET 
            deleted_at = now(), 
            updated_at = now(),
            status = 'CANCELLED'
        WHERE practitioner_id IN (
            SELECT id FROM tbl_practitioner WHERE user_id = $1
        ) AND deleted_at IS NULL
    `
	if _, err := r.db.ExecContext(ctx, subQuery, userID); err != nil {
		return fmt.Errorf("failed to deactivate practitioner subscriptions: %w", err)
	}

	return nil
}
