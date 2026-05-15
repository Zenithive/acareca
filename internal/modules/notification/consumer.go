package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"
	sharedEvents "github.com/iamarpitzala/acareca/internal/shared/events"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
	"github.com/nats-io/nats.go/jetstream"
)

// Consumer handles consuming notification events from NATS
type Consumer struct {
	events    sharedEvents.IEvent
	repo      Repository
	notifier  *sharednotification.Hub
	db        *sqlx.DB
	publisher *Publisher
}

func NewConsumer(
	events sharedEvents.IEvent,
	repo Repository,
	notifier *sharednotification.Hub,
	db *sqlx.DB,
	publisher *Publisher,
) *Consumer {
	return &Consumer{
		events:    events,
		repo:      repo,
		notifier:  notifier,
		db:        db,
		publisher: publisher,
	}
}

// StartNotificationCreateConsumer starts consuming notification creation events
func (c *Consumer) StartNotificationCreateConsumer(ctx context.Context) error {
	if c.events == nil {
		return fmt.Errorf("events system not configured")
	}

	log.Println("Starting notification create consumer...")

	return c.events.Consume(
		ctx,
		StreamNotification,
		ConsumerNotificationInApp,
		SubjectNotificationInApp,
		c.handleNotificationCreate,
	)
}

// StartEmailConsumer starts consuming email delivery events
func (c *Consumer) StartEmailConsumer(ctx context.Context) error {
	if c.events == nil {
		return fmt.Errorf("events system not configured")
	}

	log.Println("Starting email delivery consumer...")

	return c.events.Consume(
		ctx,
		StreamNotification,
		ConsumerNotificationEmail,
		SubjectNotificationEmail,
		c.handleEmailDelivery,
	)
}

// StartPushConsumer starts consuming push notification delivery events
func (c *Consumer) StartPushConsumer(ctx context.Context) error {
	if c.events == nil {
		return fmt.Errorf("events system not configured")
	}

	log.Println("Starting push notification consumer...")

	return c.events.Consume(
		ctx,
		StreamNotification,
		ConsumerNotificationPush,
		SubjectNotificationPush,
		c.handlePushDelivery,
	)
}

// handleNotificationCreate processes notification creation events
func (c *Consumer) handleNotificationCreate(msg jetstream.Msg) error {
	ctx := context.Background()

	var event NotificationEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		return fmt.Errorf("failed to unmarshal notification event: %w", err)
	}

	log.Printf("Processing notification: %s for recipient: %s", event.ID, event.RecipientID)

	// Check if user should be notified based on their event type preferences
	if !c.shouldNotifyUser(ctx, event.RecipientID, event.EntityID, event.RecipientType, event.EventType) {
		log.Printf("User %s has disabled notifications for event type %s on entity %s, skipping",
			event.RecipientID, event.EventType, event.EntityID)
		return nil
	}

	// Filter channels based on user preferences
	allowedChannels := c.filterChannels(ctx, event.RecipientID, event.Channels)
	if len(allowedChannels) == 0 {
		log.Printf("No channels enabled for notification %s, skipping", event.ID)
		return nil
	}

	// Create notification record
	notification := Notification{
		ID:            event.ID,
		RecipientID:   event.RecipientID,
		RecipientType: event.RecipientType,
		SenderID:      event.SenderID,
		SenderType:    event.SenderType,
		EventType:     event.EventType,
		EntityType:    event.EntityType,
		EntityID:      event.EntityID,
		Status:        StatusUnread,
		Payload:       event.Payload,
		CreatedAt:     event.CreatedAt,
	}

	// Create notification and deliveries in a transaction
	var notificationID uuid.UUID
	err := util.RunInTransaction(ctx, c.db, func(ctx context.Context, tx *sqlx.Tx) error {
		var txErr error
		notificationID, txErr = c.repo.CreateNotificationWithDeliveries(ctx, tx, notification, allowedChannels)
		return txErr
	})
	if err != nil {
		return fmt.Errorf("failed to create notification with deliveries: %w", err)
	}

	// Attempt real-time delivery for each channel
	for _, ch := range allowedChannels {
		switch ch {
		case ChannelInApp:
			c.deliverInApp(ctx, notificationID, event)
		case ChannelEmail:
			if err := c.publisher.PublishEmailDelivery(ctx, notificationID, event.RecipientID, event.EventType, event.Payload); err != nil {
				log.Printf("Failed to publish email event for notification %s: %v", notificationID, err)
				_ = c.repo.MarkDeliveryFailed(ctx, notificationID, ChannelEmail, "failed to publish to NATS")
			} else {
				log.Printf("Email delivery queued for notification %s", notificationID)
			}
		case ChannelPush:
			if err := c.publisher.PublishPushDelivery(ctx, notificationID, event.RecipientID, event.EventType, event.Payload); err != nil {
				log.Printf("Failed to publish push event for notification %s: %v", notificationID, err)
				_ = c.repo.MarkDeliveryFailed(ctx, notificationID, ChannelPush, "failed to publish to NATS")
			} else {
				log.Printf("Push delivery queued for notification %s", notificationID)
			}
		}
	}

	return nil
}

