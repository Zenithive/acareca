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
	events        sharedEvents.IEvent
	repo          Repository
	notifier      *sharednotification.Hub
	db            *sqlx.DB
	publisher     *Publisher
	streamManager *StreamManager
}

func NewConsumer(
	events sharedEvents.IEvent,
	repo Repository,
	notifier *sharednotification.Hub,
	db *sqlx.DB,
	publisher *Publisher,
) *Consumer {
	consumer := &Consumer{
		events:    events,
		repo:      repo,
		notifier:  notifier,
		db:        db,
		publisher: publisher,
	}
	
	// Initialize stream manager with reference to this consumer
	consumer.streamManager = NewStreamManager(events, consumer)
	
	return consumer
}

// GetStreamManager returns the stream manager for WebSocket integration
func (c *Consumer) GetStreamManager() *StreamManager {
	return c.streamManager
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

	// Check if user should be notified based on event type preferences
	if !c.shouldNotifyUser(ctx, event.RecipientID, event.EntityID, event.RecipientType, event.EventType) {
		log.Printf("User %s opted out of event type %s for entity %s", event.RecipientID, event.EventType, event.EntityID)
		return nil
	}

	// Filter channels based on user preferences
	allowedChannels := c.getEnabledChannels(ctx, event.RecipientID, event.Channels)
	if len(allowedChannels) == 0 {
		log.Printf("No channels enabled for notification %s", event.ID)
		return nil
	}

	// Create notification in database
	notificationID, err := c.createNotification(ctx, event, allowedChannels)
	if err != nil {
		return err
	}

	// Deliver to each enabled channel
	c.deliverToChannels(ctx, notificationID, event, allowedChannels)

	return nil
}

// handleEmailDelivery processes email delivery events
func (c *Consumer) handleEmailDelivery(msg jetstream.Msg) error {
	ctx := context.Background()

	notificationID, err := c.parseDeliveryEvent(msg.Data())
	if err != nil {
		return err
	}

	log.Printf("Processing email delivery for notification: %s", notificationID)

	// TODO: Implement actual email sending logic here
	// Example: err := emailService.Send(ctx, notificationID)

	if err := c.repo.MarkDeliveryDelivered(ctx, notificationID, ChannelEmail); err != nil {
		return fmt.Errorf("failed to mark email as delivered: %w", err)
	}

	return nil
}

// handlePushDelivery processes push notification delivery events
func (c *Consumer) handlePushDelivery(msg jetstream.Msg) error {
	ctx := context.Background()

	notificationID, err := c.parseDeliveryEvent(msg.Data())
	if err != nil {
		return err
	}

	log.Printf("Processing push delivery for notification: %s", notificationID)

	// TODO: Implement actual push notification logic here (FCM, APNs, etc.)
	// Example: err := pushService.Send(ctx, notificationID)

	if err := c.repo.MarkDeliveryDelivered(ctx, notificationID, ChannelPush); err != nil {
		return fmt.Errorf("failed to mark push as delivered: %w", err)
	}

	return nil
}

// createNotification creates notification and delivery records in a transaction
func (c *Consumer) createNotification(ctx context.Context, event NotificationEvent, channels []Channel) (uuid.UUID, error) {
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

	var notificationID uuid.UUID
	err := util.RunInTransaction(ctx, c.db, func(ctx context.Context, tx *sqlx.Tx) error {
		var txErr error
		notificationID, txErr = c.repo.CreateNotificationWithDeliveries(ctx, tx, notification, channels)
		return txErr
	})

	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create notification: %w", err)
	}

	return notificationID, nil
}

// deliverToChannels attempts delivery to all enabled channels
func (c *Consumer) deliverToChannels(ctx context.Context, notificationID uuid.UUID, event NotificationEvent, channels []Channel) {
	for _, ch := range channels {
		switch ch {
		case ChannelInApp:
			c.deliverInApp(ctx, notificationID, event)
		case ChannelEmail:
			c.queueEmailDelivery(ctx, notificationID, event)
		case ChannelPush:
			c.queuePushDelivery(ctx, notificationID, event)
		}
	}
}

