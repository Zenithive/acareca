package billing

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

// RqCheckout is the request body for POST /billing/checkout.
type RqCheckout struct {
	SubscriptionID int `json:"subscription_id" validate:"required,min=1"`
}

// RsCheckoutSession is the response for POST /billing/checkout.
type RsCheckoutSession struct {
	URL string `json:"url"`
}

// RsPortalSession is the response for POST /billing/portal.
type RsPortalSession struct {
	URL string `json:"url"`
}

// PractitionerStripe is a read model for the billing module.
type PractitionerStripe struct {
	ID               uuid.UUID
	Email            string
	FirstName        string
	LastName         string
	StripeCustomerID *string
}

// SubscriptionStripe is a read model for the billing module.
type SubscriptionStripe struct {
	ID            int
	Name          string
	Price         float64
	StripePriceID *string
}

// BillingHistoryFilter extends common.Filter for the admin billing history endpoint.
type BillingHistoryFilter struct {
	PractitionerID *uuid.UUID `form:"practitioner_id"`
	Status         *string    `form:"status"`
	FromDate       *time.Time `form:"from_date"`
	ToDate         *time.Time `form:"to_date"`
	common.Filter
}

// RsBillingHistoryRow is one row in the admin billing history response.
type RsBillingHistoryRow struct {
	ID                   int       `json:"id"`
	PractitionerID       uuid.UUID `json:"practitioner_id"`
	PractitionerEmail    string    `json:"practitioner_email"`
	PractitionerName     string    `json:"practitioner_name"`
	SubscriptionName     string    `json:"subscription_name"`
	Status               string    `json:"status"`
	StartDate            time.Time `json:"start_date"`
	EndDate              time.Time `json:"end_date"`
	StripeSubscriptionID *string   `json:"stripe_subscription_id,omitempty"`
	StripeInvoiceID      *string   `json:"stripe_invoice_id,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
}

// Sentinel errors for the billing module.
var (
	ErrTrialPlanNotPurchasable   = errors.New("trial plan cannot be purchased via checkout")
	ErrMissingStripePriceID      = errors.New("subscription has no stripe_price_id; sync it to Stripe first")
	ErrAlreadyActiveSubscription = errors.New("practitioner already has an active subscription for this plan")
	ErrNoBillingAccount          = errors.New("no billing account found for this practitioner")
	ErrInvalidWebhookSignature   = errors.New("invalid stripe webhook signature")
)

// toCommonFilter converts BillingHistoryFilter to a common.Filter for use in repository queries.
func (f *BillingHistoryFilter) toCommonFilter() common.Filter {
	filters := map[string]interface{}{}
	if f.PractitionerID != nil {
		filters["practitioner_id"] = *f.PractitionerID
	}
	if f.Status != nil {
		filters["status"] = *f.Status
	}
	if f.FromDate != nil {
		filters["created_at_gte"] = *f.FromDate
	}
	if f.ToDate != nil {
		filters["created_at_lte"] = *f.ToDate
	}
	return common.NewFilter(f.Search, filters, nil, f.Limit, f.Offset, f.SortBy, f.OrderBy)
}

// RsSubscriptionPlan is the public-facing plan listing response.
type RsSubscriptionPlan struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Description  *string `json:"description"`
	Price        float64 `json:"price"`
	DurationDays int     `json:"duration_days"`
}
