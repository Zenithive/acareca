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
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/business/subscription"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/samber/lo"
	stripe "github.com/stripe/stripe-go/v82"
)

// HandleWebhook verifies the Stripe webhook signature and processes the event.
func (s *service) HandleWebhook(ctx context.Context, payload []byte, sigHeader string) error {
	fmt.Println("===========================================run run")
	webhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")

	event, err := s.stripeClient.ConstructWebhookEvent(payload, sigHeader, webhookSecret)
	fmt.Println("===========================================run run")
	if err != nil {
		log.Printf("webhook signature verification failed: sigHeader=%q secretLen=%d err=%v", sigHeader, len(webhookSecret), err)
		s.auditSvc.LogSystemIssue(ctx, auditctx.ActionSystemWarning, auditctx.ActionBillingWebhookSigInvalid,
			err, "", "Stripe", "WEBHOOK", auditctx.ModuleBilling)
		return ErrInvalidWebhookSignature
	}
	fmt.Println("===========================================run run", event.Type)

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
	fmt.Println("=======================================================run this")
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		return fmt.Errorf("parse checkout session: %w", err)
	}
	fmt.Println("=======================================================run this")

	practitionerIDStr, ok := session.Metadata["practitioner_id"]
	if !ok {
		return fmt.Errorf("missing practitioner_id in checkout session metadata")
	}
	subscriptionIDStr, ok := session.Metadata["subscription_id"]
	if !ok {
		return fmt.Errorf("missing subscription_id in checkout session metadata")
	}
	fmt.Println("=======================================================run this")

	practitionerID, err := uuid.Parse(practitionerIDStr)
	if err != nil {
		return fmt.Errorf("invalid practitioner_id: %w", err)
	}
	subscriptionID, err := strconv.Atoi(subscriptionIDStr)
	if err != nil {
		return fmt.Errorf("invalid subscription_id: %w", err)
	}
	fmt.Println("=======================================================run this")

	if session.Subscription == nil {
		return fmt.Errorf("checkout session has no subscription")
	}

	// Retrieve the Stripe subscription to get period end from items
	stripeSub, err := s.stripeClient.RetrieveSubscription(session.Subscription.ID)
	if err != nil {
		return fmt.Errorf("retrieve stripe subscription: %w", err)
	}
	fmt.Println("=======================================================run this")

	var invoiceIDPtr *string
	if stripeSub.LatestInvoice != nil && stripeSub.LatestInvoice.ID != "" {
		id := stripeSub.LatestInvoice.ID
		invoiceIDPtr = &id
	}

	fmt.Println("=======================================================run this")
	endDate := periodEnd(stripeSub)

	upsert := &subscription.WebhookUpsert{
		PractitionerID:       practitionerID,
		SubscriptionID:       subscriptionID,
		StripeSubscriptionID: stripeSub.ID,
		StripeInvoiceID:      invoiceIDPtr,
		Status:               subscription.StatusActive,
		StartDate:            time.Now(),
		EndDate:              endDate,
	}

	err = s.subRepo.UpsertFromWebhook(ctx, upsert)
	if err != nil {
		// CRITICAL: User paid but system failed to activate subscription in DB
		s.auditSvc.LogSystemIssue(ctx, auditctx.ActionSystemError, auditctx.ActionBillingActivationFailed,
			err, practitionerIDStr, subscriptionIDStr, auditctx.EntitySubscription, auditctx.ModuleBilling)
		return err
	}

	if err := s.subRepo.MarkPractitionerSubscriptionComplete(ctx, practitionerID); err != nil {
		log.Printf("ERROR: Failed to set subscription_status=COMPLETE for practitioner %s: %v", practitionerID, err)
	} else {
		log.Printf("✅ Set subscription_status=COMPLETE for practitioner %s", practitionerID)
	}
	fmt.Println("=======================================================run this")

	// LOG SUCCESS AUDIT (Payment Successful)
	pIDStr := practitionerID.String()
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		PracticeID: &pIDStr,
		Action:     auditctx.ActionBillingPaymentSuccess,
		Module:     auditctx.ModuleBilling,
		EntityType: lo.ToPtr(auditctx.EntitySubscription),
		EntityID:   &pIDStr,
		AfterState: map[string]interface{}{
			"stripe_sub_id": stripeSub.ID,
			"amount_total":  session.AmountTotal,
			"status":        subscription.StatusActive,
			"end_date":      endDate,
		},
	})

	go s.notifySubscriptionAlert(practitionerID, subscriptionID, string(subscription.StatusActive), time.Now(), endDate)

	fmt.Printf("\n[DEBUG]Subscription checkout completed notification has been called from Billing\n")

	return err
}

