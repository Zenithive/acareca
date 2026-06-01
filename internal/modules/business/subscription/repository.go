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
	ListHistoryByPractitionerID(ctx context.Context, practitionerID uuid.UUID, f common.Filter) ([]*RsActiveSubscription, error)
	Update(ctx context.Context, s *PractitionerSubscription) (*PractitionerSubscription, error)
	Delete(ctx context.Context, id int) error
	CountByPractitionerID(ctx context.Context, practitionerID uuid.UUID, f common.Filter) (int, error)
	GetActiveSubscription(ctx context.Context, practitionerID uuid.UUID) (*RsActiveSubscription, error)
	UpsertFromWebhook(ctx context.Context, s *WebhookUpsert) error
	UpdateStripeFields(ctx context.Context, stripeSubID string, invoiceID *string, status Status, endDate time.Time) error
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

var subscriptionColumns = map[string]string{
	"id":              "ps.id",
	"practitioner_id": "ps.practitioner_id",
	"subscription_id": "ps.subscription_id",
	"status":          "ps.status",
	"start_date":      "ps.start_date",
	"end_date":        "ps.end_date",
}

var subscriptionSearchCols = []string{"ps.status"}

type dbSubscriptionRow struct {
	PractitionerSubscription
	SName        string  `db:"s_name"`
	SDescription *string `db:"s_description"`
}

func (r *repository) Create(ctx context.Context, s *PractitionerSubscription, tx *sqlx.Tx) (*PractitionerSubscription, error) {
	query := `
		INSERT INTO tbl_practitioner_subscription (practitioner_id, subscription_id, start_date, end_date, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, practitioner_id, subscription_id, start_date, end_date, status, stripe_subscription_id, stripe_invoice_id, created_at, updated_at, deleted_at
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
		SELECT id, practitioner_id, subscription_id, start_date, end_date, status, stripe_subscription_id, stripe_invoice_id, created_at, updated_at, deleted_at
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

func (r *repository) ListByPractitionerID(ctx context.Context, practitionerID uuid.UUID, f common.Filter) ([]*PractitionerSubscription, error) {
	base := `
		SELECT ps.id, ps.practitioner_id, ps.subscription_id, ps.start_date, ps.end_date, ps.status, ps.stripe_subscription_id, ps.stripe_invoice_id, ps.created_at, ps.updated_at, ps.deleted_at
		FROM tbl_practitioner_subscription ps
		WHERE ps.practitioner_id = ? AND ps.deleted_at IS NULL
	`
	query, filterArgs := common.BuildQuery(base, f, subscriptionColumns, subscriptionSearchCols, false)
	args := append([]interface{}{practitionerID}, filterArgs...)

	var list []*PractitionerSubscription
	if err := r.db.SelectContext(ctx, &list, r.db.Rebind(query), args...); err != nil {
		return nil, fmt.Errorf("list practitioner subscriptions: %w", err)
	}
	return list, nil
}

func (r *repository) ListHistoryByPractitionerID(ctx context.Context, practitionerID uuid.UUID, f common.Filter) ([]*RsActiveSubscription, error) {
	base := `
        SELECT 
            ps.id, ps.practitioner_id, ps.subscription_id, ps.start_date, ps.end_date, ps.status, ps.stripe_subscription_id, ps.stripe_invoice_id, ps.created_at, ps.updated_at,
            s.name AS s_name, s.description AS s_description
        FROM tbl_practitioner_subscription ps
        INNER JOIN tbl_subscription s ON ps.subscription_id = s.id
        WHERE ps.practitioner_id = ? AND ps.deleted_at IS NULL
    `
	query, filterArgs := common.BuildQuery(base, f, subscriptionColumns, subscriptionSearchCols, false)
	args := append([]interface{}{practitionerID}, filterArgs...)

	var rows []dbSubscriptionRow
	if err := r.db.SelectContext(ctx, &rows, r.db.Rebind(query), args...); err != nil {
		return nil, fmt.Errorf("list subscription history: %w", err)
	}

	result := make([]*RsActiveSubscription, len(rows))
	for i, row := range rows {
		result[i] = r.mapToActiveSubscription(&row)
	}
	return result, nil
}

func (r *repository) Update(ctx context.Context, s *PractitionerSubscription) (*PractitionerSubscription, error) {
	query := `
		UPDATE tbl_practitioner_subscription
		SET status = $2, updated_at = $3
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, practitioner_id, subscription_id, start_date, end_date, status, stripe_subscription_id, stripe_invoice_id, created_at, updated_at, deleted_at
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
	base := `FROM tbl_practitioner_subscription ps WHERE ps.practitioner_id = ? AND ps.deleted_at IS NULL`

	query, filterArgs := common.BuildQuery(base, f, subscriptionColumns, subscriptionSearchCols, true)
	args := append([]interface{}{practitionerID}, filterArgs...)

	var count int
	if err := r.db.GetContext(ctx, &count, r.db.Rebind(query), args...); err != nil {
		return 0, fmt.Errorf("count practitioner subscriptions: %w", err)
	}
	return count, nil
}