// handleEmailDelivery processes email delivery events
func (c *Consumer) handleEmailDelivery(msg jetstream.Msg) error {
	ctx := context.Background()

	var event map[string]interface{}
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		return fmt.Errorf("failed to unmarshal email event: %w", err)
	}

	notificationIDStr, ok := event["notification_id"].(string)
	if !ok {
		return fmt.Errorf("invalid notification_id in email event")
	}

	notificationID, err := uuid.Parse(notificationIDStr)
	if err != nil {
		return fmt.Errorf("failed to parse notification_id: %w", err)
	}

	log.Printf("Processing email delivery for notification: %s", notificationID)

	// TODO: Implement actual email sending logic here

	if err := c.repo.MarkDeliveryDelivered(ctx, notificationID, ChannelEmail); err != nil {
		return fmt.Errorf("failed to mark email as delivered: %w", err)
	}

	return nil
}

// handlePushDelivery processes push notification delivery events
func (c *Consumer) handlePushDelivery(msg jetstream.Msg) error {
	ctx := context.Background()

	var event map[string]interface{}
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		return fmt.Errorf("failed to unmarshal push event: %w", err)
	}

	notificationIDStr, ok := event["notification_id"].(string)
	if !ok {
		return fmt.Errorf("invalid notification_id in push event")
	}

	notificationID, err := uuid.Parse(notificationIDStr)
	if err != nil {
		return fmt.Errorf("failed to parse notification_id: %w", err)
	}

	log.Printf("Processing push delivery for notification: %s", notificationID)

	// TODO: Implement actual push notification logic here (FCM, APNs, etc.)

	if err := c.repo.MarkDeliveryDelivered(ctx, notificationID, ChannelPush); err != nil {
		return fmt.Errorf("failed to mark push as delivered: %w", err)
	}

	return nil
}

// deliverInApp attempts to deliver notification via WebSocket
func (c *Consumer) deliverInApp(ctx context.Context, notificationID uuid.UUID, event NotificationEvent) {
	if c.notifier == nil {
		log.Printf("Notifier hub not available for in-app delivery")
		_ = c.repo.MarkDeliveryFailed(
			ctx,
			notificationID,
			ChannelInApp,
			"notifier hub not available",
		)
		return
	}

	push := map[string]any{
		"id":           notificationID,
		"recipient_id": event.RecipientID,
		"sender_id":    event.SenderID,
		"event_type":   event.EventType,
		"entity_type":  event.EntityType,
		"entity_id":    event.EntityID,
		"status":       StatusUnread,
		"payload":      event.Payload,
		"created_at":   event.CreatedAt,
	}

	if c.notifier.Push(event.RecipientID, push) {
		_ = c.repo.MarkDeliveryDelivered(ctx, notificationID, ChannelInApp)
		log.Printf("In-app notification delivered: %s", notificationID)
	} else {
		_ = c.repo.MarkDeliveryFailed(
			ctx,
			notificationID,
			ChannelInApp,
			"no active WebSocket clients",
		)
		log.Printf("In-app notification failed (no active clients): %s", notificationID)
	}
}

// filterChannels returns channels that are enabled for the user based on their preferences
// It checks preferences by userID, entityID, entityType, and maps EventType to NotificationEventType
func (c *Consumer) filterChannels(ctx context.Context, userID uuid.UUID, requestedChannels []Channel) []Channel {
	prefs, err := c.repo.GetAllPreferences(ctx, userID)
	if err != nil || len(prefs) == 0 {
		log.Printf("No preferences found for user %s, using default (in_app only)", userID)
		return []Channel{ChannelInApp}
	}

	// Merge all channel preferences across all user preferences
	enabledChannels := make(map[Channel]bool)
	for _, pref := range prefs {
		for channelKey, enabled := range pref.Channels {
			if enabled {
				ch := Channel(channelKey)
				if ch.IsValid() {
					enabledChannels[ch] = true
				}
			}
		}
	}

	// Filter requested channels by enabled channels
	var allowedChannels []Channel
	for _, ch := range requestedChannels {
		if enabledChannels[ch] {
			allowedChannels = append(allowedChannels, ch)
		}
	}

	return allowedChannels
}

// shouldNotifyUser checks if the user should be notified based on their preferences
// It considers entityID, entityType, and event type preferences
func (c *Consumer) shouldNotifyUser(ctx context.Context, userID, entityID uuid.UUID, entityType ActorType, eventType EventType) bool {
	prefs, err := c.repo.GetAllPreferences(ctx, userID)
	if err != nil || len(prefs) == 0 {
		// No preferences found - use default behavior (notify for all events)
		return true
	}

	notificationEventType := MapEventTypeToNotificationEventType(eventType)

	// Check if user has a preference for this entity and event type
	for _, pref := range prefs {
		// Match by entityID and entityType
		if pref.EntityID == entityID && pref.EntityType == string(entityType) {
			// Check if this event type is in the user's enabled event types
			for _, enabledEventType := range pref.EventType {
				if enabledEventType == notificationEventType {
					// User has this event type enabled for this entity
					return true
				}
			}
			// User has preferences for this entity but this event type is not enabled
			return false
		}
	}

	// No specific preference for this entity - use default behavior (notify)
	return true
}
