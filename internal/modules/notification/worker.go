package notification

import (
	"context"
	"encoding/json"
	"log"
	"time"

	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
)

const (
	workerInterval  = 30 * time.Second
	workerBatchSize = 50
)

// StartRetryWorker polls for FAILED in_app deliveries and retries them via WebSocket.
func StartRetryWorker(ctx context.Context, repo Repository, hub *sharednotification.Hub) {
	ticker := time.NewTicker(workerInterval)
	defer ticker.Stop()

	log.Println("notification retry worker started")
	for {
		select {
		case <-ctx.Done():
			log.Println("notification retry worker stopped")
			return
		case <-ticker.C:
			retryFailed(ctx, repo, hub)
		}
	}
}

func retryFailed(ctx context.Context, repo Repository, hub *sharednotification.Hub) {
	deliveries, err := repo.ListFailedInAppDeliveries(ctx, workerBatchSize)
	if err != nil {
		log.Printf("retry worker: list failed deliveries: %v", err)
		return
	}
	if len(deliveries) == 0 {
		return
	}

	log.Printf("retry worker: retrying %d failed in_app deliveries", len(deliveries))

	for _, d := range deliveries {
		push := map[string]any{
			"id":           d.NotificationID,
			"recipient_id": d.RecipientID,
			"event_type":   d.EventType,
			"entity_type":  d.EntityType,
			"entity_id":    d.EntityID,
			"payload":      json.RawMessage(d.Payload),
			"created_at":   d.CreatedAt,
		}

		// Try to push to WebSocket first
		if hub.Push(d.RecipientID, push) {
			// Push succeeded, mark as delivered
			if err := repo.MarkDeliveryDelivered(ctx, d.NotificationID, ChannelInApp); err != nil {
				log.Printf("retry worker: failed to mark delivered %s: %v", d.NotificationID, err)
			} else {
				log.Printf("retry worker: delivered %s", d.NotificationID)
			}
		} else {
			// Push failed, increment retry count
			if err := repo.MarkDeliveryFailed(ctx, d.NotificationID, ChannelInApp, "no active WebSocket clients"); err != nil {
				log.Printf("retry worker: failed to update retry count %s: %v", d.NotificationID, err)
			}
		}
	}
}
