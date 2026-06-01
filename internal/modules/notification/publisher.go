package notification

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	sharedEvents "github.com/iamarpitzala/acareca/internal/shared/events"
)

type Publisher struct {
	events sharedEvents.IEvent
}

func NewPublisher(events sharedEvents.IEvent) *Publisher {
	return &Publisher{
		events: events,
	}
}

func (p *Publisher) PublishNotification(ctx context.Context, event NotificationEvent) error {
	if p.events == nil {
		return fmt.Errorf("events system not configured")
	}

	if err := p.events.Publish(ctx, SubjectNotificationInApp, event); err != nil {
		return fmt.Errorf("failed to publish notification event: %w", err)
	}

	return nil
}

func (p *Publisher) PublishEmailDelivery(ctx context.Context, notificationID, recipientID uuid.UUID, eventType EventType, payload interface{}) error {
	if p.events == nil {
		return fmt.Errorf("events system not configured")
	}

	emailEvent := map[string]interface{}{
		"notification_id": notificationID,
		"recipient_id":    recipientID,
		"event_type":      eventType,
		"payload":         payload,
	}

	if err := p.events.Publish(ctx, SubjectNotificationEmail, emailEvent); err != nil {
		return fmt.Errorf("failed to publish email event: %w", err)
	}

	return nil
}

func (p *Publisher) PublishPushDelivery(ctx context.Context, notificationID, recipientID uuid.UUID, eventType EventType, payload interface{}) error {
	if p.events == nil {
		return fmt.Errorf("events system not configured")
	}

	pushEvent := map[string]interface{}{
		"notification_id": notificationID,
		"recipient_id":    recipientID,
		"event_type":      eventType,
		"payload":         payload,
	}

	if err := p.events.Publish(ctx, SubjectNotificationPush, pushEvent); err != nil {
		return fmt.Errorf("failed to publish push event: %w", err)
	}

	return nil
}
