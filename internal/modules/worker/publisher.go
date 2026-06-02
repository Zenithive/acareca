package worker

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/notification"
	sharedEvents "github.com/iamarpitzala/acareca/internal/shared/events"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type Publisher struct {
	events sharedEvents.IEvent
}

func NewPublisher(events sharedEvents.IEvent) *Publisher {
	return &Publisher{
		events: events,
	}
}

// PublishEvent publishes an event to the specified subject
func (p *Publisher) PublishEvent(ctx context.Context, subject string, event interface{}) error {
	if p.events == nil {
		return fmt.Errorf("events system not configured")
	}

	if err := p.events.Publish(ctx, subject, event); err != nil {
		return fmt.Errorf("failed to publish event to %s: %w", subject, err)
	}

	return nil
}

// PublishNotification publishes a notification event to the in-app channel
func (p *Publisher) PublishNotification(ctx context.Context, event notification.NotificationEvent) error {
	return p.PublishEvent(ctx, notification.SubjectNotificationInApp, event)
}

// PublishEmailDelivery publishes a notification to the email delivery channel
func (p *Publisher) PublishEmailDelivery(ctx context.Context, notificationID, recipientID uuid.UUID, eventType util.EventType, payload interface{}) error {
	emailEvent := map[string]any{
		"notification_id": notificationID,
		"recipient_id":    recipientID,
		"event_type":      eventType,
		"payload":         payload,
	}
	return p.PublishEvent(ctx, notification.SubjectNotificationEmail, emailEvent)
}

// PublishPushDelivery publishes a notification to the push delivery channel
func (p *Publisher) PublishPushDelivery(ctx context.Context, notificationID, recipientID uuid.UUID, eventType util.EventType, payload interface{}) error {
	pushEvent := map[string]any{
		"notification_id": notificationID,
		"recipient_id":    recipientID,
		"event_type":      eventType,
		"payload":         payload,
	}
	return p.PublishEvent(ctx, notification.SubjectNotificationPush, pushEvent)
}
