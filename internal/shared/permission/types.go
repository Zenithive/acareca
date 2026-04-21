package permission

import "github.com/google/uuid"

type Resource string

type Action string

type Role string

// Constants for Resources
const (
	ResourceReport        Resource = "report"
	ResourceUser          Resource = "user"
	ResourceSalesPurchase Resource = "sales_purchase"
	ResourceLockDate      Resource = "lock_date"
)

// Constants for Actions
const (
	ActionRead   Action = "read"
	ActionCreate Action = "create"
	// ActionUpdate Action = "update"
	// ActionDelete Action = "delete"
	// ActionAll    Action = "all"
	// ActionManage Action = "manage"
	// ActionView   Action = "view"
	// ActionExport Action = "export"
)

// Constants for Roles
const (
	RoleAccountant Role = "accountant"
)

type Permission struct {
	Resource Resource `json:"resource"`
	Action   Action   `json:"action"`
}

type Context struct {
	UserID         uuid.UUID              `json:"user_id"`
	Role           Role                   `json:"role"`
	PractitionerID *uuid.UUID             `json:"practitioner_id,omitempty"`
	AccountantID   *uuid.UUID             `json:"accountant_id,omitempty"`
	EntityID       *uuid.UUID             `json:"entity_id,omitempty"`
	EntityType     *string                `json:"entity_type,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

type PermissionSet struct {
	Read   bool `json:"read"`
	Create bool `json:"create"`
}

func (ps *PermissionSet) HasPermission(action Action) bool {
	if ps == nil {
		return false
	}
	switch action {
	case ActionRead:
		return ps.Read
	case ActionCreate:
		return ps.Create
	default:
		return false
	}
}

func (ps *PermissionSet) Grant(action Action) {
	if ps == nil {
		return
	}

	switch action {
	case ActionRead:
		ps.Read = true
	case ActionCreate:
		ps.Create = true
	}
}

func (ps *PermissionSet) Revoke(action Action) {
	if ps == nil {
		return
	}

	switch action {
	case ActionRead:
		ps.Read = false
	case ActionCreate:
		ps.Create = false
	}
}

func (ps *PermissionSet) IsEmpty() bool {
	if ps == nil {
		return true
	}
	return !ps.Read && !ps.Create
}

type EntityPermission struct {
	EntityID    uuid.UUID     `json:"entity_id"`
	EntityType  string        `json:"entity_type"`
	Permissions PermissionSet `json:"permissions"`
}

type CheckResult struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

func Allow() CheckResult {
	return CheckResult{Allowed: true}
}

func Deny(reason string) CheckResult {
	return CheckResult{Allowed: false, Reason: reason}
}
