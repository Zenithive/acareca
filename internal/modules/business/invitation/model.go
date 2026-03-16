package invitation

import (
	"time"

	"github.com/google/uuid"
)

// InvitationStatus defines the allowed states for an invitation
type InvitationStatus string

const (
	StatusSent      InvitationStatus = "SENT"
	StatusAccepted  InvitationStatus = "ACCEPTED"
	StatusCompleted InvitationStatus = "COMPLETED"
	StatusRejected  InvitationStatus = "REJECTED"
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

// RsInviteProcess helps the frontend navigate after a link click
type RsInviteProcess struct {
	InvitationID uuid.UUID        `json:"invitation_id"`
	Status       InvitationStatus `json:"status"`
	IsFound      bool             `json:"is_found"`
}