func (s *service) handleInvoicePaymentFailed(ctx context.Context, event stripe.Event) error {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		return fmt.Errorf("parse invoice: %w", err)
	}

	stripeSubID := invoice.Parent.SubscriptionDetails.Subscription.ID
	invoiceID := invoice.ID

	// In stripe-go v82, subscription is accessed via Parent.SubscriptionDetails
	if invoice.Parent == nil || invoice.Parent.SubscriptionDetails == nil || invoice.Parent.SubscriptionDetails.Subscription == nil {
		return fmt.Errorf("invoice has no subscription reference")
	}

	err := s.subRepo.UpdateStripeFields(ctx, stripeSubID, &invoiceID, subscription.StatusPastDue, time.Time{})
	if err != nil {
		// Warning: System couldn't mark account as past_due — route to system.warning.alert
		s.auditSvc.LogSystemIssue(ctx, auditctx.ActionSystemWarning, auditctx.ActionBillingStatusUpdateFailed,
			err, "", stripeSubID, auditctx.EntitySubscription, auditctx.ModuleBilling)
		return err
	}

	// Log payment failure as a billing event so admins with billing.alert preference are notified
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		Action:     auditctx.ActionBillingPaymentFailed,
		Module:     auditctx.ModuleBilling,
		EntityType: lo.ToPtr(auditctx.EntitySubscription),
		// EntityID:   &stripeSubID,
		AfterState: map[string]interface{}{
			"invoice_id":    invoiceID,
			"stripe_sub_id": stripeSubID,
			"status":        subscription.StatusPastDue,
		},
	})

	var practitionerID uuid.UUID
	if invoice.Parent.SubscriptionDetails.Subscription.Metadata != nil {
		if pracIDStr, ok := invoice.Parent.SubscriptionDetails.Subscription.Metadata["practitioner_id"]; ok {
			practitionerID, _ = uuid.Parse(pracIDStr)
		}
	}

	go s.notifyPaymentFailedAlert(practitionerID, stripeSubID, invoiceID, float64(invoice.AmountDue)/100.0)

	return nil
}

func (s *service) handleSubscriptionDeleted(ctx context.Context, event stripe.Event) error {
	var stripeSub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &stripeSub); err != nil {
		return fmt.Errorf("parse subscription: %w", err)
	}

	err := s.subRepo.UpdateStripeFields(ctx, stripeSub.ID, nil, subscription.StatusCancelled, time.Time{})
	if err != nil {
		return err
	}

	var practitionerID uuid.UUID
	if stripeSub.Metadata != nil {
		if pracIDStr, ok := stripeSub.Metadata["practitioner_id"]; ok {
			practitionerID, _ = uuid.Parse(pracIDStr)
		}
	}

	if practitionerID != uuid.Nil {
		if err := s.subRepo.MarkPractitionerSubscriptionPending(ctx, practitionerID); err != nil {
			log.Printf("ERROR: Failed to set subscription_status=PENDING for practitioner %s: %v", practitionerID, err)
		} else {
			log.Printf("✅ Set subscription_status=PENDING for practitioner %s (subscription deleted)", practitionerID)
		}
	}

	go s.notifySubscriptionDeletedAlert(practitionerID, stripeSub.ID)

	return nil
}

