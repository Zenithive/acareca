package coa

import (
	"context"

	"github.com/google/uuid"
)

type IService interface {
	Create(ctx context.Context, rq RqAccountTemplate) error
	Update(ctx context.Context, rq RqUpdateAccountTemplate) error
	Delete(ctx context.Context, id uuid.UUID, adminID uuid.UUID) error
	GetById(ctx context.Context, id uuid.UUID) (RsAccountTemplate, error)
	List(ctx context.Context) ([]RsAccountTemplate, error)
}

type service struct {
	repo IRepo
}

func NewService(repo IRepo) IService {
	return &service{
		repo: repo,
	}
}

func (s *service) Create(ctx context.Context, rq RqAccountTemplate) error {
	if rq.Key == "" {
		rq.Key = GenerateKeyFromName(rq.Name)
	}

	dbAccount := rq.ToDB()
	if err := s.repo.Create(ctx, dbAccount); err != nil {
		return err
	}

	return s.repo.SeedNewTemplateToAllPractitioners(ctx, dbAccount.ID)
}

func (s *service) Update(ctx context.Context, rq RqUpdateAccountTemplate) error {
	account, err := s.repo.GetByID(ctx, rq.ID)
	if err != nil {
		return err
	}

	rq.ApplyTo(&account)

	if rq.Name != nil {
		account.Key = GenerateKeyFromName(*rq.Name)
	}

	if rq.Key != "" {
		account.Key = rq.Key
	}

	return s.repo.Update(ctx, account)
}

func (s *service) Delete(ctx context.Context, id uuid.UUID, adminID uuid.UUID) error {
	return s.repo.Delete(ctx, id, adminID)
}

func (s *service) GetById(ctx context.Context, id uuid.UUID) (RsAccountTemplate, error) {
	account, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return RsAccountTemplate{}, err
	}

	return account.ToResponse(), nil
}

func (s *service) List(ctx context.Context) ([]RsAccountTemplate, error) {
	accounts, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	return ToResponses(accounts), nil
}
