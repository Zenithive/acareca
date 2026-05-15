package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	sharedEvents "github.com/iamarpitzala/acareca/internal/shared/events"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
	"github.com/nats-io/nats.go/jetstream"
)

type Service interface {
	// Core notification operations
	Publish(ctx context.Context, rq RqNotification) error
	List(ctx context.Context, recipientID uuid.UUID, filter FilterNotification) (*util.RsList, error)
	MarkRead(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error
	MarkAllRead(ctx context.Context, recipientID uuid.UUID) error
	MarkDismissed(ctx context.Context, ids []uuid.UUID, recipientID uuid.UUID) error

	// Preference operations
	GetPreferences(ctx context.Context, userID uuid.UUID) ([]NotificationPreference, error)
	UpdatePreference(ctx context.Context, userID, entityID uuid.UUID, role string, rq RqUpdatePreference) error

	// Consumer operations (for NATS event processing)
	StartNotificationCreateConsumer(ctx context.Context) error
	StartEmailConsumer(ctx context.Context) error
	StartPushConsumer(ctx context.Context) error
	PreferenceSetting(ctx context.Context, userID uuid.UUID, entityID uuid.UUID, entityType string) error
}

type service struct {
	repo     Repository
	notifier *sharednotification.Hub
	events   sharedEvents.IEvent
	db       *sqlx.DB
}

func NewService(repo Repository, notifier *sharednotification.Hub, events sharedEvents.IEvent, db *sqlx.DB) Service {
	return &service{
		repo:     repo,
		notifier: notifier,
		events:   events,
		db:       db,
	}
}

// Publish publishes notification asynchronously via NATS
func (s *service) Publish(ctx context.Context, rq RqNotification) error {
	// If events system is not available, return error
	if s.events == nil {
		return fmt.Errorf("events system not configured - cannot publish notification")
	}

	event := NotificationEvent{
		ID:            rq.ID,
		RecipientID:   rq.RecipientID,
		RecipientType: rq.RecipientType,
		SenderID:      rq.SenderID,
		SenderType:    rq.SenderType,
		EventType:     rq.EventType,
		EntityType:    rq.EntityType,
		EntityID:      rq.EntityID,
		Payload:       rq.Payload,
		Channels:      rq.Channels,
		CreatedAt:     rq.CreatedAt,
	}

	if err := s.events.Publish(ctx, SubjectNotificationInApp, event); err != nil {
		return fmt.Errorf("failed to publish notification event: %w", err)
	}

	return nil
}

func (s *service) List(ctx context.Context, recipientID uuid.UUID, filter FilterNotification) (*util.RsList, error) {
	notifications, total, err := s.repo.ListByRecipient(ctx, recipientID, filter)
	if err != nil {
		return nil, err
	}

	// Get the GLOBAL unread count
	unreadCount := 0
	unreadCount, err = s.repo.GetUnreadCount(ctx, recipientID)
	if err != nil {
		log.Printf("Error in count notifications: %s", err)
	}

	// Set pagination defaults
	limit := 10
	if filter.Limit != nil && *filter.Limit > 0 {
		limit = *filter.Limit
	}
	
	offset := 0
	if filter.Offset != nil && *filter.Offset >= 0 {
		offset = *filter.Offset
	}

	// Create response with unread count in metadata
	result := &util.RsList{}
	result.MapToList(map[string]interface{}{
		"notifications": notifications,
		"unread_count":  unreadCount,
	}, total, offset, limit)

	return result, nil
}

func (s *service) MarkRead(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error {
	return s.repo.MarkRead(ctx, id, recipientID)
}

func (s *service) MarkAllRead(ctx context.Context, recipientID uuid.UUID) error {
	return s.repo.MarkAllRead(ctx, recipientID)
}

func (s *service) MarkDismissed(ctx context.Context, ids []uuid.UUID, recipientID uuid.UUID) error {
	return s.repo.MarkDismissed(ctx, ids, recipientID)
}

func (s *service) GetPreferences(ctx context.Context, userID uuid.UUID) ([]NotificationPreference, error) {
	prefs, err := s.repo.GetAllPreferences(ctx, userID)
	if err != nil {
		return nil, err
	}
	if prefs == nil {
		return []NotificationPreference{}, nil
	}
	return prefs, nil
}

func (s *service) UpdatePreference(ctx context.Context, userID, entityID uuid.UUID, role string, rq RqUpdatePreference) error {
	pref := NotificationPreference{
		UserID:     userID,
		EntityID:   entityID,
		EntityType: role,
		EventType:  rq.EventType,
		Channels:   rq.Channels,
	}
	return s.repo.CreatePreference(ctx, pref)
}

func (s *service) filterChannels(ctx context.Context, userID uuid.UUID, requestedChannels []Channel) []Channel {
	// Get all preferences for the user
	prefs, err := s.repo.GetAllPreferences(ctx, userID)
	if err != nil || len(prefs) == 0 {
		log.Printf("No preferences found for user %s, using default (in_app only)", userID)
		return []Channel{ChannelInApp}
	}

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

	var allowedChannels []Channel
	for _, ch := range requestedChannels {
		if enabledChannels[ch] {
			allowedChannels = append(allowedChannels, ch)
		}
	}

	return allowedChannels
}

// StartNotificationCreateConsumer starts consuming notification creation events
func (s *service) StartNotificationCreateConsumer(ctx context.Context) error {
	if s.events == nil {
		return fmt.Errorf("events system not configured")
	}

	return s.events.Consume(
		ctx,
		StreamNotification,
		ConsumerNotificationInApp,
		SubjectNotificationInApp,
		s.handleNotificationCreate,
	)
}

// handleNotificationCreate processes notification creation events
func (s *service) handleNotificationCreate(msg jetstream.Msg) error {
	ctx := context.Background()
	var event NotificationEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		return fmt.Errorf("failed to unmarshal notification event: %w", err)
	}

	// Filter channels based on user preferences
	allowedChannels := s.filterChannels(ctx, event.RecipientID, event.Channels)
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
	err := util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		var txErr error
		notificationID, txErr = s.repo.CreateNotificationWithDeliveries(ctx, tx, notification, allowedChannels)
		return txErr
	})
	if err != nil {
		return fmt.Errorf("failed to create notification with deliveries: %w", err)
	}

	// Attempt real-time delivery for each channel
	for _, ch := range allowedChannels {
		switch ch {
		case ChannelInApp:
			s.deliverInApp(ctx, notificationID, event)
		case ChannelEmail:
			// Publish to email subject for email consumer to process
			emailEvent := map[string]interface{}{
				"notification_id": notificationID,
				"recipient_id":    event.RecipientID,
				"event_type":      event.EventType,
				"payload":         event.Payload,
			}
			if err := s.events.Publish(ctx, SubjectNotificationEmail, emailEvent); err != nil {
				log.Printf("Failed to publish email event for notification %s: %v", notificationID, err)
				_ = s.repo.MarkDeliveryFailed(ctx, notificationID, ChannelEmail, "failed to publish to NATS")
			} else {
				log.Printf("Email delivery queued for notification %s", notificationID)
			}
		case ChannelPush:
			// Publish to push subject for push consumer to process
			pushEvent := map[string]interface{}{
				"notification_id": notificationID,
				"recipient_id":    event.RecipientID,
				"event_type":      event.EventType,
				"payload":         event.Payload,
			}
			if err := s.events.Publish(ctx, SubjectNotificationPush, pushEvent); err != nil {
				log.Printf("Failed to publish push event for notification %s: %v", notificationID, err)
				_ = s.repo.MarkDeliveryFailed(ctx, notificationID, ChannelPush, "failed to publish to NATS")
			} else {
				log.Printf("Push delivery queued for notification %s", notificationID)
			}
		}
	}

	return nil
}

