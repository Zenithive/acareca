package notification

import (
	"context"

	"github.com/google/uuid"
)

type Service interface {
	Publish(ctx context.Context, rq RqNotification) error
	List(ctx context.Context, recipientID uuid.UUID, filter FilterNotification) (RsListNotification, error)
	MarkRead(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error
	MarkDismissed(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Publish(ctx context.Context, rq RqNotification) error {
	return s.repo.CreateNotification(ctx, rq.MapToDB())
}

func (s *service) List(ctx context.Context, recipientID uuid.UUID, filter FilterNotification) (RsListNotification, error) {
	notifications, total, err := s.repo.ListByRecipient(ctx, recipientID, filter)
	if err != nil {
		return RsListNotification{}, err
	}

	unread := 0
	for _, n := range notifications {
		if n.Status == StatusPending || n.Status == StatusDelivered {
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
