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

	consumer.streamManager = NewStreamManager(events, consumer)

	return consumer
}

func (c *Consumer) GetStreamManager() *StreamManager {
	return c.streamManager
}

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

func (c *Consumer) handleNotificationCreate(msg jetstream.Msg) error {
	ctx := context.Background()

	var event NotificationEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		return fmt.Errorf("failed to unmarshal notification event: %w", err)
	}

	if !c.shouldNotifyUser(ctx, event.RecipientID, event.EntityID, event.RecipientType, event.EventType) {
		log.Printf("User %s opted out of event type %s for entity %s", event.RecipientID, event.EventType, event.EntityID)
		return nil
	}

	allowedChannels := c.getEnabledChannels(ctx, event.RecipientID, event.EntityID, event.RecipientType, event.EventType, event.Channels)
	if len(allowedChannels) == 0 {
		log.Printf("No channels enabled for notification %s", event.ID)
		return nil
	}

	notificationID, err := c.createNotification(ctx, event, allowedChannels)
	if err != nil {
		return err
	}

	c.deliverToChannels(ctx, notificationID, event, allowedChannels)

	return nil
}

func (c *Consumer) handleEmailDelivery(msg jetstream.Msg) error {
	ctx := context.Background()

	notificationID, err := c.parseDeliveryEvent(msg.Data())
	if err != nil {
		return err
	}

	log.Printf("Processing email delivery for notification: %s", notificationID)

	if err := c.repo.MarkDeliveryDelivered(ctx, notificationID, ChannelEmail); err != nil {
		return fmt.Errorf("failed to mark email as delivered: %w", err)
	}

	return nil
}

func (c *Consumer) handlePushDelivery(msg jetstream.Msg) error {
	ctx := context.Background()

	notificationID, err := c.parseDeliveryEvent(msg.Data())
	if err != nil {
		return err
	}

	log.Printf("Processing push delivery for notification: %s", notificationID)

	if err := c.repo.MarkDeliveryDelivered(ctx, notificationID, ChannelPush); err != nil {
		return fmt.Errorf("failed to mark push as delivered: %w", err)
	}

	return nil
}

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

func (c *Consumer) deliverInApp(ctx context.Context, notificationID uuid.UUID, event NotificationEvent) {
	pushedToWebSocket := false

	if c.streamManager != nil && c.streamManager.IsUserStreamActive(event.RecipientID) {
		if c.streamManager.DeliverToUser(event.RecipientID, event) {
			pushedToWebSocket = true
			log.Printf("Notification pushed to active WebSocket for user %s", event.RecipientID)
		}
	} else if c.notifier != nil {
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
			pushedToWebSocket = true
			log.Printf("Notification pushed to active WebSocket via hub: %s", notificationID)
		}
	}

	if err := c.repo.MarkDeliveryDelivered(ctx, notificationID, ChannelInApp); err != nil {
		log.Printf("Failed to mark delivery as delivered %s: %v", notificationID, err)
	} else {
		if pushedToWebSocket {
			log.Printf("In-app notification delivered (pushed to WebSocket): %s", notificationID)
		} else {
			log.Printf("In-app notification delivered (stored for later retrieval): %s", notificationID)
		}
	}
}

func (c *Consumer) queueEmailDelivery(ctx context.Context, notificationID uuid.UUID, event NotificationEvent) {
	if err := c.publisher.PublishEmailDelivery(ctx, notificationID, event.RecipientID, event.EventType, event.Payload); err != nil {
		log.Printf("Failed to queue email for notification %s: %v", notificationID, err)
		_ = c.repo.MarkDeliveryFailed(ctx, notificationID, ChannelEmail, "failed to publish to NATS")
	} else {
		log.Printf("Email delivery queued for notification %s", notificationID)
	}
}

func (c *Consumer) queuePushDelivery(ctx context.Context, notificationID uuid.UUID, event NotificationEvent) {
	if err := c.publisher.PublishPushDelivery(ctx, notificationID, event.RecipientID, event.EventType, event.Payload); err != nil {
		log.Printf("Failed to queue push for notification %s: %v", notificationID, err)
		_ = c.repo.MarkDeliveryFailed(ctx, notificationID, ChannelPush, "failed to publish to NATS")
	} else {
		log.Printf("Push delivery queued for notification %s", notificationID)
	}
}

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

func (c *Consumer) getEnabledChannels(ctx context.Context,userID, entityID uuid.UUID,entityType ActorType,eventType EventType,requestedChannels []Channel,) []Channel {

	// ❗ FIX 1: pass entityID, NOT userID
	prefs, err := c.repo.GetAllPreferencesByentityID(ctx, userID)


	if err != nil || len(prefs) == 0 {
		return []Channel{}
	}

	notificationEventType := MapEventTypeToNotificationEventType(eventType)
	var matchingPref *NotificationPreference
	for i := range prefs {
		if prefs[i].EntityID == userID &&
			prefs[i].EntityType == string(entityType) &&
			prefs[i].EventType == notificationEventType {
			matchingPref = &prefs[i]
			break
		}
	}
	if matchingPref == nil {
		return []Channel{}
	}

	enabledChannels := make(map[Channel]bool)

	for channelKey, enabled := range matchingPref.Channels {
		if enabled {
			ch := Channel(channelKey)
			if ch.IsValid() {
				enabledChannels[ch] = true
			}
		}
	}

	var allowedChannels []Channel

	for _, ch := range requestedChannels {
		if enabledChannels[ch] {
			allowedChannels = append(allowedChannels, ch)
		}
	}

	return allowedChannels
}

func (c *Consumer) shouldNotifyUser(ctx context.Context, userID, entityID uuid.UUID, entityType ActorType, eventType EventType) bool {
	prefs, err := c.repo.GetAllPreferences(ctx, userID)
	if err != nil || len(prefs) == 0 {
		return true
	}

	notificationEventType := MapEventTypeToNotificationEventType(eventType)

	for _, pref := range prefs {
		if pref.EntityID == entityID && pref.EntityType == string(entityType) {
			if pref.EventType == notificationEventType {
				return true
			}
			return false
		}
	}

	return true
}
