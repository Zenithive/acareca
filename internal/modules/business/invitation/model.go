package invitation

import (
	"fmt"
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
	Permission   *Permissions     `json:"permissions"`
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

type PermissionName string

const (
	PermSalesPurchases      PermissionName = "sales_purchases"
	PermLockDates           PermissionName = "lock_dates"
	PermManageUsers         PermissionName = "manage_users"
	PermReportsViewDownload PermissionName = "reports_view_download"
)

var AllPermissions = []PermissionName{
	PermSalesPurchases,
	PermLockDates,
	PermManageUsers,
	PermReportsViewDownload,
}

type Permission struct {
	Name        PermissionName `db:"name,omitempty"`
	AccessLevel AccessLevel    `json:"access_level"`
}

type AccessLevel struct {
	Read  bool `db:"read"`
	Write bool `db:"write"`
}

type PermissionsData struct {
	SalesPurchases      Permission
	LockDates           Permission
	ManageUsers         Permission
	ReportsViewDownload Permission
}

// func (p PermissionsData) toRows() []Permission {
// 	return []Permission{
// 		{Name: "sales_purchases", AccessLevel: AccessLevel{Read: p.SalesPurchases.AccessLevel.Read, Write: p.SalesPurchases.AccessLevel.Write}},
// 		{Name: "lock_dates", AccessLevel: AccessLevel{Read: p.LockDates.AccessLevel.Read, Write: p.LockDates.AccessLevel.Write}},
// 		{Name: "manage_users", AccessLevel: AccessLevel{Read: p.ManageUsers.AccessLevel.Read, Write: p.ManageUsers.AccessLevel.Write}},
// 		{Name: "reports_view_download", AccessLevel: AccessLevel{Read: p.ReportsViewDownload.AccessLevel.Read, Write: p.ReportsViewDownload.AccessLevel.Write}},
// 	}
// }

type Permissions map[PermissionName]AccessLevel

func (p Permissions) Get(name PermissionName) AccessLevel {
	return p[name]
}

func (p *Permissions) ToRows() []Permission {
	rows := make([]Permission, 0, len(AllPermissions))
	for _, name := range AllPermissions {
		rows = append(rows, Permission{
			Name:        name,
			AccessLevel: p.Get(name),
		})
	}
	return rows
}

func (p Permissions) Validate() error {
	for _, name := range AllPermissions {
		if _, ok := p[name]; !ok {
			return fmt.Errorf("missing permission: %q", name)
		}
	}
	return nil
}

func (p Permissions) Has(name PermissionName, write bool) bool {
	al := p.Get(name)
	if write {
		return al.Write
	}
	return al.Read
}

type RqGrantPermission struct {
	AccountantID *uuid.UUID   `json:"accountant_id,omitempty"`
	Email        string       `json:"email" validate:"omitempty,email"`
	Permissions  *Permissions `json:"permissions" validate:"required"`
}

type RqUpdatePermissions struct {
	AccountantID *uuid.UUID   `json:"accountant_id,omitempty"`
	Email        string       `json:"email" validate:"required,email"`
	Permissions  *Permissions `json:"permissions" validate:"required"`
}

type InvitationPermission struct {
	ID             uuid.UUID   `json:"id"`
	PractitionerID uuid.UUID   `json:"practitioner_id"`
	AccountantID   uuid.UUID   `json:"accountant_id"`
	Permissions    Permissions `json:"permissions"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

type RsPermission struct {
	ID             uuid.UUID   `json:"id"`
	PractitionerID uuid.UUID   `json:"practitioner_id"`
	AccountantID   uuid.UUID   `json:"accountant_id"`
	Permissions    Permissions `json:"permissions"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

type RsPermissions map[PermissionName]AccessLevel

func (p *Permission) ToRsPermission() *RsPermission {
	return &RsPermission{
		Permissions: Permissions{
			p.Name: p.AccessLevel,
		},
	}
}

func (p Permissions) FromRow(row Permission) {
	p[PermissionName(row.Name)] = AccessLevel{
		Read:  row.AccessLevel.Read,
		Write: row.AccessLevel.Write,
	}
}