func (r *repository) GetActiveSubscription(ctx context.Context, practitionerID uuid.UUID) (*RsActiveSubscription, error) {
	query := `
		SELECT 
			ps.id, ps.practitioner_id, ps.subscription_id, ps.start_date, ps.end_date, ps.status, ps.stripe_subscription_id, ps.stripe_invoice_id, ps.created_at, ps.updated_at,
			s.name AS s_name, s.description AS s_description
		FROM tbl_practitioner_subscription ps
		INNER JOIN tbl_subscription s ON ps.subscription_id = s.id
		WHERE ps.practitioner_id = $1 
		  AND ps.status = 'ACTIVE' 
		  AND ps.start_date <= NOW()
		  AND ps.end_date >= NOW()
		  AND ps.deleted_at IS NULL
		  AND s.deleted_at IS NULL
		ORDER BY ps.created_at DESC
		LIMIT 1
	`
	var row dbSubscriptionRow
	if err := r.db.QueryRowxContext(ctx, query, practitionerID).StructScan(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get active practitioner subscription with join: %w", err)
	}

	return r.mapToActiveSubscription(&row), nil
}

func (r *repository) UpsertFromWebhook(ctx context.Context, s *WebhookUpsert) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	deactivateQuery := `
        UPDATE tbl_practitioner_subscription
        SET status = 'INACTIVE', updated_at = $2
        WHERE practitioner_id = $1 AND status = 'ACTIVE'
    `
	now := time.Now()
	if _, err = tx.ExecContext(ctx, deactivateQuery, s.PractitionerID, now); err != nil {
		return fmt.Errorf("deactivate old plans: %w", err)
	}

	upsertQuery := `
        INSERT INTO tbl_practitioner_subscription
            (practitioner_id, subscription_id, stripe_subscription_id, stripe_invoice_id, status, start_date, end_date, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
        ON CONFLICT (stripe_subscription_id) DO UPDATE SET
            status     = EXCLUDED.status,
            end_date   = EXCLUDED.end_date,
            stripe_invoice_id = EXCLUDED.stripe_invoice_id,
            updated_at = EXCLUDED.updated_at
    `
	_, err = tx.ExecContext(ctx, upsertQuery,
		s.PractitionerID, s.SubscriptionID, s.StripeSubscriptionID,
		s.StripeInvoiceID, string(s.Status), s.StartDate, s.EndDate, now,
	)
	if err != nil {
		return fmt.Errorf("upsert new subscription: %w", err)
	}

	return tx.Commit()
}

func (r *repository) UpdateStripeFields(ctx context.Context, stripeSubID string, invoiceID *string, status Status, endDate time.Time) error {
	query := `
		UPDATE tbl_practitioner_subscription
		SET status = $2, end_date = $3, stripe_invoice_id = $4, updated_at = NOW()
		WHERE stripe_subscription_id = $1 AND deleted_at IS NULL
	`
	res, err := r.db.ExecContext(ctx, query, stripeSubID, string(status), endDate, invoiceID)
	if err != nil {
		return fmt.Errorf("update stripe fields: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *repository) mapToActiveSubscription(row *dbSubscriptionRow) *RsActiveSubscription {
	return &RsActiveSubscription{
		ID:             row.ID,
		PractitionerID: row.PractitionerID,
		StartDate:      row.StartDate,
		EndDate:        row.EndDate,
		Status:         row.Status,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Subscription: SubscriptionInfo{
			ID:          row.SubscriptionID,
			Name:        row.SName,
			Description: row.SDescription,
		},
	}
}
