package accountant

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type IService interface {
	CreateAccountant(ctx context.Context, req *RqCreateAccountant, tx *sqlx.Tx) (*RsAccountant, error)
	GetAccountantByUserID(ctx context.Context, userID string) (*RsAccountant, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) IService {
	return &service{repo: repo}
}

func (s *service) CreateAccountant(ctx context.Context, req *RqCreateAccountant, tx *sqlx.Tx) (*RsAccountant, error) {
	// Check if accounatant already exists
	existing, err := s.repo.GetAccountantByUserID(ctx, req.UserID)
	if err == nil && existing != nil {
		return existing, nil
	}

	// Create Accountant and Settings
	t, err := s.repo.CreateAccountant(ctx, req, tx)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (s *service) GetAccountantByUserID(ctx context.Context, userID string) (*RsAccountant, error) {
	// Check if accounatant already exists
	existing, err := s.repo.GetAccountantByUserID(ctx, userID)
	if err == nil && existing != nil {
		return existing, nil
	}

	return nil, fmt.Errorf("accountant not found for user ID: %s", userID)
}
