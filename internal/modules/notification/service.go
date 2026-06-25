package notification

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/notification/preference"
	sharedEvents "github.com/iamarpitzala/acareca/internal/shared/events"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type Service interface {
	Publish(ctx context.Context, rq RqNotification) error
	PublishEvent(ctx context.Context, subject string, event interface{}) error
	List(ctx context.Context, recipientID uuid.UUID, filter FilterNotification) (*util.RsList, error)
	MarkRead(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error
	MarkAllRead(ctx context.Context, recipientID uuid.UUID) error
	MarkDismissed(ctx context.Context, ids []uuid.UUID, recipientID uuid.UUID) error
	GetPreferences(ctx context.Context, userID uuid.UUID) ([]preference.Preference, error)
}

type service struct {
	repo     Repository
	events   sharedEvents.IEvent
	DB       *sqlx.DB
	PrefRepo preference.Repository
}

func NewService(repo Repository, events sharedEvents.IEvent, db *sqlx.DB, prefRepo preference.Repository) Service {
	return &service{
		repo:     repo,
		events:   events,
		DB:       db,
		PrefRepo: prefRepo,
	}
}

func (s *service) Publish(ctx context.Context, rq RqNotification) error {
	if s.events == nil {
		return fmt.Errorf("events system not configured")
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

// PublishEvent publishes an event to the specified subject
func (s *service) PublishEvent(ctx context.Context, subject string, event interface{}) error {
	if s.events == nil {
		return fmt.Errorf("events system not configured")
	}

	if err := s.events.Publish(ctx, subject, event); err != nil {
		return fmt.Errorf("failed to publish event to %s: %w", subject, err)
	}

	return nil
}

func (s *service) List(ctx context.Context, recipientID uuid.UUID, filter FilterNotification) (*util.RsList, error) {
	notifications, total, err := s.repo.ListByRecipient(ctx, recipientID, filter)
	if err != nil {
		return nil, err
	}

	unreadCount := 0
	unreadCount, err = s.repo.GetUnreadCount(ctx, recipientID)
	if err != nil {
		log.Printf("Error in count notifications: %s", err)
	}

	limit := 10
	if filter.Limit != nil && *filter.Limit > 0 {
		limit = *filter.Limit
	}

	offset := 0
	if filter.Offset != nil && *filter.Offset >= 0 {
		offset = *filter.Offset
	}

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

func (s *service) GetPreferences(ctx context.Context, userID uuid.UUID) ([]preference.Preference, error) {
	return s.PrefRepo.GetAllPreferences(ctx, userID)
}
