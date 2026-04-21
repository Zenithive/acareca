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
	Email       string       `json:"email" validate:"required,email"`
	Permissions *Permissions `json:"permissions" validate:"required"`
}

// RsInvitation is the response after an invitation is created
type RsInvitation struct {
	ID           uuid.UUID        `json:"id"`
	Email        string           `json:"email"`
	AccountantID *uuid.UUID       `json:"accountant_id"`
	InviteLink   string           `json:"invite_link"`
	Status       InvitationStatus `json:"status"`
	ExpiresAt    time.Time        `json:"expires_at"`
	Permissions  *Permissions     `json:"permissions"`
}

type UserDetails struct {
	FirstName string `json:"first_name" db:"first_name"`
	LastName  string `json:"last_name"  db:"last_name"`
	Email     string `json:"email"      db:"email"`
}

type RsInviteDetails struct {
	InvitationID uuid.UUID        `json:"invitation_id"`
	Status       InvitationStatus `json:"status"`
	IsFound      bool             `json:"is_found"`
	SentBy       UserDetails      `json:"sent_by"`
	SentTo       UserDetails      `json:"sent_to"`
	SenderRole   string           `json:"sender_role"`
	AccountantID *uuid.UUID       `json:"id"`
	Email        string           `json:"email"`
	Permissions  *Permissions     `json:"permissions"`
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

// Permissions represents feature-based permissions with read/write access
type Permissions struct {
	SalesPurchases *AccessLevel `json:"sales_purchases,omitempty"`
	LockDates      *AccessLevel `json:"lock_dates,omitempty"`
	Users          *AccessLevel `json:"users,omitempty"`
	Reports        *AccessLevel `json:"reports,omitempty"`
}

// AccessLevel represents read/write access for a feature
type AccessLevel struct {
	Read  bool `json:"read"`
	Write bool `json:"write"`
}

// IsEmpty checks if permissions are empty
func (p *Permissions) IsEmpty() bool {
	if p == nil {
		return true
	}
	return p.SalesPurchases == nil &&
		p.LockDates == nil &&
		p.Users == nil &&
		p.Reports == nil
}

// HasReadAccess checks if read access is granted for a feature
func (p *Permissions) HasReadAccess(feature string) bool {
	if p == nil {
		return false
	}

	access := p.getAccessByName(feature)
	return access != nil && access.Read
}

// HasWriteAccess checks if write access is granted for a feature
func (p *Permissions) HasWriteAccess(feature string) bool {
	if p == nil {
		return false
	}

	access := p.getAccessByName(feature)
	return access != nil && access.Write
}

// HasAccess checks if any access (read or write) is granted for a feature
// Kept for backward compatibility
func (p *Permissions) HasAccess(feature string) bool {
	return p.HasReadAccess(feature) || p.HasWriteAccess(feature)
}

// getAccessByName returns access level by feature name
func (p *Permissions) getAccessByName(feature string) *AccessLevel {
	if p == nil {
		return nil
	}

	switch strings.ToLower(feature) {
	case "sales_purchases", "salespurchases":
		return p.SalesPurchases
	case "lock_dates", "lockdates":
		return p.LockDates
	case "users", "user":
		return p.Users
	case "reports", "report":
		return p.Reports
	default:
		return nil
	}
}

// RqGrantPermission is the input for granting/updating permissions
type RqGrantPermission struct {
	AccountantID *uuid.UUID   `json:"accountant_id,omitempty"`
	Email        string       `json:"email" validate:"omitempty,email"`
	Permissions  *Permissions `json:"permissions" validate:"required"`
}

// RqUpdatePermissions is the input for updating permissions
type RqUpdatePermissions struct {
	AccountantID *uuid.UUID   `json:"accountant_id,omitempty"`
	Email        string       `json:"email" validate:"required,email"`
	Permissions  *Permissions `json:"permissions" validate:"required"`
}

// RqPermissionDetail is deprecated - kept for backward compatibility
// Use Permissions directly instead
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

// DefaultAccountantPermissions returns default permissions for accountants
func DefaultAccountantPermissions() *Permissions {
	return &Permissions{
		SalesPurchases: &AccessLevel{Read: true, Write: false},
		LockDates:      &AccessLevel{Read: true, Write: false},
		Users:          &AccessLevel{Read: true, Write: false},
		Reports:        &AccessLevel{Read: true, Write: false},
	}
}

// ValidatePermissions ensures at least one permission is granted
func ValidatePermissions(p *Permissions) error {
	if p == nil || p.IsEmpty() {
		return errors.New("at least one permission must be granted")
	}

	// Ensure write access implies read access
	if p.SalesPurchases != nil && p.SalesPurchases.Write && !p.SalesPurchases.Read {
		p.SalesPurchases.Read = true
	}
	if p.LockDates != nil && p.LockDates.Write && !p.LockDates.Read {
		p.LockDates.Read = true
	}
	if p.Users != nil && p.Users.Write && !p.Users.Read {
		p.Users.Read = true
	}
	if p.Reports != nil && p.Reports.Write && !p.Reports.Read {
		p.Reports.Read = true
	}

	return nil
}
