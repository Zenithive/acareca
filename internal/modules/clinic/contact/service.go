package contact

import (
	"context"

	"github.com/google/uuid"
)

type IService interface {
	Create(ctx context.Context, rq RqContact) (RsContact, error)
	Update(ctx context.Context, contact RqUpdateContact) error
	Delete(ctx context.Context, Id uuid.UUID) error
	Get(ctx context.Context, Id uuid.UUID) (RsContact, error)
	List(ctx context.Context) ([]RsContact, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) IService {
	return &service{repo: repo}
}

// Create implements [Service].
func (s *service) Create(ctx context.Context, rq RqContact) (RsContact, error) {
	contact, err := s.repo.Create(ctx, rq.ToContact())
	if err != nil {
		return RsContact{}, err
	}

	return contact.ToRsContact(), nil
}

// Delete implements [Service].
func (s *service) Delete(ctx context.Context, Id uuid.UUID) error {
	return s.repo.Delete(ctx, Id)
}

// Get implements [Service].
func (s *service) Get(ctx context.Context, Id uuid.UUID) (RsContact, error) {
	contact, err := s.repo.Get(ctx, Id)
	if err != nil {
		return RsContact{}, err
	}
	return contact.ToRsContact(), nil
}

// List implements [Service].
func (s *service) List(ctx context.Context) ([]RsContact, error) {
	contacts, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	var rsContacts []RsContact
	for _, contact := range contacts {
		rsContacts = append(rsContacts, contact.ToRsContact())
	}
	return rsContacts, nil
}

// Update implements [Service].
func (s *service) Update(ctx context.Context, contact RqUpdateContact) error {
	return s.repo.Update(ctx, contact.ToContact())
}
