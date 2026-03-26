package notification

import (
	"context"
)

type Service interface {
	Publish(ctx context.Context, rq RqNotification) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// Publish implements [Service].
func (s *service) Publish(ctx context.Context, rq RqNotification) error {
	notification := rq.MapToDB()
	err := s.repo.CreateNotification(ctx, notification)
	if err != nil {
		return err
	}

	return nil
}
