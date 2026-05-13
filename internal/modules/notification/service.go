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
	"github.com/nats-io/nats.go/jetstream"
)

type Service interface {
	Publish(ctx context.Context, rq RqNotification) error
	List(ctx context.Context, recipientID uuid.UUID, filter FilterNotification) (RsListNotification, error)
	MarkRead(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error
	MarkAllRead(ctx context.Context, recipientID uuid.UUID) error
	MarkDismissed(ctx context.Context, ids []uuid.UUID, recipientID uuid.UUID) error
	GetPreferences(ctx context.Context, userID uuid.UUID) ([]NotificationPreference, error)
	UpdatePreference(ctx context.Context, userID, entityID uuid.UUID, role string, rq RqUpdatePreference) error
}

type service struct {
	repo     Repository
	notifier *sharednotification.Hub
	events   sharedEvents.IEvent
}

func NewService(repo Repository, notifier *sharednotification.Hub, events sharedEvents.IEvent) Service {
	return &service{
		repo:     repo,
		notifier: notifier,
		events:   events,
	}
}

// Publish publishes notification asynchronously via NATS (recommended)
func (s *service) Publish(ctx context.Context, rq RqNotification) error {
	if err := s.events.Publish(ctx, SubjectNotificationCreate, rq); err != nil {
		return err
	}

	return nil
}

func (s *service) List(ctx context.Context, recipientID uuid.UUID, filter FilterNotification) (RsListNotification, error) {
	notifications, total, page, limit, err := s.repo.ListByRecipient(ctx, recipientID, filter)
	if err != nil {
		return RsListNotification{}, err
	}

	// Get the GLOBAL unread count
	unread := 0
	unread, err = s.repo.GetUnreadCount(ctx, recipientID)
	if err != nil {
		fmt.Printf("Error in count notifications: %s", err)
	}

	return RsListNotification{
		Notifications: notifications,
		UnreadCount:   unread,
		Total:         total,
		Page:          page,
		Limit:         limit,
	}, nil
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
	return s.repo.UpsertPreference(ctx, pref)
}

func (s *service) filterChannels(ctx context.Context, prefs *NotificationPreference, rq RqNotification) ([]Channel, int, error) {
	var allowedChannels []Channel

	for _, ch := range rq.Channels {
		if err := prefs.Channels.Scan(ch); err != nil {
			return nil, 0, err
		}
		allowedChannels = append(allowedChannels, ch)
	}
	return allowedChannels, len(allowedChannels), nil
}

// PublishNotification publishes a notification creation event to NATS
func (s *service) PublishNotification(ctx context.Context, rq RqNotification) error {
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

	if err := s.events.Publish(ctx, SubjectNotificationCreate, event); err != nil {
		return fmt.Errorf("failed to publish notification event: %w", err)
	}

	return nil
}

// PublishEmailDelivery publishes an email delivery event
func (s *service) PublishEmailDelivery(ctx context.Context, notificationID, recipientID uuid.UUID, payload json.RawMessage) error {
	event := map[string]interface{}{
		"notification_id": notificationID,
		"recipient_id":    recipientID,
		"payload":         payload,
		"timestamp":       time.Now(),
	}

	if err := s.events.Publish(ctx, SubjectNotificationEmail, event); err != nil {
		return fmt.Errorf("failed to publish email delivery event: %w", err)
	}

	return nil
}

// PublishPushDelivery publishes a push notification delivery event
func (s *service) PublishPushDelivery(ctx context.Context, notificationID, recipientID uuid.UUID, payload json.RawMessage) error {
	event := map[string]interface{}{
		"notification_id": notificationID,
		"recipient_id":    recipientID,
		"payload":         payload,
		"timestamp":       time.Now(),
	}

	if err := s.events.Publish(ctx, SubjectNotificationPush, event); err != nil {
		return fmt.Errorf("failed to publish push delivery event: %w", err)
	}

	return nil
}

// StartNotificationCreateConsumer starts consuming notification creation events
func (s *service) StartNotificationCreateConsumer(ctx context.Context) error {
	log.Println("Starting notification create consumer...")

	return s.events.Consume(
		ctx,
		StreamNotifications,
		ConsumerCreate,
		SubjectNotificationCreate,
		s.handleNotificationCreate,
	)
}

// handleNotificationCreate processes notification creation events
func (s *service) handleNotificationCreate(ctx context.Context, msg jetstream.Msg) error {
	var event NotificationEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		log.Printf("Failed to unmarshal notification event: %v", err)
		return err
	}

	log.Printf("Processing notification: %s for recipient: %s", event.ID, event.RecipientID)

	// Fetch preferences for the recipient
	prefs, err := s.repo.GetPreference(
		ctx,
		event.RecipientID,
		event.RecipientType,
		NotificationEventType(event.EventType),
	)
	if err != nil {
		log.Printf("Failed to fetch preferences (using defaults): %v", err)
		// Continue with default channels if preferences not found
		return err
	}

	// Filter channels based on preferences
	allowedChannels, len, err := s.filterChannels(ctx, prefs, event)
	if err != nil {
		log.Printf("Failed to filter channels: %v", err)
		return err
	}
	if len == 0 {
		log.Printf("No channels allowed for notification %s, skipping", event.ID)
		return nil // Successfully processed (user opted out)
	}

	// Create notification in database
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

	notificationID, err := s.repo.CreateNotification(ctx, notification)
	if err != nil {
		log.Printf("Failed to create notification: %v", err)
		return err
	}

	// Create delivery records
	if err := s.repo.CreateDeliveries(ctx, notificationID, allowedChannels); err != nil {
		log.Printf("Failed to create deliveries: %v", err)
		return err
	}

	// Attempt real-time delivery for each channel
	for _, ch := range allowedChannels {
		switch ch {
		case ChannelInApp:
			s.deliverInApp(ctx, notificationID, event)
		case ChannelEmail:
			// Publish to email worker queue
			log.Printf("Email delivery for notification %s (implement email worker)", notificationID)
		case ChannelPush:
			// Publish to push notification worker queue
			log.Printf("Push delivery for notification %s (implement push worker)", notificationID)
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
	log.Println("Starting email delivery consumer...")

	return s.events.Consume(
		ctx,
		StreamNotifications,
		ConsumerEmail,
		SubjectNotificationEmail,
		s.handleEmailDelivery,
	)
}

// handleEmailDelivery processes email delivery events
func (s *service) handleEmailDelivery(ctx context.Context, cmsg jetstream.Msg) error {
	var event map[string]interface{}
	if err := json.Unmarshal(cmsg.Data(), &event); err != nil {
		log.Printf("Failed to unmarshal email event: %v", err)
		return err
	}

	notificationID, _ := uuid.Parse(event["notification_id"].(string))

	// TODO: Implement actual email sending logic here
	// For now, just mark as delivered
	log.Printf("Processing email delivery for notification: %s", notificationID)

	// Simulate email sending
	// err := emailService.Send(...)
	// if err != nil {
	//     return err
	// }

	return s.repo.MarkDeliveryDelivered(ctx, notificationID, ChannelEmail)
}

// StartPushConsumer starts consuming push notification delivery events
func (s *service) StartPushConsumer(ctx context.Context) error {
	log.Println("Starting push notification consumer...")

	return s.events.Consume(
		ctx,
		StreamNotifications,
		ConsumerPush,
		SubjectNotificationPush,
		s.handlePushDelivery,
	)
}

// handlePushDelivery processes push notification delivery events
func (s *service) handlePushDelivery(ctx context.Context, msg jetstream.Msg) error {
	var event map[string]interface{}
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		log.Printf("Failed to unmarshal push event: %v", err)
		return err
	}

	notificationID, _ := uuid.Parse(event["notification_id"].(string))

	// TODO: Implement actual push notification logic here (FCM, APNs, etc.)
	log.Printf("Processing push delivery for notification: %s", notificationID)

	// Simulate push sending
	// err := pushService.Send(...)
	// if err != nil {
	//     return err
	// }

	return s.repo.MarkDeliveryDelivered(ctx, notificationID, ChannelPush)
}
