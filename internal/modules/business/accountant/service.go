package accountant

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type IService interface {
	CreateAccountant(ctx context.Context, req *RqCreateAccountant, tx *sqlx.Tx) (*RsAccountant, error)
	GetAccountantByUserID(ctx context.Context, userID string) (*RsAccountant, error)
	ListUsers(ctx context.Context) ([]RsAccountantUser, error)
	ListClinics(ctx context.Context) ([]ClinicDetail, error)
	ListForms(ctx context.Context) ([]RsAccountantForm, error)
	getAccountantID(ctx context.Context) (string, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) IService {
	return &service{repo: repo}
}

func (s *service) CreateAccountant(ctx context.Context, req *RqCreateAccountant, tx *sqlx.Tx) (*RsAccountant, error) {
	// Check if accountant already exists
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
	// Check if accountant already exists
	existing, err := s.repo.GetAccountantByUserID(ctx, userID)
	if err == nil && existing != nil {
		return existing, nil
	}

	return nil, fmt.Errorf("accountant not found for user ID: %s", userID)
}

func (s *service) ListUsers(ctx context.Context) ([]RsAccountantUser, error) {

	userIDInterface := ctx.Value(util.EntityIDKey)

	var accountantID string
	switch v := userIDInterface.(type) {
	case string:
		accountantID = v
	case uuid.UUID:
		accountantID = v.String()
	default:

		fmt.Printf("DEBUG: Context Value is %v, Type is %T\n", userIDInterface, userIDInterface)
		return nil, fmt.Errorf("authentication failed: accountant identity not found")
	}

	users, err := s.repo.GetAllUsers(ctx, accountantID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch clinics: %v", err)
	}

	return users, nil
}

func (s *service) ListClinics(ctx context.Context) ([]ClinicDetail, error) {
	accID, err := s.getAccountantID(ctx)
	if err != nil {
		return nil, err
	}
	return s.repo.GetClinicsForAccountant(ctx, accID)
}

func (s *service) ListForms(ctx context.Context) ([]RsAccountantForm, error) {
	accID, err := s.getAccountantID(ctx)
	if err != nil {
		return nil, err
	}
	return s.repo.GetFormsForAccountant(ctx, accID)
}

func (s *service) getAccountantID(ctx context.Context) (string, error) {
	val := ctx.Value(util.EntityIDKey)
	switch v := val.(type) {
	case string:
		return v, nil
	case uuid.UUID:
		return v.String(), nil
	default:
		return "", fmt.Errorf("invalid accountant identity in context")
	}
}
