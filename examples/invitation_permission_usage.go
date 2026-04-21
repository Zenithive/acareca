package main

import (
	"encoding/json"
	"fmt"

	"github.com/iamarpitzala/acareca/internal/modules/business/invitation"
)

func main() {
	fmt.Println("=== Invitation Permission System Usage Examples ===\n")

	// Example 1: Send Invite Payload
	sendInviteExample()

	// Example 2: Get Invite Details Response
	getInviteDetailsExample()

	// Example 3: Update Permissions
	updatePermissionsExample()

	// Example 4: Check Permissions
	checkPermissionsExample()
}

func sendInviteExample() {
	fmt.Println("Example 1: Send Invite Payload")
	fmt.Println("POST /invitations")

	payload := invitation.RqSendInvitation{
		Email: "user@email.com",
		Permissions: &invitation.Permissions{
			SalesPurchases: &invitation.AccessLevel{Read: true, Write: true},
			LockDates:      &invitation.AccessLevel{Read: true, Write: false},
			Users:          &invitation.AccessLevel{Read: false, Write: false},
			Reports:        &invitation.AccessLevel{Read: true, Write: false},
		},
	}

	jsonData, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Println(string(jsonData))
	fmt.Println()
}

func getInviteDetailsExample() {
	fmt.Println("Example 2: Get Invite Details Response")
	fmt.Println("GET /invitations/{id}")

	response := map[string]interface{}{
		"status": 200,
		"data": map[string]interface{}{
			"invitation_id": "dc070ae5-3029-4c41-8ebe-734fa57d6f24",
			"status":        "COMPLETED",
			"is_found":      true,
			"sent_by": map[string]string{
				"first_name": "Heer",
				"last_name":  "Mistry",
				"email":      "xaurivuyoji-1284@yopmail.com",
			},
			"sent_to": map[string]string{
				"first_name": "Heer",
				"last_name":  "Mistry",
				"email":      "mettuquoinatta-7089@yopmail.com",
			},
			"sender_role": "PRACTITIONER",
			"id":          "3901c38b-a4a7-4379-85da-76ae690d1620",
			"email":       "mettuquoinatta-7089@yopmail.com",
			"permissions": map[string]map[string]bool{
				"sales_purchases": {"read": true, "write": true},
				"lock_dates":      {"read": true, "write": false},
				"users":           {"read": false, "write": false},
				"reports":         {"read": true, "write": false},
			},
		},
		"message": "Invitation details fetched successfully",
	}

	jsonData, _ := json.MarshalIndent(response, "", "  ")
	fmt.Println(string(jsonData))
	fmt.Println()
}

func updatePermissionsExample() {
	fmt.Println("Example 3: Update Permissions")
	fmt.Println("PUT /invitations/permissions")

	payload := invitation.RqUpdatePermissions{
		Email: "accdev@yopmail.com",
		Permissions: &invitation.Permissions{
			SalesPurchases: &invitation.AccessLevel{Read: true, Write: true},
			LockDates:      &invitation.AccessLevel{Read: true, Write: false},
			Users:          &invitation.AccessLevel{Read: false, Write: false},
			Reports:        &invitation.AccessLevel{Read: true, Write: false},
		},
	}

	jsonData, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Println(string(jsonData))
	fmt.Println()
}

func checkPermissionsExample() {
	fmt.Println("Example 4: Check Permissions")

	perms := &invitation.Permissions{
		SalesPurchases: &invitation.AccessLevel{Read: true, Write: true},
		LockDates:      &invitation.AccessLevel{Read: true, Write: false},
		Users:          &invitation.AccessLevel{Read: false, Write: false},
		Reports:        &invitation.AccessLevel{Read: true, Write: false},
	}

	// Check various permissions
	checks := []struct {
		feature string
		action  string
	}{
		{"sales_purchases", "read"},
		{"sales_purchases", "write"},
		{"lock_dates", "read"},
		{"lock_dates", "write"},
		{"users", "read"},
		{"reports", "read"},
	}

	for _, check := range checks {
		var allowed bool
		if check.action == "read" {
			allowed = perms.HasReadAccess(check.feature)
		} else {
			allowed = perms.HasWriteAccess(check.feature)
		}

		status := "✓ ALLOWED"
		if !allowed {
			status = "✗ DENIED"
		}
		fmt.Printf("%s - %s %s\n", status, check.action, check.feature)
	}

	fmt.Println()

	// Validate permissions
	if err := invitation.ValidatePermissions(perms); err != nil {
		fmt.Printf("Validation Error: %v\n", err)
	} else {
		fmt.Println("✓ Permissions are valid")
	}

	// Check if empty
	fmt.Printf("Is Empty: %v\n", perms.IsEmpty())

	// Get default permissions
	fmt.Println("\nDefault Accountant Permissions:")
	defaultPerms := invitation.DefaultAccountantPermissions()
	jsonData, _ := json.MarshalIndent(defaultPerms, "", "  ")
	fmt.Println(string(jsonData))
}
