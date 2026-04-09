package invitation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRepository is a mock implementation of Repository
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) GetByID(ctx context.Context, id uuid.UUID) (*Invitation, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Invitation), args.Error(1)
}

func (m *MockRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status InvitationStatus, entityID *uuid.UUID) error {
	args := m.Called(ctx, id, status, entityID)
	return args.Error(0)
}

func (m *MockRepository) GetAccountantIDByEmail(ctx context.Context, email string) (*uuid.UUID, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*uuid.UUID), args.Error(1)
}

// Add other required methods as stubs
func (m *MockRepository) Create(ctx context.Context, inv *Invitation) error { return nil }
func (m *MockRepository) GetByEmail(ctx context.Context, email string) (*Invitation, error) { return nil, nil }
func (m *MockRepository) GetPractitionerName(ctx context.Context, practitionerID uuid.UUID) (string, error) { return "", nil }
func (m *MockRepository) GetUserIDByEmail(ctx context.Context, email string) (*uuid.UUID, error) { return nil, nil }
func (m *MockRepository) List(ctx context.Context, f common.Filter) ([]*Invitation, error) { return nil, nil }
func (m *MockRepository) Count(ctx context.Context, f common.Filter) (int, error) { return 0, nil }
func (m *MockRepository) GetInvitationByID(ctx context.Context, id uuid.UUID) (*InvitationExtended, error) { return nil, nil }
func (m *MockRepository) GetUserDetailsByEmail(ctx context.Context, email string) (*UserDetails, error) { return nil, nil }
func (m *MockRepository) CountDailyInvitesByEmail(ctx context.Context, practitionerID uuid.UUID, email string) (int, error) { return 0, nil }
func (m *MockRepository) GetEmailByAccountantID(ctx context.Context, accountantID uuid.UUID) (string, error) { return "", nil }
func (m *MockRepository) ListByEmail(ctx context.Context, email string, f common.Filter) ([]*Invitation, error) { return nil, nil }
func (m *MockRepository) CountByEmail(ctx context.Context, email string, f common.Filter) (int, error) { return 0, nil }
func (m *MockRepository) GetPermissions(ctx context.Context, accountantID uuid.UUID, entityID uuid.UUID) (*Permissions, error) { return nil, nil }
func (m *MockRepository) GrantEntityPermissionTx(ctx context.Context, tx *sqlx.Tx, practitionerID, accountantID, entityID uuid.UUID, entityType string, perms Permissions) error { return nil }
func (m *MockRepository) DeletePermissionsByEntityTx(ctx context.Context, tx *sqlx.Tx, entityID uuid.UUID) error { return nil }
func (m *MockRepository) GetPractitionerLinkedToAccountant(ctx context.Context, accountantID uuid.UUID) (uuid.UUID, error) { return uuid.Nil, nil }
func (m *MockRepository) GrantEntityPermission(ctx context.Context, pID, aID, eID uuid.UUID, eType string, permJson []byte) error { return nil }
func (m *MockRepository) DeleteAllPermissionsForAccountantTx(ctx context.Context, tx *sqlx.Tx, practitionerID, accountantID uuid.UUID) error { return nil }
func (m *MockRepository) UpdateStatusTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, status InvitationStatus, entityID *uuid.UUID) error { return nil }
func (m *MockRepository) ListAccountantPermissions(ctx context.Context, accountantID uuid.UUID, f common.Filter) ([]AccountantPermissionRow, error) { return nil, nil }
func (m *MockRepository) CountAccountantPermissions(ctx context.Context, accountantID uuid.UUID, f common.Filter) (int, error) { return 0, nil }

// TestProcessInvitation_ExpiredInvitation tests that expired invitations are rejected
func TestProcessInvitation_ExpiredInvitation(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := &service{
		repo:         mockRepo,
		inviteConfig: InviteDefaultConfig(),
	}

	inviteID := uuid.New()
	expiredInvite := &Invitation{
		ID:        inviteID,
		Status:    StatusSent,
		ExpiresAt: time.Now().Add(-24 * time.Hour), // Expired yesterday
	}

	mockRepo.On("GetByID", mock.Anything, inviteID).Return(expiredInvite, nil)

	req := &RqProcessAction{
		TokenID: inviteID,
		Action:  ActionAccept,
	}

	_, err := svc.ProcessInvitation(context.Background(), req)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvitationExpired))
	mockRepo.AssertExpectations(t)
}

