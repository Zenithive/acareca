package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
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
}

func NewService(repo Repository, notifier *sharednotification.Hub) Service {
	return &service{repo: repo, notifier: notifier}
}

func (s *service) Publish(ctx context.Context, rq RqNotification) error {
	// Fetch preferences for the recipient
	prefs, err := s.repo.GetPreference(ctx, rq.RecipientID, rq.RecipientType, NotificationEventType(rq.EventType))
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			fmt.Printf("Err in fetching preferences: %s\n", err)
		}
	}

	allowedChannels, len, err := s.filterChannels(ctx, prefs, rq)
	if err != nil {
		return err
	}
	if len == 0 {
		return nil // No channels allowed, skip notification
	}

	notificationID, err := s.repo.CreateNotification(ctx, rq.MapToDB())
	if err != nil {
		return err
	}

	if err := s.repo.CreateDeliveries(ctx, notificationID, allowedChannels); err != nil {
		return err
	}

	// Attempt in_app delivery via WebSocket and update delivery status
	for _, ch := range allowedChannels {
		if ch.IsValid() && s.notifier != nil {
			push := map[string]any{
				"id":           notificationID,
				"recipient_id": rq.RecipientID,
				"sender_id":    rq.SenderID,
				"event_type":   rq.EventType,
				"entity_type":  rq.EntityType,
				"entity_id":    rq.EntityID,
				"status":       rq.Status,
				"payload":      json.RawMessage(rq.Payload),
				"created_at":   rq.CreatedAt,
			}
			if s.notifier.Push(rq.RecipientID, push) {
				_ = s.repo.MarkDeliveryDelivered(ctx, notificationID, ChannelInApp)
			} else {
				_ = s.repo.MarkDeliveryFailed(ctx, notificationID, ChannelInApp, "no active WebSocket clients")
			}
		}
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
