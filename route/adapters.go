package route

import (
	"context"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/auth"
	"github.com/iamarpitzala/acareca/internal/modules/builder/form"
	"github.com/iamarpitzala/acareca/internal/modules/file"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

// FileAuthServiceAdapter adapts auth.Service to file.AuthService interface
type FileAuthServiceAdapter struct {
	authSvc auth.Service
}

// NewFileAuthServiceAdapter creates a new adapter for file service
func NewFileAuthServiceAdapter(authSvc auth.Service) file.AuthService {
	return &FileAuthServiceAdapter{authSvc: authSvc}
}

// GetUserByID implements file.AuthService
func (a *FileAuthServiceAdapter) GetUserByID(ctx context.Context, entityID uuid.UUID, entityType util.ActorType) (*file.UserInfo, error) {
	user, err := a.authSvc.GetUserByID(ctx, entityID, entityType)
	if err != nil {
		return nil, err
	}

	return &file.UserInfo{
		FirstName: user.FirstName,
		LastName:  user.LastName,
	}, nil
}

// FormAuthServiceAdapter adapts auth.Service to form.AuthService interface
type FormAuthServiceAdapter struct {
	authSvc auth.Service
}

// NewFormAuthServiceAdapter creates a new adapter for form service
func NewFormAuthServiceAdapter(authSvc auth.Service) form.AuthService {
	return &FormAuthServiceAdapter{authSvc: authSvc}
}

// GetUserByID implements form.AuthService
func (a *FormAuthServiceAdapter) GetUserByID(ctx context.Context, entityID uuid.UUID, entityType util.ActorType) (*form.AuthUserInfo, error) {
	user, err := a.authSvc.GetUserByID(ctx, entityID, entityType)
	if err != nil {
		return nil, err
	}

	return &form.AuthUserInfo{
		ID:        user.ID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
	}, nil
}