// TestProcessInvitation_InvalidatedInvitation tests that resent invitations cannot be used
func TestProcessInvitation_InvalidatedInvitation(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := &service{
		repo:         mockRepo,
		inviteConfig: InviteDefaultConfig(),
	}

	inviteID := uuid.New()
	resentInvite := &Invitation{
		ID:        inviteID,
		Status:    StatusResent,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	mockRepo.On("GetByID", mock.Anything, inviteID).Return(resentInvite, nil)

	req := &RqProcessAction{
		TokenID: inviteID,
		Action:  ActionAccept,
	}

	_, err := svc.ProcessInvitation(context.Background(), req)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvitationInvalidated))
	mockRepo.AssertExpectations(t)
}

// TestProcessInvitation_AlreadyProcessed tests that completed invitations cannot be reused
func TestProcessInvitation_AlreadyProcessed(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := &service{
		repo:         mockRepo,
		inviteConfig: InviteDefaultConfig(),
	}

	inviteID := uuid.New()
	completedInvite := &Invitation{
		ID:        inviteID,
		Status:    StatusCompleted,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	mockRepo.On("GetByID", mock.Anything, inviteID).Return(completedInvite, nil)

	req := &RqProcessAction{
		TokenID: inviteID,
		Action:  ActionAccept,
	}

	_, err := svc.ProcessInvitation(context.Background(), req)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvitationAlreadyUsed))
	mockRepo.AssertExpectations(t)
}

// TestProcessInvitation_InvalidAction tests that invalid actions are rejected
func TestProcessInvitation_InvalidAction(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := &service{
		repo:         mockRepo,
		inviteConfig: InviteDefaultConfig(),
	}

	inviteID := uuid.New()
	validInvite := &Invitation{
		ID:        inviteID,
		Status:    StatusSent,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	mockRepo.On("GetByID", mock.Anything, inviteID).Return(validInvite, nil)

	req := &RqProcessAction{
		TokenID: inviteID,
		Action:  "INVALID_ACTION",
	}

	_, err := svc.ProcessInvitation(context.Background(), req)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidAction))
	mockRepo.AssertExpectations(t)
}

// TestProcessInvitation_SuccessfulAccept tests successful invitation acceptance
func TestProcessInvitation_SuccessfulAccept(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := &service{
		repo:         mockRepo,
		inviteConfig: InviteDefaultConfig(),
		auditSvc:     nil, // Audit service not needed for this test
	}

	inviteID := uuid.New()
	practitionerID := uuid.New()
	validInvite := &Invitation{
		ID:             inviteID,
		PractitionerID: practitionerID,
		Email:          "accountant@example.com",
		Status:         StatusSent,
		ExpiresAt:      time.Now().Add(24 * time.Hour),
	}

	mockRepo.On("GetByID", mock.Anything, inviteID).Return(validInvite, nil)
	mockRepo.On("GetAccountantIDByEmail", mock.Anything, "accountant@example.com").Return(nil, nil)
	mockRepo.On("UpdateStatus", mock.Anything, inviteID, StatusAccepted, (*uuid.UUID)(nil)).Return(nil)

	req := &RqProcessAction{
		TokenID: inviteID,
		Action:  ActionAccept,
	}

	result, err := svc.ProcessInvitation(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, StatusAccepted, result.Status)
	assert.False(t, result.IsFound)
	mockRepo.AssertExpectations(t)
}

// TestRevokeInvite_NotFound tests revoking non-existent invitation
func TestRevokeInvite_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := &service{
		repo:         mockRepo,
		inviteConfig: InviteDefaultConfig(),
	}

	inviteID := uuid.New()
	practitionerID := uuid.New()

	mockRepo.On("GetByID", mock.Anything, inviteID).Return(nil, errors.New("not found"))

	err := svc.RevokeInvite(context.Background(), practitionerID, inviteID)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvitationNotFound))
	mockRepo.AssertExpectations(t)
}

// TestRevokeInvite_Unauthorized tests revoking invitation by wrong practitioner
func TestRevokeInvite_Unauthorized(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := &service{
		repo:         mockRepo,
		inviteConfig: InviteDefaultConfig(),
	}

	inviteID := uuid.New()
	practitionerID := uuid.New()
	wrongPractitionerID := uuid.New()

	invite := &Invitation{
		ID:             inviteID,
		PractitionerID: practitionerID,
		Status:         StatusCompleted,
	}

	mockRepo.On("GetByID", mock.Anything, inviteID).Return(invite, nil)

	err := svc.RevokeInvite(context.Background(), wrongPractitionerID, inviteID)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnauthorizedInvite))
	mockRepo.AssertExpectations(t)
}
