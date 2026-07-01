package subscription

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/notification"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

const (
	// Worker runs every 6 hours
	expiryCheckInterval = 6 * time.Hour
	
	// Notification thresholds (days before expiry)
	expiryWarning7Days = 7
	expiryWarning1Day  = 1
)

type ExpiryWorker struct {
	repo      Repository
	publisher *notification.Publisher
}

func NewExpiryWorker(repo Repository, publisher *notification.Publisher) *ExpiryWorker {
	return &ExpiryWorker{
		repo:      repo,
		publisher: publisher,
	}
}

// StartExpiryWorker runs a background worker that checks for expiring and expired subscriptions
func (w *ExpiryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(expiryCheckInterval)
	defer ticker.Stop()

	log.Println("✅ Subscription expiry worker started")
	
	// Run immediately on start
	w.checkAndNotify(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("Subscription expiry worker stopped")
			return
		case <-ticker.C:
			w.checkAndNotify(ctx)
		}
	}
}

func (w *ExpiryWorker) checkAndNotify(ctx context.Context) {
	// 1. Mark expired subscriptions
	if err := w.markExpiredSubscriptions(ctx); err != nil {
		log.Printf("ERROR: Failed to mark expired subscriptions: %v", err)
	}

	// 2. Notify about subscriptions expiring in 7 days
	if err := w.notifyExpiringSubscriptions(ctx, expiryWarning7Days); err != nil {
		log.Printf("ERROR: Failed to notify 7-day expiring subscriptions: %v", err)
	}

	// 3. Notify about subscriptions expiring in 1 day
	if err := w.notifyExpiringSubscriptions(ctx, expiryWarning1Day); err != nil {
		log.Printf("ERROR: Failed to notify 1-day expiring subscriptions: %v", err)
	}
}

func (w *ExpiryWorker) markExpiredSubscriptions(ctx context.Context) error {
	expired, err := w.repo.ListExpiredSubscriptions(ctx)
	if err != nil {
		return fmt.Errorf("list expired subscriptions: %w", err)
	}

	if len(expired) == 0 {
		return nil
	}

	log.Printf("Found %d expired subscriptions to process", len(expired))

	for _, sub := range expired {
		// Mark subscription row as EXPIRED
		if err := w.repo.MarkAsExpired(ctx, sub.ID); err != nil {
			log.Printf("ERROR: Failed to mark subscription %d as expired: %v", sub.ID, err)
			continue
		}

		log.Printf("✅ Marked subscription %d as EXPIRED for practitioner %s", sub.ID, sub.PractitionerID)

		// Set subscription_status = PENDING on tbl_practitioner so the Auth middleware
		// returns 402 on the practitioner's next request.
		if err := w.repo.MarkPractitionerSubscriptionPending(ctx, sub.PractitionerID); err != nil {
			log.Printf("ERROR: Failed to set subscription_status=PENDING for practitioner %s: %v", sub.PractitionerID, err)
			// Non-fatal: subscription is already marked EXPIRED, continue with notification
		} else {
			log.Printf("✅ Set subscription_status=PENDING for practitioner %s", sub.PractitionerID)
		}

		// Send expiry notification
		if err := w.sendExpiryNotification(ctx, sub); err != nil {
			log.Printf("ERROR: Failed to send expiry notification for subscription %d: %v", sub.ID, err)
		}
	}

	return nil
}

func (w *ExpiryWorker) notifyExpiringSubscriptions(ctx context.Context, daysBeforeExpiry int) error {
	expiring, err := w.repo.ListExpiringSubscriptions(ctx, daysBeforeExpiry)
	if err != nil {
		return fmt.Errorf("list expiring subscriptions (%d days): %w", daysBeforeExpiry, err)
	}

	if len(expiring) == 0 {
		return nil
	}

	log.Printf("Found %d subscriptions expiring in %d day(s)", len(expiring), daysBeforeExpiry)

	for _, sub := range expiring {
		// Calculate days remaining
		daysRemaining := int(time.Until(sub.EndDate).Hours() / 24)
		
		// Only send notification if it matches the threshold (avoid duplicate notifications)
		if daysRemaining == daysBeforeExpiry {
			if err := w.sendExpiringNotification(ctx, sub, daysRemaining); err != nil {
				log.Printf("ERROR: Failed to send expiring notification for subscription %d: %v", sub.ID, err)
			} else {
				log.Printf("✅ Sent expiry warning for subscription %d (expires in %d day(s))", sub.ID, daysRemaining)
			}
		}
	}

	return nil
}

func (w *ExpiryWorker) sendExpiringNotification(ctx context.Context, sub *PractitionerSubscription, daysRemaining int) error {
	var title, body string
	if daysRemaining == 1 {
		title = "⚠️ Your Subscription Expires Tomorrow"
		body = fmt.Sprintf("Your subscription will expire on %s. Please renew to continue using all features.", sub.EndDate.Format("January 2, 2006"))
	} else {
		title = fmt.Sprintf("⚠️ Your Subscription Expires in %d Days", daysRemaining)
		body = fmt.Sprintf("Your subscription will expire on %s. Please renew to avoid any service interruption.", sub.EndDate.Format("January 2, 2006"))
	}

	publishReq := notification.PublishRequest{
		Recipients: []notification.RecipientWithPreferences{
			{
				RecipientID:   sub.PractitionerID,
				RecipientType: util.ActorPractitioner,
				UserID:        sub.PractitionerID,
			},
		},
		SenderID:   uuid.Nil,
		SenderType: util.ActorSystem,
		SenderName: "System",
		EventType:  util.EventSubscriptionExpiring,
		EntityType: util.EntitySubscription,
		EntityID:   sub.PractitionerID,
		EntityKey:  fmt.Sprintf("subscription_%d", sub.ID),
		Title:      title,
		Body:       body,
		ExtraData: map[string]interface{}{
			"subscription_id": sub.ID,
			"end_date":        sub.EndDate.Format(time.RFC3339),
			"days_remaining":  daysRemaining,
		},
	}

	return w.publisher.Publish(ctx, publishReq)
}

func (w *ExpiryWorker) sendExpiryNotification(ctx context.Context, sub *PractitionerSubscription) error {
	title := "❌ Your Subscription Has Expired"
	body := fmt.Sprintf("Your subscription expired on %s. Please renew to regain access to all features.", sub.EndDate.Format("January 2, 2006"))

	publishReq := notification.PublishRequest{
		Recipients: []notification.RecipientWithPreferences{
			{
				RecipientID:   sub.PractitionerID,
				RecipientType: util.ActorPractitioner,
				UserID:        sub.PractitionerID,
			},
		},
		SenderID:   uuid.Nil,
		SenderType: util.ActorSystem,
		SenderName: "System",
		EventType:  util.EventSubscriptionExpired,
		EntityType: util.EntitySubscription,
		EntityID:   sub.PractitionerID,
		EntityKey:  fmt.Sprintf("subscription_%d", sub.ID),
		Title:      title,
		Body:       body,
		ExtraData: map[string]interface{}{
			"subscription_id": sub.ID,
			"end_date":        sub.EndDate.Format(time.RFC3339),
			"status":          "expired",
		},
	}

	return w.publisher.Publish(ctx, publishReq)
}
