package invitation

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

// InvitationStatus defines the allowed states for an invitation
type InvitationStatus string

const (
	StatusSent      InvitationStatus = "SENT"
	StatusAccepted  InvitationStatus = "ACCEPTED"
	StatusCompleted InvitationStatus = "COMPLETED"
	StatusRejected  InvitationStatus = "REJECTED"
	StatusResent    InvitationStatus = "RESENT"
	StatusRevoked   InvitationStatus = "REVOKED"
)

// Invitation represents the tbl_invitation schema
type Invitation struct {
	ID             uuid.UUID        `json:"id" db:"id"`
	PractitionerID uuid.UUID        `json:"practitioner_id" db:"practitioner_id"`
	EntityID       *uuid.UUID       `json:"entity_id" db:"entity_id"`
	Email          string           `json:"email" db:"email"`
	Status         InvitationStatus `json:"status" db:"status"`
	CreatedAt      time.Time        `json:"created_at" db:"created_at"`
	ExpiresAt      time.Time        `json:"expires_at" db:"expires_at"`
}

// RqSendInvitation is the input for creating a new invitation
type RqSendInvitation struct {
	Email       string               `json:"email" validate:"required,email"`
	Permissions []RqPermissionDetail `json:"permissions"`
}

// RsInvitation is the response after an invitation is created
type RsInvitation struct {
	ID           uuid.UUID            `json:"id"`
	Email        string               `json:"email"`
	AccountantID *uuid.UUID           `json:"accountant_id"`
	InviteLink   string               `json:"invite_link"`
	Status       InvitationStatus     `json:"status"`
	ExpiresAt    time.Time            `json:"expires_at"`
	Permissions  []RqPermissionDetail `json:"permissions"`
}

type UserDetails struct {
	FirstName string `json:"first_name" db:"first_name"`
	LastName  string `json:"last_name"  db:"last_name"`
	Email     string `json:"email"      db:"email"`
}

type RsInviteDetails struct {
	InvitationID uuid.UUID            `json:"invitation_id"`
	Status       InvitationStatus     `json:"status"`
	IsFound      bool                 `json:"is_found"`
	SentBy       UserDetails          `json:"sent_by"`
	SentTo       UserDetails          `json:"sent_to"`
	SenderRole   string               `json:"sender_role"`
	AccountantID *uuid.UUID           `json:"id"`
	Email        string               `json:"email"`
	Permissions  []RqPermissionDetail `json:"permissions"`
}

// RsInviteProcess helps the frontend navigate after a link click
type RsInviteProcess struct {
	InvitationID   uuid.UUID        `json:"invitation_id"`
	PractitionerID uuid.UUID        `json:"practitioner_id" db:"practitioner_id"`
	Email          string           `json:"email" db:"email"`
	Status         InvitationStatus `json:"status"`
	IsFound        bool             `json:"is_found"`
}

// RsInvitationListItem is the standardized response for ListInvitations (both practitioner and accountant)
type RsInvitationListItem struct {
	ID                uuid.UUID        `json:"id" db:"id"`
	PractitionerID    uuid.UUID        `json:"practitioner_id" db:"practitioner_id"`
	PractitionerEmail string           `json:"practitioner_email" db:"practitioner_email"`
	EntityID          *uuid.UUID       `json:"entity_id" db:"entity_id"`
	Email             string           `json:"email" db:"email"`
	Status            InvitationStatus `json:"status" db:"status"`
	InviteLink        string           `json:"invite_link"`
	CreatedAt         time.Time        `json:"created_at" db:"created_at"`
	ExpiresAt         time.Time        `json:"expires_at" db:"expires_at"`
}

// Internal struct for Repository JOIN result
type InvitationExtended struct {
	Invitation
	SenderFirstName string `db:"sender_first_name"`
	SenderLastName  string `db:"sender_last_name"`
	SenderEmail     string `db:"sender_email"`
}

// RqProcessAction is the input for accepting or rejecting
type RqProcessAction struct {
	TokenID uuid.UUID `json:"token_id" validate:"required"`
	Action  string    `json:"action" validate:"required,oneof=ACCEPT REJECT"`
}

