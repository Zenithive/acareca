package permission

import (
	"context"

	"github.com/google/uuid"
)

type InvitationServiceAdapter struct {
	getPermissions                   func(ctx context.Context, accountantID uuid.UUID, entityID uuid.UUID) (*PermissionSet, error)
	getPermissionsByEmail            func(ctx context.Context, practitionerID uuid.UUID, email string, entityID uuid.UUID) (*PermissionSet, error)
	isAccountantLinkedToPractitioner func(ctx context.Context, practitionerID, accountantID uuid.UUID) (bool, error)
}

func NewInvitationServiceAdapter(
	getPermissions func(ctx context.Context, accountantID uuid.UUID, entityID uuid.UUID) (*PermissionSet, error),
	getPermissionsByEmail func(ctx context.Context, practitionerID uuid.UUID, email string, entityID uuid.UUID) (*PermissionSet, error),
	isAccountantLinkedToPractitioner func(ctx context.Context, practitionerID, accountantID uuid.UUID) (bool, error),
) *InvitationServiceAdapter {
	return &InvitationServiceAdapter{
		getPermissions:                   getPermissions,
		getPermissionsByEmail:            getPermissionsByEmail,
		isAccountantLinkedToPractitioner: isAccountantLinkedToPractitioner,
	}
}

func (a *InvitationServiceAdapter) GetEntityPermissions(accountantID uuid.UUID, entityID uuid.UUID) (*PermissionSet, error) {
	return a.getPermissions(context.Background(), accountantID, entityID)
}

func (a *InvitationServiceAdapter) GetEntityPermissionsByEmail(practitionerID uuid.UUID, email string, entityID uuid.UUID) (*PermissionSet, error) {
	return a.getPermissionsByEmail(context.Background(), practitionerID, email, entityID)
}

func (a *InvitationServiceAdapter) IsAccountantLinkedToPractitioner(practitionerID, accountantID uuid.UUID) (bool, error) {
	return a.isAccountantLinkedToPractitioner(context.Background(), practitionerID, accountantID)
}

func ConvertInvitationPermissions(invPerms interface{}) *PermissionSet {
	type InvitationPermissions struct {
		Read   bool `json:"read,omitempty"`
		Create bool `json:"create,omitempty"`
	}

	if invPerms == nil {
		return nil
	}

	switch p := invPerms.(type) {
	case InvitationPermissions:
		return &PermissionSet{
			Read:   p.Read,
			Create: p.Create,
		}
	case *InvitationPermissions:
		if p == nil {
			return nil
		}
		return &PermissionSet{
			Read:   p.Read,
			Create: p.Create,
		}
	case PermissionSet:
		return &p
	case *PermissionSet:
		return p
	default:
		return nil
	}
}
