package notification

import (
	"context"

	"github.com/google/uuid"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
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

// GetPreferences adapts the GetPreferences method
func (a *ServiceAdapter) GetPreferences(ctx context.Context, userID uuid.UUID) (sharednotification.NotificationPreferences, error) {
	prefs, err := a.svc.GetPreferences(ctx, userID)
	if err != nil {
		return sharednotification.NotificationPreferences{}, err
	}

	// Convert []Channel to map[string]bool
	channelsMap := make(map[string]bool)
	for _, ch := range prefs.Channels {
		channelsMap[string(ch)] = true
	}

	// Convert preference.Preference to sharednotification.NotificationPreferences
	return sharednotification.NotificationPreferences{
		EventType: prefs.EventType,
		Channels:  channelsMap,
	}, nil
}