func (s *service) handleSubscriptionUpdated(ctx context.Context, event stripe.Event) error {
	var stripeSub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &stripeSub); err != nil {
		return fmt.Errorf("parse subscription: %w", err)
	}

	status := mapStripeStatus(string(stripeSub.Status))
	endDate := periodEnd(&stripeSub)

	var invoiceIDPtr *string
	if stripeSub.LatestInvoice != nil && stripeSub.LatestInvoice.ID != "" {
		id := stripeSub.LatestInvoice.ID
		invoiceIDPtr = &id
	}

	err := s.subRepo.UpdateStripeFields(ctx, stripeSub.ID, invoiceIDPtr, status, endDate)
	if err != nil {
		return err
	}

	var practitionerID uuid.UUID
	var internalSubID int
	if stripeSub.Metadata != nil {
		if pracIDStr, ok := stripeSub.Metadata["practitioner_id"]; ok {
			practitionerID, _ = uuid.Parse(pracIDStr)
		}
		if idStr, ok := stripeSub.Metadata["subscription_id"]; ok {
			internalSubID, _ = strconv.Atoi(idStr)
		}
	}

	// Update practitioner subscription_status based on the subscription status
	if practitionerID != uuid.Nil {
		if status == subscription.StatusActive {
			if err := s.subRepo.MarkPractitionerSubscriptionComplete(ctx, practitionerID); err != nil {
				log.Printf("ERROR: Failed to set subscription_status=COMPLETE for practitioner %s: %v", practitionerID, err)
			} else {
				log.Printf("✅ Set subscription_status=COMPLETE for practitioner %s", practitionerID)
			}
		} else {
			// If subscription is not active (PAST_DUE, CANCELLED, PAUSED, EXPIRED), set to PENDING
			if err := s.subRepo.MarkPractitionerSubscriptionPending(ctx, practitionerID); err != nil {
				log.Printf("ERROR: Failed to set subscription_status=PENDING for practitioner %s: %v", practitionerID, err)
			} else {
				log.Printf("✅ Set subscription_status=PENDING for practitioner %s (status: %s)", practitionerID, status)
			}
		}

		go s.notifySubscriptionAlert(practitionerID, internalSubID, string(status), time.Now(), endDate)
	}

	fmt.Printf("\n[DEBUG]Subscription updated notification has been called from Billing\n")

	return nil
}

func (s *service) notifySubscriptionAlert(practitionerID uuid.UUID, subscriptionPlanID int, status string, activationDate time.Time, endDate time.Time) {
	if s.adminRepo == nil {
		log.Printf("[WARN] adminRepo is nil, cannot fetch admin recipients")
		return
	}

	// 1. Fetch admins using the proper struct field: s.adminRepo
	admins, err := s.adminRepo.GetAllAdmins(context.Background())
	if err != nil {
		log.Printf("[WARN] failed to get admins for subscription alert: %v", err)
		return
	}

	recipients := make([]sharednotification.RecipientWithPreferences, 0, len(admins))
	for _, a := range admins {
		recipients = append(recipients, sharednotification.RecipientWithPreferences{
			RecipientID:   a.ID,
			RecipientType: util.ActorAdmin,
			UserID:        a.User.ID,
		})
	}

	if len(recipients) == 0 {
		log.Printf("[INFO] no admin recipients found for subscription notification")
		return
	}

	prac, err := s.repo.GetPractitionerWithStripe(context.Background(), practitionerID)
	practitionerName := "Unknown Practitioner"
	if err == nil && prac != nil {
		practitionerName = prac.FirstName + " " + prac.LastName
	}

	planName := fmt.Sprintf("Plan #%d", subscriptionPlanID)
	if subModel, err := s.repo.GetSubscriptionWithStripe(context.Background(), subscriptionPlanID); err == nil && subModel != nil {
		planName = subModel.Name
	}

	title := "Practitioner Subscription Activated"
	body := fmt.Sprintf("Practitioner %s has subscribed to %s Plan. Renewal Date: %s", practitionerName, planName, endDate.Format("02-01-2006"))

	if s.notificationPub == nil {
		log.Printf("[WARN] notificationPub is nil, skipping alert dispatch")
		return
	}

	_ = s.notificationPub.Publish(context.Background(), sharednotification.PublishRequest{
		Recipients: recipients,
		SenderID:   practitionerID,
		SenderType: util.ActorPractitioner,
		SenderName: practitionerName,
		EventType:  util.EventType(util.EventSubscriptionAlert),
		EntityType: util.EntitySubscription,
		EntityID:   practitionerID,
		EntityKey:  "practitioner_id",
		Title:      title,
		Body:       body,
	})
}

