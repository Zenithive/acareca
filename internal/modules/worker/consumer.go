package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/notification"
	"github.com/iamarpitzala/acareca/internal/modules/notification/preference"
	sharedEvents "github.com/iamarpitzala/acareca/internal/shared/events"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
	"github.com/nats-io/nats.go/jetstream"
)

type Consumer struct {
	events        sharedEvents.IEvent
	repo          notification.Repository
	prefRepo      preference.Repository
	notifier      *sharednotification.Hub
	db            *sqlx.DB
	publisher     *Publisher
	streamManager *StreamManager
}

func NewConsumer(events sharedEvents.IEvent, repo notification.Repository, notifier *sharednotification.Hub, db *sqlx.DB, publisher *Publisher, prefRepo preference.Repository) *Consumer {
	consumer := &Consumer{
		events:    events,
		repo:      repo,
		notifier:  notifier,
		db:        db,
		publisher: publisher,
		prefRepo:  prefRepo,
	}

	consumer.streamManager = NewStreamManager(events, consumer)

	return consumer
}

func (c *Consumer) GetStreamManager() *StreamManager {
	return c.streamManager
}

func (c *Consumer) StartNotificationInAppConsumer(ctx context.Context) error {
	if c.events == nil {
		return fmt.Errorf("events system not configured")
	}

	log.Println("Starting notification in-app consumer...")

	return c.events.Consume(
		ctx,
		notification.StreamNotification,
		notification.ConsumerNotificationInApp,
		notification.SubjectNotificationInApp,
		c.handleNotificationInApp,
	)
}

func (c *Consumer) handleNotificationInApp(msg jetstream.Msg) error {
	ctx := context.Background()

	var event notification.NotificationEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		return fmt.Errorf("failed to unmarshal notification event: %w", err)
	}

	if err, _ := c.shouldNotifyUser(ctx, event.RecipientID, event.EntityID, event.RecipientType, event.EventType); err != nil {
		log.Printf("User %s opted out of event type %s for entity %s", event.RecipientID, event.EventType, event.EntityID)
		return nil
	}

	notificationID, err := c.createNotification(ctx, event, event.Channels)
	if err != nil {
		return err
	}

	c.deliverToChannels(ctx, notificationID, event, event.Channels)

	return nil
}

func (c *Consumer) createNotification(ctx context.Context, event notification.NotificationEvent, channels []util.Channel) (uuid.UUID, error) {
	notif := notification.Notification{
		ID:            event.ID,
		RecipientID:   event.RecipientID,
		RecipientType: event.RecipientType,
		SenderID:      event.SenderID,
		SenderType:    event.SenderType,
		EventType:     event.EventType,
		EntityType:    event.EntityType,
		EntityID:      event.EntityID,
		Status:        util.StatusUnread,
		Payload:       event.Payload,
		CreatedAt:     event.CreatedAt,
	}

	var notificationID uuid.UUID
	err := util.RunInTransaction(ctx, c.db, func(ctx context.Context, tx *sqlx.Tx) error {
		var txErr error
		notificationID, txErr = c.repo.CreateNotificationWithDeliveries(ctx, tx, notif, channels)
		return txErr
	})

	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create notification: %w", err)
	}

	return notificationID, nil
}

func (c *Consumer) deliverToChannels(ctx context.Context, notificationID uuid.UUID, event notification.NotificationEvent, channels []util.Channel) {
	for _, ch := range channels {
		switch ch {
		case util.ChannelInApp:
			c.deliverInApp(ctx, notificationID, event)
		}
	}
}

func (c *Consumer) deliverInApp(ctx context.Context, notificationID uuid.UUID, event notification.NotificationEvent) {
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
			"status":       util.StatusUnread,
			"payload":      event.Payload,
			"created_at":   event.CreatedAt,
		}

		if c.notifier.Push(event.RecipientID, payload) {
			pushedToWebSocket = true
			log.Printf("Notification pushed to active WebSocket via hub: %s", notificationID)
		}
	}

	if err := c.repo.MarkDeliveryDelivered(ctx, notificationID, util.ChannelInApp); err != nil {
		log.Printf("Failed to mark delivery as delivered %s: %v", notificationID, err)
	} else {
		if pushedToWebSocket {
			log.Printf("In-app notification delivered (pushed to WebSocket): %s", notificationID)
		} else {
			log.Printf("In-app notification delivered (stored for later retrieval): %s", notificationID)
		}
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

func (c *Consumer) shouldNotifyUser(ctx context.Context, userID, entityID uuid.UUID, entityType util.ActorType, eventType util.EventType) (error, bool) {
	pref, err := c.prefRepo.GetPreferencesByUserID(ctx, userID)
	if err != nil {
		return err, true
	}

	notificationEventType := mapEventTypeToNotificationEventType(eventType)

	if pref.EntityID == entityID && pref.EntityType == string(entityType) {
		for _, pref := range pref.EventType {
			if pref == notificationEventType {
				return nil, true
			}
		}
		return nil, false
	}

	return nil, true
}

// mapEventTypeToNotificationEventType maps event types to notification event types
func mapEventTypeToNotificationEventType(eventType util.EventType) util.NotificationEventType {
	switch eventType {
	case util.EventTransactionCreated, util.EventTransactionUpdated:
		return util.EventNewTransaction
	case util.EventClinicUpdated, util.EventFormSubmitted, util.EventFormUpdated, util.EventDocumentUploaded,
		util.EventInviteSent, util.EventInviteAccepted, util.EventInviteDeclined:
		return util.EventAccountantActivityAlert
	case util.EventSystemError, util.EventSystemWarning, util.EventAuditLogCreated:
		return util.EventSystemActivityAlert
	default:
		return util.EventSystemActivityAlert
	}
}
