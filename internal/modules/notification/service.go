package notification

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	sharedEvents "github.com/iamarpitzala/acareca/internal/shared/events"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type Service interface {
	Publish(ctx context.Context, rq RqNotification) error
	List(ctx context.Context, recipientID uuid.UUID, filter FilterNotification) (*util.RsList, error)
	MarkRead(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error
	MarkAllRead(ctx context.Context, recipientID uuid.UUID) error
	MarkDismissed(ctx context.Context, ids []uuid.UUID, recipientID uuid.UUID) error
	GetPreferences(ctx context.Context, userID uuid.UUID) ([]NotificationPreference, error)
	UpdatePreference(ctx context.Context, userID, entityID uuid.UUID, role string, rq RqUpdatePreference) error
	PreferenceSetting(ctx context.Context, tx *sqlx.Tx, userID uuid.UUID, entityID uuid.UUID, entityType string) error
}

type service struct {
	repo      Repository
	publisher *Publisher
	DB        *sqlx.DB
}

func NewService(repo Repository, events sharedEvents.IEvent, db *sqlx.DB) Service {
	return &service{
		repo:      repo,
		publisher: NewPublisher(events),
		DB:        db,
	}
}

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
	fmt.Printf("Publishing notification event_Channel: %+v\n", event.Channels)
	fmt.Printf("Publishing notification event_Type: %+v\n", event.EventType)
	fmt.Printf("Publishing notification event_ID: %+v\n", event.ID)
	fmt.Printf("Publishing notification event_Entity_Id: %+v\n", event.EntityID)
	fmt.Printf("Publishing notification event_Recipient_Id: %+v\n", event.RecipientID)
	return s.publisher.PublishNotification(ctx, event)
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

	if !rq.Channels[string(ChannelInApp)] &&
		!rq.Channels[string(ChannelEmail)] &&
		!rq.Channels[string(ChannelPush)] {
		return errors.New("at least one notification channel must be enabled")
	}

	err := util.RunInTransaction(ctx, s.DB, func(ctx context.Context, tx *sqlx.Tx) error {

		for _, event := range rq.EventType {

			// optional logs
			if rq.Channels[string(ChannelInApp)] {
				fmt.Println("InApp enabled")
			}
			if rq.Channels[string(ChannelEmail)] {
				fmt.Println("Email enabled")
			}
			if rq.Channels[string(ChannelPush)] {
				fmt.Println("Push enabled")
			}

			pref := NotificationPreference{
				UserID:     userID,
				EntityID:   entityID,
				EntityType: role,
				EventType:  event,
				Channels:   rq.Channels,
			}

			if err := s.repo.CreatePreference(ctx, pref, tx); err != nil {
				return fmt.Errorf("failed to create preference: %w", err)
			}
		}

		return nil
	})

	return err
}

// PreferenceSetting creates default notification preferences for a new user (one row per event type)
func (s *service) PreferenceSetting(ctx context.Context, tx *sqlx.Tx, userID uuid.UUID, entityID uuid.UUID, entityType string) error {

	// chack if preferences alredy exist for this user and entity
	existingPrefs, err := s.GetPreferences(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get existing preferences: %w", err)
	}
	if len(existingPrefs) == 0 {
		defaultChannels := NotificationChannels{
			string(ChannelInApp): true,
			string(ChannelEmail): false,
			string(ChannelPush):  false,
		}
		eventTypes := []NotificationEventType{
			EventNewTransaction,
			EventAccountantActivityAlert,
			EventSystemActivityAlert,
		}
		for _, et := range eventTypes {
			pref := NotificationPreference{
				UserID:     userID,
				EntityID:   entityID,
				EntityType: entityType,
				EventType:  et,
				Channels:   defaultChannels,
				CreatedAt:  time.Now(),
			}
			if err := s.repo.CreatePreference(ctx, pref, tx); err != nil {
				return fmt.Errorf("failed to create preference: %w", err)
			}
		}
	}

	return nil
}
