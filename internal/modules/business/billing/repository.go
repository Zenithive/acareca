package billing

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

type Repository interface {
	GetPractitionerWithStripe(ctx context.Context, practitionerID uuid.UUID) (*PractitionerStripe, error)
	GetSubscriptionWithStripe(ctx context.Context, subscriptionID int) (*SubscriptionStripe, error)
	ListBillingHistory(ctx context.Context, f common.Filter) ([]*RsBillingHistoryRow, error)
	CountBillingHistory(ctx context.Context, f common.Filter) (int, error)
	ListActiveSubscriptions(ctx context.Context) ([]*RsSubscriptionPlan, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) GetPractitionerWithStripe(ctx context.Context, practitionerID uuid.UUID) (*PractitionerStripe, error) {
	const query = `
		SELECT p.id, u.email, u.first_name, u.last_name, p.stripe_customer_id
		FROM tbl_practitioner p
		JOIN tbl_user u ON u.id = p.user_id AND u.deleted_at IS NULL
		WHERE p.id = $1 AND p.deleted_at IS NULL
	`
	var row struct {
		ID               uuid.UUID `db:"id"`
		Email            string    `db:"email"`
		FirstName        string    `db:"first_name"`
		LastName         string    `db:"last_name"`
		StripeCustomerID *string   `db:"stripe_customer_id"`
	}
	if err := r.db.QueryRowxContext(ctx, query, practitionerID).StructScan(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("practitioner not found")
		}
		return nil, fmt.Errorf("get practitioner with stripe: %w", err)
	}
	return &PractitionerStripe{
		ID:               row.ID,
		Email:            row.Email,
		FirstName:        row.FirstName,
		LastName:         row.LastName,
		StripeCustomerID: row.StripeCustomerID,
	}, nil
}

func (r *repository) GetSubscriptionWithStripe(ctx context.Context, subscriptionID int) (*SubscriptionStripe, error) {
	const query = `
		SELECT id, name, price, stripe_price_id
		FROM tbl_subscription
		WHERE id = $1 AND deleted_at IS NULL
	`
	var row struct {
		ID            int     `db:"id"`
		Name          string  `db:"name"`
		Price         float64 `db:"price"`
		StripePriceID *string `db:"stripe_price_id"`
	}
	if err := r.db.QueryRowxContext(ctx, query, subscriptionID).StructScan(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("subscription not found")
		}
		return nil, fmt.Errorf("get subscription with stripe: %w", err)
	}
	return &SubscriptionStripe{
		ID:            row.ID,
		Name:          row.Name,
		Price:         row.Price,
		StripePriceID: row.StripePriceID,
	}, nil
}

var billingHistoryColumns = map[string]string{
	"id":              "ps.id",
	"practitioner_id": "ps.practitioner_id",
	"status":          "ps.status",
	"start_date":      "ps.start_date",
	"end_date":        "ps.end_date",
	"created_at":      "ps.created_at",
}

func (r *repository) ListBillingHistory(ctx context.Context, f common.Filter) ([]*RsBillingHistoryRow, error) {
	base := `
		SELECT
			ps.id, ps.practitioner_id,
			u.email AS practitioner_email,
			u.first_name || ' ' || u.last_name AS practitioner_name,
			s.name AS subscription_name,
			ps.status, ps.start_date, ps.end_date,
			ps.stripe_subscription_id, ps.stripe_invoice_id, ps.created_at
		FROM tbl_practitioner_subscription ps
		JOIN tbl_practitioner p ON p.id = ps.practitioner_id AND p.deleted_at IS NULL
		JOIN tbl_user u ON u.id = p.user_id AND u.deleted_at IS NULL
		JOIN tbl_subscription s ON s.id = ps.subscription_id AND s.deleted_at IS NULL
		WHERE ps.deleted_at IS NULL
	`
	query, args := common.BuildQuery(base, f, billingHistoryColumns, nil, false)

	var rows []struct {
		ID                   int       `db:"id"`
		PractitionerID       uuid.UUID `db:"practitioner_id"`
		PractitionerEmail    string    `db:"practitioner_email"`
		PractitionerName     string    `db:"practitioner_name"`
		SubscriptionName     string    `db:"subscription_name"`
		Status               string    `db:"status"`
		StartDate            time.Time `db:"start_date"`
		EndDate              time.Time `db:"end_date"`
		StripeSubscriptionID *string   `db:"stripe_subscription_id"`
		StripeInvoiceID      *string   `db:"stripe_invoice_id"`
		CreatedAt            time.Time `db:"created_at"`
	}
	if err := r.db.SelectContext(ctx, &rows, r.db.Rebind(query), args...); err != nil {
		return nil, fmt.Errorf("list billing history: %w", err)
	}

	result := make([]*RsBillingHistoryRow, len(rows))
	for i, row := range rows {
		result[i] = &RsBillingHistoryRow{
			ID:                   row.ID,
			PractitionerID:       row.PractitionerID,
			PractitionerEmail:    row.PractitionerEmail,
			PractitionerName:     row.PractitionerName,
			SubscriptionName:     row.SubscriptionName,
			Status:               row.Status,
			StartDate:            row.StartDate,
			EndDate:              row.EndDate,
			StripeSubscriptionID: row.StripeSubscriptionID,
			StripeInvoiceID:      row.StripeInvoiceID,
			CreatedAt:            row.CreatedAt,
		}
	}
	return result, nil
}

func (r *repository) CountBillingHistory(ctx context.Context, f common.Filter) (int, error) {
	base := `
		FROM tbl_practitioner_subscription ps
		JOIN tbl_practitioner p ON p.id = ps.practitioner_id AND p.deleted_at IS NULL
		JOIN tbl_user u ON u.id = p.user_id AND u.deleted_at IS NULL
		JOIN tbl_subscription s ON s.id = ps.subscription_id AND s.deleted_at IS NULL
		WHERE ps.deleted_at IS NULL
	`
	query, args := common.BuildQuery(base, f, billingHistoryColumns, nil, true)

	var count int
	if err := r.db.GetContext(ctx, &count, r.db.Rebind(query), args...); err != nil {
		return 0, fmt.Errorf("count billing history: %w", err)
	}
	return count, nil
}

func (r *repository) ListActiveSubscriptions(ctx context.Context) ([]*RsSubscriptionPlan, error) {
	const query = `
		SELECT id, name, description, price, duration_days
		FROM tbl_subscription
		WHERE is_active = true AND deleted_at IS NULL
		ORDER BY price ASC
	`
	var rows []struct {
		ID           int     `db:"id"`
		Name         string  `db:"name"`
		Description  *string `db:"description"`
		Price        float64 `db:"price"`
		DurationDays int     `db:"duration_days"`
	}
	if err := r.db.SelectContext(ctx, &rows, query); err != nil {
		return nil, fmt.Errorf("list active subscriptions: %w", err)
	}
	result := make([]*RsSubscriptionPlan, len(rows))
	for i, row := range rows {
		result[i] = &RsSubscriptionPlan{
			ID:           row.ID,
			Name:         row.Name,
			Description:  row.Description,
			Price:        row.Price,
			DurationDays: row.DurationDays,
		}
	}
	return result, nil
}
