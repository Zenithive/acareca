package invitation

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

// InvitationStatus defines the allowed states for an invitation
type InvitationStatus string

const (
	StatusSent      InvitationStatus = "SENT"
	StatusAccepted  InvitationStatus = "ACCEPTED"
	StatusCompleted InvitationStatus = "COMPLETED"
	StatusRejected  InvitationStatus = "REJECTED"
	StatusResent    InvitationStatus = "RESENT"
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
	Email string `json:"email" validate:"required,email"`
}

// RsInvitation is the response after an invitation is created
type RsInvitation struct {
	ID         uuid.UUID        `json:"id"`
	Email      string           `json:"email"`
	InviteLink string           `json:"invite_link"`
	Status     InvitationStatus `json:"status"`
	ExpiresAt  time.Time        `json:"expires_at"`
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
}

// RsInviteProcess helps the frontend navigate after a link click
type RsInviteProcess struct {
	InvitationID   uuid.UUID        `json:"invitation_id"`
	PractitionerID uuid.UUID        `json:"practitioner_id" db:"practitioner_id"`
	Email          string           `json:"email" db:"email"`
	Status         InvitationStatus `json:"status"`
	IsFound        bool             `json:"is_found"`
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
	"status":          "status",
	"created_at":      "created_at",
	"practitioner_id": "practitioner_id",
	"entity_id":       "entity_id",
}

var invitationSearchCols = []string{"email"}

type Filter struct {
	Status *string `form:"status"`
	common.Filter
}

func (filter *Filter) MapToFilter(pID, aID *uuid.UUID) common.Filter {
	filters := map[string]interface{}{}

	// Role-based security: Apply the correct ID based on who is asking
	if pID != nil {
		filters["practitioner_id"] = *pID
	} else if aID != nil {
		filters["entity_id"] = *aID
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
