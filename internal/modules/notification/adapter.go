package notification

import (
	"context"

	"github.com/google/uuid"

	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

// ServiceAdapter adapts the notification.Service to sharednotification.NotificationService interface
type ServiceAdapter struct {
	svc Service
}

// NewServiceAdapter creates a new adapter
func NewServiceAdapter(svc Service) sharednotification.NotificationService {
	return &ServiceAdapter{svc: svc}
}

// Publish adapts the Publish method
func (a *ServiceAdapter) Publish(ctx context.Context, rq sharednotification.NotificationRequest) error {
	// Convert sharednotification.NotificationRequest to notification.RqNotification
	notifReq := RqNotification{
		ID:            rq.ID,
		RecipientID:   rq.RecipientID,
		RecipientType: rq.RecipientType,
		SenderID:      rq.SenderID,
		SenderType:    rq.SenderType,
		EventType:     rq.EventType,
		EntityType:    rq.EntityType,
		EntityID:      rq.EntityID,
		Status:        rq.Status,
		Payload:       rq.Payload,
		Channels:      rq.Channels,
		CreatedAt:     rq.CreatedAt,
	}

	return a.svc.Publish(ctx, notifReq)
}

// GetPreferences adapts the GetPreferences method and returns a map of event types to channels
func (a *ServiceAdapter) GetPreferences(ctx context.Context, userID uuid.UUID) (map[util.NotificationEventType][]util.Channel, error) {
	prefs, err := a.svc.GetPreferences(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Build a map: eventType -> channels
	prefMap := make(map[util.NotificationEventType][]util.Channel, len(prefs))
	for _, pref := range prefs {
		// Each preference row has one event type (stored as array with single element)
		if len(pref.EventType) > 0 {
			prefMap[pref.EventType[0]] = pref.Channels
		}
	}

	return prefMap, nil
}