func (s *service) notifyPaymentFailedAlert(practitionerID uuid.UUID, stripeSubID string, invoiceID string, amountDue float64) {
	if s.adminRepo == nil || s.notificationPub == nil {
		return
	}

	admins, err := s.adminRepo.GetAllAdmins(context.Background())
	if err != nil || len(admins) == 0 {
		return
	}

	recipients := make([]sharednotification.RecipientWithPreferences, 0, len(admins))
	for _, a := range admins {
		recipients = append(recipients, sharednotification.RecipientWithPreferences{
			RecipientID:   a.ID,
			RecipientType: util.ActorAdmin,
			UserID:        a.User.ID,
		})
	}

	practitionerName := "Unknown Practitioner"
	if practitionerID != uuid.Nil {
		if prac, err := s.repo.GetPractitionerWithStripe(context.Background(), practitionerID); err == nil && prac != nil {
			practitionerName = prac.FirstName + " " + prac.LastName
		}
	}

	title := "Subscription Payment Failed"
	body := fmt.Sprintf("Payment of $%.2f failed for practitioner %s (Invoice: %s)", amountDue, practitionerName, invoiceID)

	_ = s.notificationPub.Publish(context.Background(), sharednotification.PublishRequest{
		Recipients: recipients,
		SenderID:   practitionerID,
		SenderType: util.ActorPractitioner,
		SenderName: practitionerName,
		EventType:  util.EventType(util.EventSubscriptionAlert),
		EntityType: util.EntitySubscription,
		EntityID:   practitionerID,
		EntityKey:  "practitioner_id",
		Title:      title,
		Body:       body,
	})
}

func (s *service) notifySubscriptionDeletedAlert(practitionerID uuid.UUID, stripeSubID string) {
	if s.adminRepo == nil || s.notificationPub == nil {
		return
	}

	admins, err := s.adminRepo.GetAllAdmins(context.Background())
	if err != nil || len(admins) == 0 {
		return
	}

	recipients := make([]sharednotification.RecipientWithPreferences, 0, len(admins))
	for _, a := range admins {
		recipients = append(recipients, sharednotification.RecipientWithPreferences{
			RecipientID:   a.ID,
			RecipientType: util.ActorAdmin,
			UserID:        a.User.ID,
		})
	}

	practitionerName := "Unknown Practitioner"
	if practitionerID != uuid.Nil {
		if prac, err := s.repo.GetPractitionerWithStripe(context.Background(), practitionerID); err == nil && prac != nil {
			practitionerName = prac.FirstName + " " + prac.LastName
		}
	}

	title := "Subscription Cancelled"
	body := fmt.Sprintf("The subscription for practitioner %s has been cancelled", practitionerName)

	_ = s.notificationPub.Publish(context.Background(), sharednotification.PublishRequest{
		Recipients: recipients,
		SenderID:   practitionerID,
		SenderType: util.ActorPractitioner,
		SenderName: practitionerName,
		EventType:  util.EventType(util.EventSubscriptionAlert),
		EntityType: util.EntitySubscription,
		EntityID:   practitionerID,
		EntityKey:  "practitioner_id",
		Title:      title,
		Body:       body,
	})
}

func periodEnd(sub *stripe.Subscription) time.Time {
	if sub.Items != nil && len(sub.Items.Data) > 0 {
		return time.Unix(sub.Items.Data[0].CurrentPeriodEnd, 0)
	}
	return time.Time{}
}

// mapStripeStatus maps a Stripe subscription status string to a local Status.
func mapStripeStatus(stripeStatus string) subscription.Status {
	switch stripeStatus {
	case "active", "trialing":
		return subscription.StatusActive
	case "past_due", "unpaid", "incomplete":
		return subscription.StatusPastDue
	case "canceled", "incomplete_expired":
		return subscription.StatusCancelled
	case "paused":
		return subscription.StatusPaused
	default:
		return subscription.StatusPastDue
	}
}