// deliverInApp attempts to deliver notification via WebSocket
func (s *service) deliverInApp(ctx context.Context, notificationID uuid.UUID, event NotificationEvent) {
	if s.notifier == nil {
		log.Printf("Notifier hub not available for in-app delivery")
		_ = s.repo.MarkDeliveryFailed(
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

	if s.notifier.Push(event.RecipientID, push) {
		_ = s.repo.MarkDeliveryDelivered(ctx, notificationID, ChannelInApp)
		log.Printf("In-app notification delivered: %s", notificationID)
	} else {
		_ = s.repo.MarkDeliveryFailed(
			ctx,
			notificationID,
			ChannelInApp,
			"no active WebSocket clients",
		)
		log.Printf("In-app notification failed (no active clients): %s", notificationID)
	}
}

// StartEmailConsumer starts consuming email delivery events
func (s *service) StartEmailConsumer(ctx context.Context) error {
	if s.events == nil {
		return fmt.Errorf("events system not configured")
	}

	log.Println("Starting email delivery consumer...")

	return s.events.Consume(
		ctx,
		StreamNotification,
		ConsumerNotificationEmail,
		SubjectNotificationEmail,
		s.handleEmailDelivery,
	)
}

// handleEmailDelivery processes email delivery events
func (s *service) handleEmailDelivery(msg jetstream.Msg) error {
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
	// For now, just mark as delivered
	// err := emailService.Send(...)
	// if err != nil {
	//     return fmt.Errorf("failed to send email: %w", err)
	// }

	if err := s.repo.MarkDeliveryDelivered(ctx, notificationID, ChannelEmail); err != nil {
		return fmt.Errorf("failed to mark email as delivered: %w", err)
	}

	return nil
}

// StartPushConsumer starts consuming push notification delivery events
func (s *service) StartPushConsumer(ctx context.Context) error {
	if s.events == nil {
		return fmt.Errorf("events system not configured")
	}

	log.Println("Starting push notification consumer...")

	return s.events.Consume(
		ctx,
		StreamNotification,
		ConsumerNotificationPush,
		SubjectNotificationPush,
		s.handlePushDelivery,
	)
}

// handlePushDelivery processes push notification delivery events
func (s *service) handlePushDelivery(msg jetstream.Msg) error {
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
	// err := pushService.Send(...)
	// if err != nil {
	//     return fmt.Errorf("failed to send push notification: %w", err)
	// }

	if err := s.repo.MarkDeliveryDelivered(ctx, notificationID, ChannelPush); err != nil {
		return fmt.Errorf("failed to mark push as delivered: %w", err)
	}

	return nil
}

// PreferenceSetting implements [Service].
func (s *service) PreferenceSetting(ctx context.Context, userID uuid.UUID, entityID uuid.UUID, entityType string) error {
	var p NotificationPreference
	p.UserID = userID
	p.EntityID = entityID
	p.EntityType = entityType
	p.Channels = NotificationChannels{
		string(ChannelInApp): true,
		string(ChannelEmail): false,
		string(ChannelPush):  false,
	}
	p.EventType = NotificationEventTypes{
		EventNewTransaction,
		EventAccountantActivityAlert,
		EventSystemActivityAlert,
	}
	p.CreatedAt = time.Now()

	err := s.repo.CreatePreference(ctx, p)
	if err != nil {
		return fmt.Errorf("failed to create preference: %w", err)
	}

	return nil
}
