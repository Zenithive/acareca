package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/permission"
)

func main() {
	// Example 1: Simple RBAC check
	fmt.Println("=== Example 1: RBAC Permission Check ===")
	rbacExample()

	fmt.Println("\n=== Example 3: Accountant with Entity Permissions ===")
	accountantExample()

	fmt.Println("\n=== Example 4: Multiple Permission Checks ===")
	multipleChecksExample()

	fmt.Println("\n=== Example 5: Get All Permissions ===")
	getAllPermissionsExample()
}

func rbacExample() {
	// Create a simple RBAC policy
	rbacPolicy := permission.NewRBACPolicy()
	service := permission.NewService(rbacPolicy)

	// Create an accountant context
	accountantCtx := permission.Context{
		UserID: uuid.New(),
		Role:   permission.RoleAccountant,
	}

	// Check various permissions
	checks := []struct {
		resource permission.Resource
		action   permission.Action
	}{
		{permission.ResourceReport, permission.ActionRead},
		{permission.ResourceReport, permission.ActionCreate},
	}

	for _, check := range checks {
		result := service.Check(context.Background(), accountantCtx, check.resource, check.action)
		fmt.Printf("Accountant can %s %s: %v", check.action, check.resource, result.Allowed)
		if !result.Allowed {
			fmt.Printf(" (Reason: %s)", result.Reason)
		}
		fmt.Println()
	}
}

func accountantExample() {
	// Create a mock permission provider
	mockProvider := &MockPermissionProvider{
		permissions: map[string]*permission.PermissionSet{
			"entity1": {Read: true, Create: true},
			"entity2": {Read: true, Create: false},
		},
		linkedPairs: map[string]bool{
			"practitioner1-accountant1": true,
		},
	}

	// Create service with default policies (RBAC + ABAC + Relationship)
	service := permission.NewDefaultService(mockProvider)

	accountantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	practitionerID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	entityID1 := uuid.MustParse("00000000-0000-0000-0000-000000000003")

	// Create accountant context with entity access
	accountantCtx := permission.Context{
		UserID:         uuid.New(),
		Role:           permission.RoleAccountant,
		AccountantID:   &accountantID,
		PractitionerID: &practitionerID,
		EntityID:       &entityID1,
		EntityType:     strPtr("FORM"),
	}

	// Check permissions
	checks := []struct {
		resource permission.Resource
		action   permission.Action
	}{
		{permission.ResourceReport, permission.ActionRead},
		{permission.ResourceReport, permission.ActionCreate},
	}

	for _, check := range checks {
		result := service.Check(context.Background(), accountantCtx, check.resource, check.action)
		fmt.Printf("Accountant can %s %s: %v", check.action, check.resource, result.Allowed)
		if !result.Allowed {
			fmt.Printf(" (Reason: %s)", result.Reason)
		}
		fmt.Println()
	}
}

func multipleChecksExample() {
	rbacPolicy := permission.NewRBACPolicy()
	service := permission.NewService(rbacPolicy)

	accountantCtx := permission.Context{
		UserID: uuid.New(),
		Role:   permission.RoleAccountant,
	}

	// Check multiple permissions at once
	checks := []permission.PermissionCheck{
		{ID: "check1", Resource: permission.ResourceReport, Action: permission.ActionRead},
		{ID: "check2", Resource: permission.ResourceReport, Action: permission.ActionCreate},
		{ID: "check3", Resource: permission.ResourceReport, Action: permission.ActionRead},
		{ID: "check4", Resource: permission.ResourceReport, Action: permission.ActionRead},
	}

	results := service.CheckMultiple(context.Background(), accountantCtx, checks)

	for _, check := range checks {
		result := results[check.ID]
		fmt.Printf("%s - %s %s: %v", check.ID, check.Action, check.Resource, result.Allowed)
		if !result.Allowed {
			fmt.Printf(" (Reason: %s)", result.Reason)
		}
		fmt.Println()
	}
}

func getAllPermissionsExample() {
	rbacPolicy := permission.NewRBACPolicy()
	service := permission.NewService(rbacPolicy)

	accountantCtx := permission.Context{
		UserID: uuid.New(),
		Role:   permission.RoleAccountant,
	}

	allPerms := service.GetAllPermissions(context.Background(), accountantCtx)

	fmt.Println("All permissions for Accountant:")
	for resource, perms := range allPerms {
		if !perms.IsEmpty() {
			fmt.Printf("  %s: ", resource)
			actions := []string{}
			if perms.Read {
				actions = append(actions, "read")
			}
			if perms.Create {
				actions = append(actions, "create")
			}
			fmt.Println(actions)
		}
	}
}

// MockPermissionProvider for demonstration
type MockPermissionProvider struct {
	permissions map[string]*permission.PermissionSet
	linkedPairs map[string]bool
}

func (m *MockPermissionProvider) GetEntityPermissions(accountantID uuid.UUID, entityID uuid.UUID) (*permission.PermissionSet, error) {
	key := entityID.String()
	if perms, ok := m.permissions[key]; ok {
		return perms, nil
	}
	// Return default read-only permission
	return &permission.PermissionSet{Read: true}, nil
}

func (m *MockPermissionProvider) GetEntityPermissionsByEmail(practitionerID uuid.UUID, email string, entityID uuid.UUID) (*permission.PermissionSet, error) {
	return m.GetEntityPermissions(uuid.Nil, entityID)
}

func (m *MockPermissionProvider) IsAccountantLinkedToPractitioner(practitionerID, accountantID uuid.UUID) (bool, error) {
	key := fmt.Sprintf("%s-%s", practitionerID.String(), accountantID.String())
	return m.linkedPairs[key], nil
}

func strPtr(s string) *string {
	return &s
}