// deliverInApp attempts immediate delivery via WebSocket
func (c *Consumer) deliverInApp(ctx context.Context, notificationID uuid.UUID, event NotificationEvent) {
	delivered := false

	// Try stream delivery first (for users with active WebSocket connections)
	if c.streamManager != nil && c.streamManager.IsUserStreamActive(event.RecipientID) {
		if c.streamManager.DeliverToUser(event.RecipientID, event) {
			delivered = true
			log.Printf("Notification delivered via stream to user %s", event.RecipientID)
		}
	}

	// Fallback to hub delivery if stream delivery didn't work
	if !delivered && c.notifier != nil {
		payload := map[string]any{
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

		if c.notifier.Push(event.RecipientID, payload) {
			delivered = true
			log.Printf("In-app notification delivered via hub: %s", notificationID)
		}
	}

	// Mark delivery status
	if delivered {
		_ = c.repo.MarkDeliveryDelivered(ctx, notificationID, ChannelInApp)
	} else {
		_ = c.repo.MarkDeliveryFailed(ctx, notificationID, ChannelInApp, "no active WebSocket clients")
	}
}

// queueEmailDelivery publishes email delivery event to NATS
func (c *Consumer) queueEmailDelivery(ctx context.Context, notificationID uuid.UUID, event NotificationEvent) {
	if err := c.publisher.PublishEmailDelivery(ctx, notificationID, event.RecipientID, event.EventType, event.Payload); err != nil {
		log.Printf("Failed to queue email for notification %s: %v", notificationID, err)
		_ = c.repo.MarkDeliveryFailed(ctx, notificationID, ChannelEmail, "failed to publish to NATS")
	} else {
		log.Printf("Email delivery queued for notification %s", notificationID)
	}
}

// queuePushDelivery publishes push delivery event to NATS
func (c *Consumer) queuePushDelivery(ctx context.Context, notificationID uuid.UUID, event NotificationEvent) {
	if err := c.publisher.PublishPushDelivery(ctx, notificationID, event.RecipientID, event.EventType, event.Payload); err != nil {
		log.Printf("Failed to queue push for notification %s: %v", notificationID, err)
		_ = c.repo.MarkDeliveryFailed(ctx, notificationID, ChannelPush, "failed to publish to NATS")
	} else {
		log.Printf("Push delivery queued for notification %s", notificationID)
	}
}

// parseDeliveryEvent extracts notification ID from delivery event
func (c *Consumer) parseDeliveryEvent(data []byte) (uuid.UUID, error) {
	var event map[string]interface{}
	if err := json.Unmarshal(data, &event); err != nil {
		return uuid.Nil, fmt.Errorf("failed to unmarshal delivery event: %w", err)
	}

	notificationIDStr, ok := event["notification_id"].(string)
	if !ok {
		return uuid.Nil, fmt.Errorf("invalid notification_id in delivery event")
	}

	notificationID, err := uuid.Parse(notificationIDStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to parse notification_id: %w", err)
	}

	return notificationID, nil
}

// getEnabledChannels returns channels enabled in user preferences
func (c *Consumer) getEnabledChannels(ctx context.Context, userID uuid.UUID, requestedChannels []Channel) []Channel {
	prefs, err := c.repo.GetAllPreferences(ctx, userID)
	if err != nil || len(prefs) == 0 {
		// Default: only in-app notifications
		return []Channel{ChannelInApp}
	}

	// Collect all enabled channels from user preferences
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

// shouldNotifyUser checks if user has enabled notifications for this event type and entity
func (c *Consumer) shouldNotifyUser(ctx context.Context, userID, entityID uuid.UUID, entityType ActorType, eventType EventType) bool {
	prefs, err := c.repo.GetAllPreferences(ctx, userID)
	if err != nil || len(prefs) == 0 {
		// No preferences = notify by default
		return true
	}

	notificationEventType := MapEventTypeToNotificationEventType(eventType)

	// Check if user has preferences for this specific entity
	for _, pref := range prefs {
		if pref.EntityID == entityID && pref.EntityType == string(entityType) {
			// Check if this event type is enabled
			for _, enabledEventType := range pref.EventType {
				if enabledEventType == notificationEventType {
					return true
				}
			}
			// User has preferences for this entity but event type is disabled
			return false
		}
	}

	// No specific preference for this entity = notify by default
	return true
}
