package contact

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IService interface {
	Create(ctx context.Context, rq RqContact) (RsContact, error)
	Update(ctx context.Context, contact RqUpdateContact) error
	Delete(ctx context.Context, Id uuid.UUID) error
	Get(ctx context.Context, Id uuid.UUID) (RsContact, error)
	List(ctx context.Context, clinicID uuid.UUID, filter *Filter) (*util.RsList, error)

	DeleteAddressByID(ctx context.Context, Id uuid.UUID) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) IService {
	return &service{repo: repo}
}

// Create implements [Service].
func (s *service) Create(ctx context.Context, rq RqContact) (RsContact, error) {
	contact := rq.ToContact()
	if err := validatePrimaryAddress(contact.Address); err != nil {
		return RsContact{}, err
	}

	contact, err := s.repo.Create(ctx, contact)
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
func (s *service) List(ctx context.Context, clinicID uuid.UUID, filter *Filter) (*util.RsList, error) {
	ft := filter.MapToFilter()

	contacts, err := s.repo.List(ctx, clinicID)
	if err != nil {
		return nil, err
	}

	var rsContacts []RsContact
	for _, contact := range contacts {
		rsContacts = append(rsContacts, contact.ToRsContact())
	}

	var rsList util.RsList

	rsList.MapToList(rsContacts, len(rsContacts), *ft.Offset, *ft.Limit)
	return &rsList, nil
}

// Update implements [Service].
func (s *service) Update(ctx context.Context, contact RqUpdateContact) error {
	existing, err := s.repo.Get(ctx, contact.ID)
	if err != nil {
		return err
	}

	updated := contact.ApplyToContact(existing)
	if err := validatePrimaryAddress(updated.Address); err != nil {
		return err
	}

	return s.repo.Update(ctx, updated)
}

// DeleteAddressByID implements [IService].
func (s *service) DeleteAddressByID(ctx context.Context, Id uuid.UUID) error {
	return s.repo.DeleteAddressByID(ctx, Id)
}

func validatePrimaryAddress(addresses []*Address) error {
	primaryCount := 0
	for _, addr := range addresses {
		if addr.IsPrimary {
			primaryCount++
		}
	}
	if primaryCount > 1 {
		return errors.New("only one primary address is allowed")
	}
	return nil
}
