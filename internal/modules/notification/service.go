package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"
	sharedEvents "github.com/iamarpitzala/acareca/internal/shared/events"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
	"github.com/nats-io/nats.go/jetstream"
)

type Service interface {
	// Core notification operations
	Publish(ctx context.Context, rq RqNotification) error
	List(ctx context.Context, recipientID uuid.UUID, filter FilterNotification) (RsListNotification, error)
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

	if err := s.events.Publish(ctx, SubjectNotificationCreate, event); err != nil {
		return fmt.Errorf("failed to publish notification event: %w", err)
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

// filterChannels returns only the channels that are enabled in user preferences
func (s *service) filterChannels(prefs *NotificationPreference, requestedChannels []Channel) []Channel {
	var allowedChannels []Channel

	for _, ch := range requestedChannels {
		channelKey := string(ch)
		if enabled, exists := prefs.Channels[channelKey]; exists && enabled {
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
func (s *service) handleNotificationCreate(msg jetstream.Msg) error {
	ctx := context.Background()
	
	var event NotificationEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		return fmt.Errorf("failed to unmarshal notification event: %w", err)
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
		log.Printf("Failed to fetch preferences for recipient %s, using default channels: %v", event.RecipientID, err)
		// Use default channels if preferences not found
		prefs = &NotificationPreference{
			Channels: NotificationChannels{
				string(ChannelInApp): true,
			},
		}
	}

	// Filter channels based on preferences
	allowedChannels := s.filterChannels(prefs, event.Channels)
	if len(allowedChannels) == 0 {
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
		return fmt.Errorf("failed to create notification: %w", err)
	}

	// Create delivery records
	if err := s.repo.CreateDeliveries(ctx, notificationID, allowedChannels); err != nil {
		return fmt.Errorf("failed to create deliveries: %w", err)
	}

	// Attempt real-time delivery for each channel
	for _, ch := range allowedChannels {
		switch ch {
		case ChannelInApp:
			s.deliverInApp(ctx, notificationID, event)
		case ChannelEmail:
			log.Printf("Email delivery queued for notification %s", notificationID)
			// Email delivery will be handled by email consumer
		case ChannelPush:
			log.Printf("Push delivery queued for notification %s", notificationID)
			// Push delivery will be handled by push consumer
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
		StreamNotifications,
		ConsumerEmail,
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
		StreamNotifications,
		ConsumerPush,
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
