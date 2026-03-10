package subscription

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("practitioner subscription not found")

type Repository interface {
	Create(ctx context.Context, s *TentantSubscription) (*TentantSubscription, error)
	GetByID(ctx context.Context, id int) (*TentantSubscription, error)
	ListByTentantID(ctx context.Context, tentantID int) ([]*TentantSubscription, error)
	Update(ctx context.Context, s *TentantSubscription) (*TentantSubscription, error)
	Delete(ctx context.Context, id int) error
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, s *TentantSubscription) (*TentantSubscription, error) {
	query := `
		INSERT INTO tbl_practitioner_subscription (tentant_id, subscription_id, start_date, end_date, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, tentant_id, subscription_id, start_date, end_date, status, created_at, updated_at, deleted_at
	`
	now := time.Now()
	var out TentantSubscription
	if err := r.db.QueryRowxContext(ctx, query,
		s.TentantID, s.SubscriptionID, s.StartDate, s.EndDate, string(s.Status), now, now,
	).StructScan(&out); err != nil {
		return nil, fmt.Errorf("create practitioner subscription: %w", err)
	}
	return &out, nil
}

func (r *repository) GetByID(ctx context.Context, id int) (*TentantSubscription, error) {
	query := `
		SELECT id, tentant_id, subscription_id, start_date, end_date, status, created_at, updated_at, deleted_at
		FROM tbl_practitioner_subscription
		WHERE id = $1 AND deleted_at IS NULL
	`
	var s TentantSubscription
	if err := r.db.QueryRowxContext(ctx, query, id).StructScan(&s); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get practitioner subscription: %w", err)
	}
	return &s, nil
}

func (r *repository) ListByTentantID(ctx context.Context, tentantID int) ([]*TentantSubscription, error) {
	query := `
		SELECT id, tentant_id, subscription_id, start_date, end_date, status, created_at, updated_at, deleted_at
		FROM tbl_practitioner_subscription
		WHERE tentant_id = $1 AND deleted_at IS NULL
		ORDER BY start_date DESC
	`
	var list []*TentantSubscription
	if err := r.db.SelectContext(ctx, &list, query, tentantID); err != nil {
		return nil, fmt.Errorf("list practitioner subscriptions: %w", err)
	}
	return list, nil
}

func (r *repository) Update(ctx context.Context, s *TentantSubscription) (*TentantSubscription, error) {
	query := `
		UPDATE tbl_practitioner_subscription
		SET status = $2, updated_at = $3
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, tentant_id, subscription_id, start_date, end_date, status, created_at, updated_at, deleted_at
	`
	var out TentantSubscription
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