// AccountantPermissionRow represents the raw database row
type AccountantPermissionRow struct {
	ID             uuid.UUID   `db:"id" json:"id"`
	EntityID       uuid.UUID   `db:"entity_id" json:"entity_id"`
	EntityType     string      `db:"entity_type" json:"entity_type"`
	PractitionerID uuid.UUID   `db:"practitioner_id" json:"practitioner_id"`
	AccountantID   uuid.UUID   `db:"accountant_id" json:"accountant_id"`
	Permissions    Permissions `db:"permissions" json:"permissions"`
	CreatedAt      time.Time   `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time   `db:"updated_at" json:"updated_at"`
	DeletedAt      *time.Time  `db:"deleted_at" json:"deleted_at,omitempty"`
}

// AccountantPermissionRes represents what the user sees
type AccountantPermissionRes struct {
	ID             uuid.UUID   `json:"id"`
	EntityID       uuid.UUID   `json:"entity_id"`
	EntityType     string      `json:"entity_type"`
	PractitionerID uuid.UUID   `json:"practitioner_id"`
	AccountantID   uuid.UUID   `json:"accountant_id"`
	Permissions    Permissions `json:"permissions"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

// FILTERS
var invitationColumns = map[string]string{
	"email":           "email",
	"status":          "status::text",
	"created_at":      "created_at",
	"practitioner_id": "practitioner_id",
	"entity_id":       "entity_id",
	"accountant_id":   "accountant_id",
	"deleted_at":      "deleted_at",
}

var invitationSearchCols = []string{"email"}

type Filter struct {
	Status *string `form:"status"`
	Role   string  `form:"-"`
	common.Filter
}

func (filter *Filter) MapToFilter(actorID *uuid.UUID) common.Filter {
	filters := map[string]interface{}{}

	// Role-based security: Apply the correct ID based on who is asking
	if actorID != nil && filter.Role == util.RolePractitioner {
		filters["practitioner_id"] = *actorID
	} else {
		filters["entity_id"] = *actorID
	}

	if filter.Status != nil {
		filters["status"] = *filter.Status
	}

	f := common.NewFilter(filter.Search, filters, nil, filter.Limit, filter.Offset, filter.SortBy, filter.OrderBy)

	// Only add the "Not Equal" condition if the user DID NOT provide a status filter
	if filter.Status == nil {
		f.Where = append(f.Where, common.Condition{
			Field: "status", Operator: common.OpNotEq, Value: StatusResent,
		})
	}

	return f
}

// MapToFilterAccountant builds a filter for the accountant path.
// The email WHERE clause is handled separately in the repo, so we only
// apply status and pagination here.
func (filter *Filter) MapToFilterAccountant() common.Filter {
	f := common.NewFilter(nil, nil, nil, filter.Limit, filter.Offset, filter.SortBy, filter.OrderBy)
	return f
}

type Permissions struct {
	Read   bool `json:"read,omitempty"`
	Create bool `json:"create,omitempty"`
	Update bool `json:"update,omitempty"`
	Delete bool `json:"delete,omitempty"`
	All    bool `json:"all,omitempty"`
}

// Helper to check a specific action
func (p *Permissions) HasAccess(action string) bool {
	if p == nil {
		return false // prevents nil pointer dereference if Permissions is null in the database
	}

	if p.All {
		return true
	}
	switch strings.ToLower(action) {
	case "create":
		return p.Create
	case "read":
		return p.Read
	case "update":
		return p.Update
	case "delete":
		return p.Delete
	default:
		return false
	}
}

// RqGrantPermission is the input for granting/updating permissions
type RqGrantPermission struct {
	AccountantID *uuid.UUID           `json:"accountant_id,omitempty"`
	Email        string               `json:"email" validate:"omitempty,email"`
	Permissions  []RqPermissionDetail `json:"permissions" validate:"required,dive"`
}

// RqUpdatePermissions is the input for updating permissions
type RqUpdatePermissions struct {
	AccountantID uuid.UUID            `json:"accountant_id" validate:"required"`
	Permissions  []RqPermissionDetail `json:"permissions" validate:"required,dive"`
}

// RqPermissionDetail is for individual entity permissions
type RqPermissionDetail struct {
	EntityID    uuid.UUID   `json:"entity_id" validate:"required"`
	EntityType  string      `json:"entity_type" validate:"required,oneof=CLINIC FORM ENTRY"`
	Permissions Permissions `json:"permissions" validate:"required"`
}

// Make it satisfy the sql.Scanner interface (Database -> Go)
func (p *Permissions) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("type assertion to []byte/string failed")
	}

	// First attempt: Standard Unmarshal
	err := json.Unmarshal(bytes, p)
	if err != nil {
		// Second attempt: Check if it's a double-encoded string (common in messy migrations)
		var s string
		if err2 := json.Unmarshal(bytes, &s); err2 == nil {
			return json.Unmarshal([]byte(s), p)
		}
		return err // Return original error if fallback also fails
	}

	return nil
}

// Make it satisfy the driver.Valuer interface (Go -> Database)
func (p Permissions) Value() (driver.Value, error) {
	return json.Marshal(p)
}

// MarshalJSON ensures that if All is true, all individual flags appear as true in the API response.
func (p Permissions) MarshalJSON() ([]byte, error) {
	type Alias Permissions
	if p.All {
		return json.Marshal(&struct {
			Read   bool `json:"read"`
			Create bool `json:"create"`
			Update bool `json:"update"`
			Delete bool `json:"delete"`
			All    bool `json:"all"`
			Alias
		}{
			Read:   true,
			Create: true,
			Update: true,
			Delete: true,
			All:    true,
			Alias:  (Alias)(p),
		})
	}
	return json.Marshal((Alias)(p))
}
