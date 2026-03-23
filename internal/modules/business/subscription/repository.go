package subscription

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("practitioner subscription not found")

type Repository interface {
	Create(ctx context.Context, s *PractitionerSubscription, tx *sqlx.Tx) (*PractitionerSubscription, error)
	GetByID(ctx context.Context, id int) (*PractitionerSubscription, error)
	ListByPractitionerID(ctx context.Context, practitionerID uuid.UUID, f common.Filter) ([]*PractitionerSubscription, error)
	Update(ctx context.Context, s *PractitionerSubscription) (*PractitionerSubscription, error)
	Delete(ctx context.Context, id int) error
	CountByPractitionerID(ctx context.Context, practitionerID uuid.UUID, f common.Filter) (int, error)

	GetActiveSubscription(ctx context.Context, practitionerID uuid.UUID) (*PractitionerSubscription, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, s *PractitionerSubscription, tx *sqlx.Tx) (*PractitionerSubscription, error) {
	query := `
		INSERT INTO tbl_practitioner_subscription (practitioner_id, subscription_id, start_date, end_date, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, practitioner_id, subscription_id, start_date, end_date, status, created_at, updated_at, deleted_at
	`
	now := time.Now()
	var out PractitionerSubscription

	if err := tx.QueryRowxContext(ctx, query,
		s.PractitionerID, s.SubscriptionID, s.StartDate, s.EndDate, string(s.Status), now, now,
	).StructScan(&out); err != nil {
		return nil, fmt.Errorf("create practitioner subscription: %w", err)
	}
	return &out, nil
}

func (r *repository) GetByID(ctx context.Context, id int) (*PractitionerSubscription, error) {
	query := `
		SELECT id, practitioner_id, subscription_id, start_date, end_date, status, created_at, updated_at, deleted_at
		FROM tbl_practitioner_subscription
		WHERE id = $1 AND deleted_at IS NULL
	`
	var s PractitionerSubscription
	if err := r.db.QueryRowxContext(ctx, query, id).StructScan(&s); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get practitioner subscription: %w", err)
	}
	return &s, nil
}

var subscriptionColumns = map[string]string{
	"id":              "id",
	"practitioner_id": "practitioner_id",
	"subscription_id": "subscription_id",
	"status":          "status",
	"start_date":      "start_date",
	"end_date":        "end_date",
}

var subscriptionSearchCols = []string{"status"}

func (r *repository) ListByPractitionerID(ctx context.Context, practitionerID uuid.UUID, f common.Filter) ([]*PractitionerSubscription, error) {
	base := `
		SELECT id, practitioner_id, subscription_id, start_date, end_date, status, created_at, updated_at, deleted_at
		FROM tbl_practitioner_subscription
		WHERE practitioner_id = ? AND deleted_at IS NULL
	`
	query, filterArgs := common.BuildQuery(base, f, subscriptionColumns, subscriptionSearchCols, false)
	args := append([]interface{}{practitionerID}, filterArgs...)

	var list []*PractitionerSubscription
	if err := r.db.SelectContext(ctx, &list, r.db.Rebind(query), args...); err != nil {
		return nil, fmt.Errorf("list practitioner subscriptions: %w", err)
	}
	return list, nil
}

func (r *repository) Update(ctx context.Context, s *PractitionerSubscription) (*PractitionerSubscription, error) {
	query := `
		UPDATE tbl_practitioner_subscription
		SET status = $2, updated_at = $3
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, practitioner_id, subscription_id, start_date, end_date, status, created_at, updated_at, deleted_at
	`
	var out PractitionerSubscription
	if err := r.db.QueryRowxContext(ctx, query, s.ID, string(s.Status), s.UpdatedAt).StructScan(&out); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update practitioner subscription: %w", err)
	}
	return &out, nil
}

func (r *repository) Delete(ctx context.Context, id int) error {
	query := `UPDATE tbl_practitioner_subscription SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete practitioner subscription: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *repository) CountByPractitionerID(ctx context.Context, practitionerID uuid.UUID, f common.Filter) (int, error) {
	base := `FROM tbl_practitioner_subscription WHERE practitioner_id = $1 AND deleted_at IS NULL`

	query, filterArgs := common.BuildQuery(base, f, subscriptionColumns, subscriptionSearchCols, true)
	args := append([]interface{}{practitionerID}, filterArgs...)

	var count int
	if err := r.db.GetContext(ctx, &count, r.db.Rebind(query), args...); err != nil {
		return 0, fmt.Errorf("count practitioner subscriptions: %w", err)
	}
	return count, nil
}

func (r *repository) GetActiveSubscription(ctx context.Context, practitionerID uuid.UUID) (*PractitionerSubscription, error) {
	query := `
		SELECT ps.id, ps.practitioner_id, ps.subscription_id, ps.start_date, ps.end_date, ps.status, ps.created_at, ps.updated_at, ps.deleted_at
		FROM tbl_practitioner_subscription ps
		WHERE ps.practitioner_id = $1 AND ps.status = 'ACTIVE' AND ps.deleted_at IS NULL
		ORDER BY ps.created_at DESC
		LIMIT 1
	`

	// Debug: Log the query and practitioner ID
	fmt.Printf("DEBUG: Querying for practitioner_id: %s\n", practitionerID.String())

	var s PractitionerSubscription
	if err := r.db.QueryRowxContext(ctx, query, practitionerID).StructScan(&s); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fmt.Printf("DEBUG: No active subscription found for practitioner: %s\n", practitionerID.String())
			return nil, ErrNotFound
		}
		fmt.Printf("DEBUG: Database error: %v\n", err)
		return nil, fmt.Errorf("get active practitioner subscription: %w", err)
	}

	fmt.Printf("DEBUG: Found subscription ID: %d for practitioner: %s\n", s.ID, practitionerID.String())
	return &s, nil
}
