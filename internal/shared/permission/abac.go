package permission

import (
	"github.com/google/uuid"
)

type ABACPolicy struct {
	permissionProvider PermissionProvider
}

type PermissionProvider interface {
	GetEntityPermissions(accountantID uuid.UUID, entityID uuid.UUID) (*PermissionSet, error)
	GetEntityPermissionsByEmail(practitionerID uuid.UUID, email string, entityID uuid.UUID) (*PermissionSet, error)
	IsAccountantLinkedToPractitioner(practitionerID, accountantID uuid.UUID) (bool, error)
}

func NewABACPolicy(provider PermissionProvider) *ABACPolicy {
	return &ABACPolicy{
		permissionProvider: provider,
	}
}

func (a *ABACPolicy) Check(ctx Context, resource Resource, action Action) CheckResult {
	if ctx.Role != RoleAccountant {
		return Deny("ABAC policy only applies to accountants")
	}

	if ctx.EntityID == nil {
		return Deny("entity ID required for entity-specific permission check")
	}

	if ctx.AccountantID == nil {
		return Deny("accountant ID required for permission check")
	}

	perms, err := a.permissionProvider.GetEntityPermissions(*ctx.AccountantID, *ctx.EntityID)
	if err != nil {
		return Deny("failed to fetch entity permissions: " + err.Error())
	}

	if perms == nil {
		return Deny("no permissions found for this entity")
	}

	if perms.HasPermission(action) {
		return Allow()
	}

	return Deny("entity-specific permission denied")
}

func (a *ABACPolicy) GetPermissions(ctx Context, resource Resource) PermissionSet {
	if ctx.Role != RoleAccountant || ctx.EntityID == nil || ctx.AccountantID == nil {
		return PermissionSet{}
	}

	perms, err := a.permissionProvider.GetEntityPermissions(*ctx.AccountantID, *ctx.EntityID)
	if err != nil || perms == nil {
		return PermissionSet{}
	}

	return *perms
}

type OwnershipPolicy struct{}

func NewOwnershipPolicy() *OwnershipPolicy {
	return &OwnershipPolicy{}
}

func (o *OwnershipPolicy) Check(ctx Context, resource Resource, action Action) CheckResult {
	if ctx.Role == RoleAccountant {
		return Deny("accountants don't own resources, check entity permissions")
	}

	return Deny("ownership check failed")
}

func (o *OwnershipPolicy) GetPermissions(ctx Context, resource Resource) PermissionSet {
	result := o.Check(ctx, resource, "all")
	if result.Allowed {
		return PermissionSet{Read: true, Create: true}
	}
	return PermissionSet{}
}

type RelationshipPolicy struct {
	permissionProvider PermissionProvider
}

func NewRelationshipPolicy(provider PermissionProvider) *RelationshipPolicy {
	return &RelationshipPolicy{
		permissionProvider: provider,
	}
}

func (r *RelationshipPolicy) Check(ctx Context, resource Resource, action Action) CheckResult {
	if ctx.Role != RoleAccountant {
		return Allow()
	}

	if ctx.PractitionerID == nil || ctx.AccountantID == nil {
		return Deny("practitioner and accountant IDs required")
	}

	linked, err := r.permissionProvider.IsAccountantLinkedToPractitioner(*ctx.PractitionerID, *ctx.AccountantID)
	if err != nil {
		return Deny("failed to verify relationship: " + err.Error())
	}

	if !linked {
		return Deny("no relationship exists between accountant and practitioner")
	}

	return Allow()
}

func (r *RelationshipPolicy) GetPermissions(ctx Context, resource Resource) PermissionSet {
	result := r.Check(ctx, resource, "read")
	if result.Allowed {
		return PermissionSet{Read: true}
	}
	return PermissionSet{}
}
