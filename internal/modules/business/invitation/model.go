package invitation

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type InvitationStatus string

const (
	StatusSent      InvitationStatus = "SENT"
	StatusAccepted  InvitationStatus = "ACCEPTED"
	StatusCompleted InvitationStatus = "COMPLETED"
	StatusRejected  InvitationStatus = "REJECTED"
	StatusResent    InvitationStatus = "RESENT"
	StatusRevoked   InvitationStatus = "REVOKED"
)

// Invitation reflects the physical schema representation of tbl_invitation.
type Invitation struct {
	ID             uuid.UUID        `json:"id" db:"id"`
	PractitionerID uuid.UUID        `json:"practitioner_id" db:"practitioner_id"`
	AccountantID   *uuid.UUID       `json:"accountant_id" db:"accountant_id"`
	Email          string           `json:"email" db:"email"`
	Status         InvitationStatus `json:"status" db:"status"`
	CreatedAt      time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt      *time.Time       `json:"updated_at" db:"updated_at"`
	DeletedAt      *time.Time       `json:"deleted_at" db:"deleted_at"`
	ExpiresAt      time.Time        `json:"expires_at" db:"expires_at"`
}

// InvitationExtended handles hydrated records returned from complex target repository JOIN queries.
type InvitationExtended struct {
	Invitation
	SenderFirstName string `db:"sender_first_name"`
	SenderLastName  string `db:"sender_last_name"`
	SenderEmail     string `db:"sender_email"`
}

type RqSendInvitation struct {
	Email       string       `json:"email" binding:"required,email"`
	Permissions *Permissions `json:"permissions" binding:"required"`
}

type RsInvitation struct {
	ID           uuid.UUID        `json:"id"`
	Email        string           `json:"email"`
	AccountantID *uuid.UUID       `json:"accountant_id"`
	InviteLink   string           `json:"invite_link"`
	Status       InvitationStatus `json:"status"`
	ExpiresAt    time.Time        `json:"expires_at"`
	Permissions  *Permissions     `json:"permissions,omitempty"`
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
	Permission   *Permissions     `json:"permissions,omitempty"`
}

type RsInviteProcess struct {
	InvitationID   uuid.UUID        `json:"invitation_id"`
	PractitionerID uuid.UUID        `json:"practitioner_id" db:"practitioner_id"`
	Email          string           `json:"email" db:"email"`
	Status         InvitationStatus `json:"status"`
	IsFound        bool             `json:"is_found"`
}

type RsInvitationListItem struct {
	ID                uuid.UUID        `json:"id" db:"id"`
	PractitionerID    uuid.UUID        `json:"practitioner_id" db:"practitioner_id"`
	PractitionerEmail string           `json:"practitioner_email" db:"practitioner_email"`
	AccountantID      *uuid.UUID       `json:"accountant_id" db:"accountant_id"`
	Email             string           `json:"email" db:"email"`
	Status            InvitationStatus `json:"status" db:"status"`
	InviteLink        string           `json:"invite_link"`
	CreatedAt         time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt         *time.Time       `json:"updated_at" db:"updated_at"`
	DeletedAt         *time.Time       `json:"deleted_at" db:"deleted_at"`
	ExpiresAt         time.Time        `json:"expires_at" db:"expires_at"`
}

type RqProcessAction struct {
	TokenID uuid.UUID `json:"token_id" binding:"required"`
	Action  string    `json:"action" binding:"required,oneof=ACCEPT REJECT"`
}

type RqGrantPermission struct {
	AccountantID *uuid.UUID   `json:"accountant_id,omitempty"`
	Email        string       `json:"email" binding:"omitempty,email"`
	Permissions  *Permissions `json:"permissions" binding:"required"`
}

type RqUpdatePermissions struct {
	AccountantID *uuid.UUID   `json:"accountant_id,omitempty"`
	Email        string       `json:"email" binding:"required,email"`
	Permissions  *Permissions `json:"permissions" binding:"required"`
}

// PermissionsContext standardizes unified domain response schemas across modules.
type PermissionsContext struct {
	ID             uuid.UUID   `json:"id"`
	PractitionerID uuid.UUID   `json:"practitioner_id"`
	AccountantID   uuid.UUID   `json:"accountant_id"`
	Permissions    Permissions `json:"permissions"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      *time.Time  `json:"updated_at"`
	DeletedAt      *time.Time  `json:"deleted_at"`
}

var InvitationColumns = map[string]string{
	"email":           "i.email",
	"status":          "i.status::text",
	"created_at":      "i.created_at",
	"updated_at":      "i.updated_at",
	"practitioner_id": "i.practitioner_id",
	"accountant_id":   "i.accountant_id",
	"deleted_at":      "i.deleted_at",
}

var InvitationSearchCols = []string{"email"}

type Filter struct {
	Status *string `form:"status"`
	Role   string  `form:"-"`
	common.Filter
}

func (filter *Filter) MapToFilter(actorID *uuid.UUID) common.Filter {
	filters := make(map[string]interface{})

	if actorID != nil {
		if filter.Role == util.RolePractitioner {
			filters["practitioner_id"] = *actorID
		} else {
			filters["accountant_id"] = *actorID
		}
	}

	if filter.Status != nil {
		filters["status"] = *filter.Status
	}

	f := common.NewFilter(filter.Search, filters, nil, filter.Limit, filter.Offset, filter.SortBy, filter.OrderBy)

	if filter.Status == nil {
		f.Where = append(f.Where, common.Condition{
			Field: "status", Operator: common.OpNotEq, Value: StatusResent,
		})
	}

	return f
}

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
	Name        PermissionName `json:"name" db:"permission_name"`
	AccessLevel AccessLevel    `json:"access_level"`
}

type AccessLevel struct {
	Read  bool `json:"read" db:"can_read"`
	Write bool `json:"write" db:"can_write"`
}

type PermissionRow struct {
	PermissionName PermissionName `db:"permission_name"`
	CanRead        bool           `db:"can_read"`
	CanWrite       bool           `db:"can_write"`
}

type Permissions map[PermissionName]AccessLevel

func (p Permissions) Get(name PermissionName) AccessLevel {
	return p[name]
}

func (p Permissions) Validate() error {
	for _, name := range AllPermissions {
		if _, ok := p[name]; !ok {
			return fmt.Errorf("missing verification permission context block requirement: %q", name)
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

func (p *Permissions) FromRows(rows []PermissionRow) {
	if *p == nil {
		*p = make(Permissions)
	}
	for _, row := range rows {
		(*p)[row.PermissionName] = AccessLevel{
			Read:  row.CanRead,
			Write: row.CanWrite,
		}
	}
}

func (p Permissions) FromRow(row Permission) {
	p[row.Name] = AccessLevel{
		Read:  row.AccessLevel.Read,
		Write: row.AccessLevel.Write,
	}
}

type PractitionerEmailPair struct {
	PractitionerID uuid.UUID        `db:"practitioner_id"`
	Email          string           `db:"email"`
	Status         InvitationStatus `db:"status"`
}

// AccountantInfo holds accountant details for notification purposes
type AccountantInfo struct {
	AccountantID uuid.UUID `db:"accountant_id"`
	UserID       uuid.UUID `db:"user_id"`
}
