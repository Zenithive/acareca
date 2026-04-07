package billing

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	"github.com/iamarpitzala/acareca/internal/modules/business/subscription"
	sharedstripe "github.com/iamarpitzala/acareca/internal/shared/stripe"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	stripe "github.com/stripe/stripe-go/v82"
)

// Service defines all billing operations.
type Service interface {
	CreateCheckoutSession(ctx context.Context, practitionerID uuid.UUID, req *RqCheckout) (*RsCheckoutSession, error)
	CreatePortalSession(ctx context.Context, practitionerID uuid.UUID) (*RsPortalSession, error)
	HandleWebhook(ctx context.Context, payload []byte, sigHeader string) error
	ListBillingHistory(ctx context.Context, f *BillingHistoryFilter) (*util.RsList, error)
	ListSubscriptions(ctx context.Context) ([]*RsSubscriptionPlan, error)
}

type service struct {
	repo             Repository
	practitionerRepo practitioner.Repository
	subRepo          subscription.Repository
	stripeClient     sharedstripe.StripeClient
}

// NewService constructs a billing Service.
func NewService(
	repo Repository,
	practitionerRepo practitioner.Repository,
	subRepo subscription.Repository,
	stripeClient sharedstripe.StripeClient,
) Service {
	return &service{
		repo:             repo,
		practitionerRepo: practitionerRepo,
		subRepo:          subRepo,
		stripeClient:     stripeClient,
	}
}

// CreateCheckoutSession creates a Stripe Checkout session for a practitioner upgrading to a paid plan.
func (s *service) CreateCheckoutSession(ctx context.Context, practitionerID uuid.UUID, req *RqCheckout) (*RsCheckoutSession, error) {
	// Fetch practitioner and subscription read models
	prac, err := s.repo.GetPractitionerWithStripe(ctx, practitionerID)
	if err != nil {
		return nil, fmt.Errorf("get practitioner: %w", err)
	}

	sub, err := s.repo.GetSubscriptionWithStripe(ctx, req.SubscriptionID)
	if err != nil {
		return nil, fmt.Errorf("get subscription: %w", err)
	}

	// Guard: Trial plan cannot be purchased
	if sub.Price == 0 {
		return nil, ErrTrialPlanNotPurchasable
	}

	// Guard: Subscription must have a Stripe price ID
	if sub.StripePriceID == nil {
		return nil, ErrMissingStripePriceID
	}

	// Guard: No existing active subscription for this plan
	activeSub, err := s.subRepo.GetActiveSubscription(ctx, practitionerID)
	if err != nil && !errors.Is(err, subscription.ErrNotFound) {
		return nil, fmt.Errorf("check active subscription: %w", err)
	}
	if activeSub != nil && activeSub.Subscription.ID == req.SubscriptionID {
		return nil, ErrAlreadyActiveSubscription
	}

	// Create Stripe Customer if not yet created
	customerID := ""
	if prac.StripeCustomerID != nil {
		customerID = *prac.StripeCustomerID
	} else {
		fullName := prac.FirstName + " " + prac.LastName
		cid, err := s.stripeClient.CreateCustomer(prac.Email, fullName)
		if err != nil {
			return nil, fmt.Errorf("create stripe customer: %w", err)
		}
		if err := s.practitionerRepo.UpdateStripeCustomerID(ctx, practitionerID, cid); err != nil {
			return nil, fmt.Errorf("persist stripe customer id: %w", err)
		}
		customerID = cid
	}

	successURL := os.Getenv("STRIPE_SUCCESS_URL")
	cancelURL := os.Getenv("STRIPE_CANCEL_URL")

	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(*sub.StripePriceID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
		Metadata: map[string]string{
			"practitioner_id": practitionerID.String(),
			"subscription_id": fmt.Sprintf("%d", req.SubscriptionID),
		},
	}

	url, err := s.stripeClient.CreateCheckoutSession(params)
	if err != nil {
		return nil, fmt.Errorf("create checkout session: %w", err)
	}

	return &RsCheckoutSession{URL: url}, nil
}

// CreatePortalSession creates a Stripe Billing Portal session for a practitioner.
func (s *service) CreatePortalSession(ctx context.Context, practitionerID uuid.UUID) (*RsPortalSession, error) {
	prac, err := s.repo.GetPractitionerWithStripe(ctx, practitionerID)
	if err != nil {
		return nil, fmt.Errorf("get practitioner: %w", err)
	}

	if prac.StripeCustomerID == nil {
		return nil, ErrNoBillingAccount
	}

	returnURL := os.Getenv("STRIPE_CANCEL_URL")

	url, err := s.stripeClient.CreatePortalSession(*prac.StripeCustomerID, returnURL)
	if err != nil {
		return nil, fmt.Errorf("create portal session: %w", err)
	}

	return &RsPortalSession{URL: url}, nil
}

// HandleWebhook is implemented in webhook.go.

// ListBillingHistory returns a paginated list of billing history rows.
func (s *service) ListBillingHistory(ctx context.Context, f *BillingHistoryFilter) (*util.RsList, error) {
	cf := f.toCommonFilter()

	rows, err := s.repo.ListBillingHistory(ctx, cf)
	if err != nil {
		return nil, err
	}

	total, err := s.repo.CountBillingHistory(ctx, cf)
	if err != nil {
		return nil, err
	}

	var rs util.RsList
	rs.MapToList(rows, total, *cf.Offset, *cf.Limit)
	return &rs, nil
}

// ListSubscriptions returns all active subscription plans for practitioner-facing display.
func (s *service) ListSubscriptions(ctx context.Context) ([]*RsSubscriptionPlan, error) {
	return s.repo.ListActiveSubscriptions(ctx)
}
