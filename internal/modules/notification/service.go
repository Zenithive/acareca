package notification

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	sharedEvents "github.com/iamarpitzala/acareca/internal/shared/events"
	"github.com/iamarpitzala/acareca/internal/shared/util"
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
	PreferenceSetting(ctx context.Context, userID uuid.UUID, entityID uuid.UUID, entityType string) error
}

type service struct {
	repo      Repository
	publisher *Publisher
}

func NewService(repo Repository, events sharedEvents.IEvent) Service {
	return &service{
		repo:      repo,
		publisher: NewPublisher(events),
	}
}

// Publish publishes notification asynchronously via NATS
func (s *service) Publish(ctx context.Context, rq RqNotification) error {
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

	return s.publisher.PublishNotification(ctx, event)
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

// PreferenceSetting creates default notification preferences for a new user
func (s *service) PreferenceSetting(ctx context.Context, userID uuid.UUID, entityID uuid.UUID, entityType string) error {
	pref := NotificationPreference{
		UserID:     userID,
		EntityID:   entityID,
		EntityType: entityType,
		Channels: NotificationChannels{
			string(ChannelInApp): true,
			string(ChannelEmail): false,
			string(ChannelPush):  false,
		},
		EventType: NotificationEventTypes{
			EventNewTransaction,
			EventAccountantActivityAlert,
			EventSystemActivityAlert,
		},
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreatePreference(ctx, pref); err != nil {
		return fmt.Errorf("failed to create preference: %w", err)
	}

	return nil
}
