package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/business/subscription"
	stripe "github.com/stripe/stripe-go/v82"
)

// HandleWebhook verifies the Stripe webhook signature and processes the event.
func (s *service) HandleWebhook(ctx context.Context, payload []byte, sigHeader string) error {
	webhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")

	event, err := s.stripeClient.ConstructWebhookEvent(payload, sigHeader, webhookSecret)
	if err != nil {
		log.Printf("webhook signature verification failed: sigHeader=%q secretLen=%d err=%v", sigHeader, len(webhookSecret), err)
		return ErrInvalidWebhookSignature
	}

	switch event.Type {
	case "checkout.session.completed":
		return s.handleCheckoutCompleted(ctx, event)
	case "invoice.payment_failed":
		return s.handleInvoicePaymentFailed(ctx, event)
	case "customer.subscription.deleted":
		return s.handleSubscriptionDeleted(ctx, event)
	case "customer.subscription.updated":
		return s.handleSubscriptionUpdated(ctx, event)
	default:
		// Return nil for unhandled event types to prevent Stripe retries
		return nil
	}
}

func (s *service) handleCheckoutCompleted(ctx context.Context, event stripe.Event) error {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		return fmt.Errorf("parse checkout session: %w", err)
	}

	practitionerIDStr, ok := session.Metadata["practitioner_id"]
	if !ok {
		return fmt.Errorf("missing practitioner_id in checkout session metadata")
	}
	subscriptionIDStr, ok := session.Metadata["subscription_id"]
	if !ok {
		return fmt.Errorf("missing subscription_id in checkout session metadata")
	}

	practitionerID, err := uuid.Parse(practitionerIDStr)
	if err != nil {
		return fmt.Errorf("invalid practitioner_id: %w", err)
	}
	subscriptionID, err := strconv.Atoi(subscriptionIDStr)
	if err != nil {
		return fmt.Errorf("invalid subscription_id: %w", err)
	}

	if session.Subscription == nil {
		return fmt.Errorf("checkout session has no subscription")
	}

	// Retrieve the Stripe subscription to get period end from items
	stripeSub, err := s.stripeClient.RetrieveSubscription(session.Subscription.ID)
	if err != nil {
		return fmt.Errorf("retrieve stripe subscription: %w", err)
	}

	endDate := periodEnd(stripeSub)

	upsert := &subscription.WebhookUpsert{
		PractitionerID:       practitionerID,
		SubscriptionID:       subscriptionID,
		StripeSubscriptionID: stripeSub.ID,
		Status:               subscription.StatusActive,
		StartDate:            time.Now(),
		EndDate:              endDate,
	}

	return s.subRepo.UpsertFromWebhook(ctx, upsert)
}

func (s *service) handleInvoicePaymentFailed(ctx context.Context, event stripe.Event) error {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		return fmt.Errorf("parse invoice: %w", err)
	}

	// In stripe-go v82, subscription is accessed via Parent.SubscriptionDetails
	if invoice.Parent == nil || invoice.Parent.SubscriptionDetails == nil || invoice.Parent.SubscriptionDetails.Subscription == nil {
		return fmt.Errorf("invoice has no subscription reference")
	}

	stripeSubID := invoice.Parent.SubscriptionDetails.Subscription.ID
	invoiceID := invoice.ID

	return s.subRepo.UpdateStripeFields(ctx, stripeSubID, &invoiceID, subscription.StatusPastDue, time.Time{})
}

func (s *service) handleSubscriptionDeleted(ctx context.Context, event stripe.Event) error {
	var stripeSub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &stripeSub); err != nil {
		return fmt.Errorf("parse subscription: %w", err)
	}

	return s.subRepo.UpdateStripeFields(ctx, stripeSub.ID, nil, subscription.StatusCancelled, time.Time{})
}

func (s *service) handleSubscriptionUpdated(ctx context.Context, event stripe.Event) error {
	var stripeSub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &stripeSub); err != nil {
		return fmt.Errorf("parse subscription: %w", err)
	}

	status := mapStripeStatus(string(stripeSub.Status))
	endDate := periodEnd(&stripeSub)

	return s.subRepo.UpdateStripeFields(ctx, stripeSub.ID, nil, status, endDate)
}

// periodEnd extracts the current period end from the first subscription item.
// In stripe-go v82, current_period_end lives on SubscriptionItem, not Subscription.
func periodEnd(sub *stripe.Subscription) time.Time {
	if sub.Items != nil && len(sub.Items.Data) > 0 {
		return time.Unix(sub.Items.Data[0].CurrentPeriodEnd, 0)
	}
	return time.Time{}
}

// mapStripeStatus maps a Stripe subscription status string to a local Status.
func mapStripeStatus(stripeStatus string) subscription.Status {
	switch stripeStatus {
	case "active":
		return subscription.StatusActive
	case "past_due":
		return subscription.StatusPastDue
	case "canceled":
		return subscription.StatusCancelled
	case "paused":
		return subscription.StatusPaused
	default:
		return subscription.StatusActive
	}
}
