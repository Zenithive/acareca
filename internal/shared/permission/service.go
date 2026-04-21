package permission

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type Service interface {
	Check(ctx context.Context, permCtx Context, resource Resource, action Action) CheckResult

	CheckMultiple(ctx context.Context, permCtx Context, checks []PermissionCheck) map[string]CheckResult

	GetPermissions(ctx context.Context, permCtx Context, resource Resource) PermissionSet

	GetAllPermissions(ctx context.Context, permCtx Context) map[Resource]PermissionSet

	Enforce(ctx context.Context, permCtx Context, resource Resource, action Action) error
}

type PermissionCheck struct {
	ID       string   `json:"id"`
	Resource Resource `json:"resource"`
	Action   Action   `json:"action"`
}

type service struct {
	policy Policy
}

func NewService(policy Policy) Service {
	return &service{
		policy: policy,
	}
}

func NewDefaultService(provider PermissionProvider) Service {
	rbac := NewRBACPolicy()
	abac := NewABACPolicy(provider)
	relationship := NewRelationshipPolicy(provider)

	smartPolicy := PolicyFunc(func(ctx Context, resource Resource, action Action) CheckResult {

		rbacResult := rbac.Check(ctx, resource, action)
		if !rbacResult.Allowed {
			return rbacResult
		}

		switch ctx.Role {
		case RoleAccountant:
			relResult := relationship.Check(ctx, resource, action)
			if !relResult.Allowed {
				return relResult
			}

			if ctx.EntityID != nil {
				abacResult := abac.Check(ctx, resource, action)
				if !abacResult.Allowed {
					return abacResult
				}
			}
		}

		return Allow()
	})

	return NewService(smartPolicy)
}

func (s *service) Check(ctx context.Context, permCtx Context, resource Resource, action Action) CheckResult {
	return s.policy.Check(permCtx, resource, action)
}

func (s *service) CheckMultiple(ctx context.Context, permCtx Context, checks []PermissionCheck) map[string]CheckResult {
	results := make(map[string]CheckResult, len(checks))

	for _, check := range checks {
		results[check.ID] = s.policy.Check(permCtx, check.Resource, check.Action)
	}

	return results
}

func (s *service) GetPermissions(ctx context.Context, permCtx Context, resource Resource) PermissionSet {
	return s.policy.GetPermissions(permCtx, resource)
}

func (s *service) GetAllPermissions(ctx context.Context, permCtx Context) map[Resource]PermissionSet {
	resources := []Resource{
		ResourceReport,
		ResourceUser,
		ResourceSalesPurchase,
		ResourceLockDate,
	}

	result := make(map[Resource]PermissionSet, len(resources))
	for _, resource := range resources {
		result[resource] = s.policy.GetPermissions(permCtx, resource)
	}

	return result
}

func (s *service) Enforce(ctx context.Context, permCtx Context, resource Resource, action Action) error {
	result := s.policy.Check(permCtx, resource, action)
	if !result.Allowed {
		return &PermissionError{
			Resource: resource,
			Action:   action,
			Reason:   result.Reason,
		}
	}
	return nil
}

type PermissionError struct {
	Resource Resource
	Action   Action
	Reason   string
}

func (e *PermissionError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("permission denied: cannot %s %s - %s", e.Action, e.Resource, e.Reason)
	}
	return fmt.Sprintf("permission denied: cannot %s %s", e.Action, e.Resource)
}

func IsPermissionError(err error) bool {
	_, ok := err.(*PermissionError)
	return ok
}

// Helper functions to create permission contexts
func NewAccountantContext(userID, accountantID uuid.UUID) Context {
	return Context{
		UserID:       userID,
		Role:         RoleAccountant,
		AccountantID: &accountantID,
	}
}

func NewAccountantEntityContext(userID, accountantID, practitionerID, entityID uuid.UUID, entityType string) Context {
	return Context{
		UserID:         userID,
		Role:           RoleAccountant,
		AccountantID:   &accountantID,
		PractitionerID: &practitionerID,
		EntityID:       &entityID,
		EntityType:     &entityType,
	}
}
