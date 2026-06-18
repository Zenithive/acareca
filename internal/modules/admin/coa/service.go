package coa

import (
	"context"

	"github.com/google/uuid"
)

type IService interface {
	Create(ctx context.Context, rq RqAccountTemplate) error
	Update(ctx context.Context, rq RqUpdateAccountTemplate) error
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
	return s.repo.Create(ctx, rq.ToDB())
}

func (s *service) Update(ctx context.Context, rq RqUpdateAccountTemplate) error {
	account, err := s.repo.GetByID(ctx, rq.ID)
	if err != nil {
		return err
	}

	rq.ApplyTo(&account)

	return s.repo.Update(ctx, account)
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
