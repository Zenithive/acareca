package notification

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/iamarpitzala/acareca/internal/modules/notification/preference"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

const (
	workerInterval  = 30 * time.Second
	workerBatchSize = 50
)

func StartRetryWorker(ctx context.Context, repo Repository, hub *sharednotification.Hub, preferredSvc preference.IService) {
	ticker := time.NewTicker(workerInterval)
	defer ticker.Stop()

	log.Println("notification retry worker started")
	for {
		select {
		case <-ctx.Done():
			log.Println("notification retry worker stopped")
			return
		case <-ticker.C:
			preferredSvc.Get(ctx) // optional: refresh preferences cache
			retryFailed(ctx, repo, hub, preferredSvc)
		}
	}
}

func retryFailed(ctx context.Context, repo Repository, hub *sharednotification.Hub, preferredSvc preference.IService) {
	deliveries, err := repo.ListFailedInAppDeliveries(ctx, workerBatchSize)
	if err != nil {
		log.Printf("retry worker: list failed deliveries: %v", err)
		return
	}
	if len(deliveries) == 0 {
		return
	}

	for _, d := range deliveries {
		select {
		case <-ctx.Done():
			log.Println("retry worker: context cancelled, stopping retry batch")
			return
		default:
		}

		pushedToWebSocket := false

		push := map[string]any{
			"id":             d.NotificationID,
			"recipient_id":   d.RecipientID,
			"recipient_type": util.ActorTypePractitioner,
			"sender_id":      nil,
			"sender_type":    nil,
			"event_type":     d.EventType,
			"entity_type":    d.EntityType,
			"entity_id":      d.EntityID,
			"status":         "UNREAD",
			"payload":        json.RawMessage(d.Payload),
			"created_at":     d.CreatedAt,
		}

		if hub.Push(d.RecipientID, push) {
			pushedToWebSocket = true
			log.Printf("retry worker: pushed to active WebSocket for user %s", d.RecipientID)
		}

		if err := repo.MarkDeliveryDelivered(ctx, d.NotificationID, util.ChannelInApp); err != nil {
			log.Printf("retry worker: failed to mark delivered %s: %v", d.NotificationID, err)
		} else {
			if pushedToWebSocket {
				log.Printf("retry worker: delivered %s (pushed to WebSocket)", d.NotificationID)
			} else {
				log.Printf("retry worker: delivered %s (stored for later retrieval)", d.NotificationID)
			}
		}
	}
}
