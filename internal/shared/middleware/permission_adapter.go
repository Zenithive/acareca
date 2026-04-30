package middleware

import (
	"context"

	"github.com/google/uuid"
)

// PermissionAdapter adapts an invitation service to provide feature permission checking
type PermissionAdapter struct {
	getPermissions func(ctx context.Context, accountantID uuid.UUID, practitionerID uuid.UUID) (FeaturePermissions, error)
	checkLink      func(ctx context.Context, practitionerID, accountantID uuid.UUID) (bool, error)
}

// NewPermissionAdapterFromFuncs creates a permission adapter from functions
func NewPermissionAdapterFromFuncs(
	getPerms func(ctx context.Context, accountantID uuid.UUID, practitionerID uuid.UUID) (FeaturePermissions, error),
	checkLink func(ctx context.Context, practitionerID, accountantID uuid.UUID) (bool, error),
) *PermissionAdapter {
	return &PermissionAdapter{
		getPermissions: getPerms,
		checkLink:      checkLink,
	}
}

// GetPermissionsForAccountant retrieves permissions for an accountant from a specific practitioner
func (a *PermissionAdapter) GetPermissionsForAccountant(ctx context.Context, accountantID uuid.UUID, practitionerID uuid.UUID) (FeaturePermissions, error) {
	// First check if accountant is linked to practitioner
	isLinked, err := a.checkLink(ctx, practitionerID, accountantID)
	if err != nil {
		return nil, err
	}

	if !isLinked {
		return nil, nil // Not linked, no permissions
	}

	// Get permissions
	return a.getPermissions(ctx, accountantID, practitionerID)
}
