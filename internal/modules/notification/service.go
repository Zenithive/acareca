package notification

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
)

type Service interface {
	Publish(ctx context.Context, rq RqNotification) error
	List(ctx context.Context, recipientID uuid.UUID, filter FilterNotification) (RsListNotification, error)
	MarkRead(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error
	MarkDismissed(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error
}

type service struct {
	repo     Repository
	notifier *sharednotification.Hub
}

func NewService(repo Repository, notifier *sharednotification.Hub) Service {
	return &service{repo: repo, notifier: notifier}
}

func (s *service) Publish(ctx context.Context, rq RqNotification) error {
	notificationID, err := s.repo.CreateNotification(ctx, rq.MapToDB())
	if err != nil {
		return err
	}

	channels := rq.Channels
	if len(channels) == 0 {
		channels = []Channel{ChannelInApp}
	}
	if err := s.repo.CreateDeliveries(ctx, notificationID, channels); err != nil {
		return err
	}

	// Attempt in_app delivery via WebSocket and update delivery status
	for _, ch := range channels {
		if ch == ChannelInApp && s.notifier != nil {
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
		// push / email channels: delivery workers handle those separately
	}
	return nil
}

func (s *service) List(ctx context.Context, recipientID uuid.UUID, filter FilterNotification) (RsListNotification, error) {
	notifications, total, err := s.repo.ListByRecipient(ctx, recipientID, filter)
	if err != nil {
		return RsListNotification{}, err
	}

	unread := 0
	for _, n := range notifications {
		if n.Status == StatusUnread {
			unread++
		}
	}

	return RsListNotification{
		Notifications: notifications,
		UnreadCount:   unread,
		Total:         total,
	}, nil
}

func (s *service) MarkRead(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error {
	return s.repo.MarkRead(ctx, id, recipientID)
}

func (s *service) MarkDismissed(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error {
	return s.repo.MarkDismissed(ctx, id, recipientID)
}
