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

/*
	func (s *service) ListUsers(ctx context.Context) ([]RsAccountantUser, error) {
		userIDInterface := ctx.Value(util.UserIDKey)
		userID, ok := userIDInterface.(string)
		if !ok {
			fmt.Printf("Context Keys: %+v\n", ctx)
			return nil, fmt.Errorf("userID not found in context")
		}
		return s.repo.GetAllUsers(ctx, userID)
	}
*/
func (s *service) ListUsers(ctx context.Context) ([]RsAccountantUser, error) {
	// 1. Grab the ID from the context (The ID starting with 9559fd60...)
	// This value is the Accountant's unique ID from the token
	userIDInterface := ctx.Value(util.PractitionerIDKey)

	var accountantID string
	switch v := userIDInterface.(type) {
	case string:
		accountantID = v
	case uuid.UUID:
		accountantID = v.String()
	default:
		// If this prints, ensure the Handler is passing 'c' and not 'c.Request.Context()'
		fmt.Printf("DEBUG: Context Value is %v, Type is %T\n", userIDInterface, userIDInterface)
		return nil, fmt.Errorf("authentication failed: accountant identity not found")
	}

	// 2. Direct Repository Call
	// We bypass GetAccountantByUserID because the ID in the context is the
	// Accountant ID itself, which the repository needs for the JOIN query.
	users, err := s.repo.GetAllUsers(ctx, accountantID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch clinics: %v", err)
	}

	return users, nil
}
