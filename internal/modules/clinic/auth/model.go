package auth

import (
	"time"

	"github.com/google/uuid"
)

// For email verification token operations
const (
	TokenStatusPending = "PENDING"
	TokenStatusUsed    = "USED"
	TokenStatusExpired = "EXPIRED"
	TokenStatusResent  = "RESENT"
)

// ==========================================
// REQUEST STRUCTURES
// ==========================================

type RqRegisterClinic struct {
	ClinicName  string            `json:"clinic_name" validate:"required,min=2,max=255"`
	Email       string            `json:"email" validate:"required,email"`
	Password    *string           `json:"password,omitempty" validate:"required,min=8"`
	Role        *string           `json:"role,omitempty" validate:"required,oneof=CLINIC"`
	Description *string           `json:"description,omitempty"`
	DocumentID  *string           `json:"document_id,omitempty"`
	ABN         *string           `json:"abn,omitempty" validate:"required,len=11"`
	ACN         *string           `json:"acn,omitempty" validate:"omitempty,len=9"`
	Addresses   []RqClinicAddress `json:"addresses" validate:"omitempty,min=1,dive"`
	Contacts    []RqClinicContact `json:"contacts" validate:"omitempty,min=1,dive"`
}

type RqLoginClinic struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type RqLogoutClinic struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type RqChangePassword struct {
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

type RqUpdateClinic struct {
	ClinicName  *string                    `json:"clinic_name" validate:"omitempty,min=2,max=255"`
	Description *string                    `json:"description,omitempty"`
	DocumentID  *string                    `json:"document_id,omitempty"`
	ABN         *string                    `json:"abn,omitempty"`
	ACN         *string                    `json:"acn,omitempty"`
	Addresses   *RqAddressChangeset        `json:"addresses,omitempty"`
	Contacts    *RqContactChangeset        `json:"contacts,omitempty"`
}

// RqAddressChangeset holds the three address operation buckets.
type RqAddressChangeset struct {
	Create []RqClinicAddress       `json:"create" validate:"omitempty,dive"`
	Update []RqUpdateClinicAddress `json:"update" validate:"omitempty,dive"`
	Delete []string                `json:"delete"` // address IDs (UUID strings)
}

// RqContactChangeset holds the three contact operation buckets.
type RqContactChangeset struct {
	Create []RqClinicContact       `json:"create" validate:"omitempty,dive"`
	Update []RqUpdateClinicContact `json:"update" validate:"omitempty,dive"`
	Delete []string                `json:"delete"` // contact IDs (UUID strings)
}

type RqUpdateClinicAddress struct {
	ID        string `json:"id" validate:"required,uuid"`
	Address   string `json:"address"`
	City      string `json:"city"`
	State     string `json:"state"`
	Postcode  string `json:"postcode"`
	IsPrimary bool   `json:"is_primary"`
}

type RqUpdateClinicContact struct {
	ID          string  `json:"id" validate:"required,uuid"`
	ContactType string  `json:"contact_type" validate:"oneof=PHONE WEBSITE"`
	Value       string  `json:"value"`
	Label       *string `json:"label,omitempty"`
	IsPrimary   bool    `json:"is_primary"`
}

type RqForgotPassword struct {
	Email string `json:"email" binding:"required,email"`
}

type RqResetPassword struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

type RqClinicAddress struct {
	Address   string `json:"address"`
	City      string `json:"city"`
	State     string `json:"state"`
	Postcode  string `json:"postcode"`
	IsPrimary bool   `json:"is_primary"`
}

type RqClinicContact struct {
	ContactType string  `json:"contact_type" validate:"oneof=PHONE WEBSITE"`
	Value       string  `json:"value"`
	Label       *string `json:"label,omitempty"`
	IsPrimary   bool    `json:"is_primary"`
}

// ==========================================
// DATABASE ENTITIES
// ==========================================

type Clinic struct {
	ID          uuid.UUID `db:"id"`
	DocumentID  *string   `db:"document_id"`
	ClinicName  string    `db:"clinic_name"`
	Description *string   `db:"description"`
	Email       string    `db:"email"`
	Password    *string   `db:"password"`
	Role        *string   `db:"role"`
	Verified    bool      `db:"verified"`
	ABN         *string   `db:"abn"`
	ACN         *string   `db:"acn"`
	CreatedAt   string    `db:"created_at"`
	UpdatedAt   *string   `db:"updated_at"`
	DeletedAt   *string   `db:"deleted_at"`
}

type ClinicAddress struct {
	ID        uuid.UUID `db:"id"`
	ClinicID  uuid.UUID `db:"clinic_id"`
	Address   string    `db:"address"`
	City      string    `db:"city"`
	State     string    `db:"state"`
	Postcode  string    `db:"postcode"`
	IsPrimary bool      `db:"is_primary"`
	CreatedAt string    `db:"created_at"`
	UpdatedAt *string   `db:"updated_at"`
	DeletedAt *string   `db:"deleted_at"`
}

type ClinicContact struct {
	ID          uuid.UUID `db:"id"`
	ClinicID    uuid.UUID `db:"clinic_id"`
	ContactType string    `db:"contact_type"`
	Value       string    `db:"value"`
	Label       *string   `db:"label"`
	IsPrimary   bool      `db:"is_primary"`
	CreatedAt   string    `db:"created_at"`
	UpdatedAt   *string   `db:"updated_at"`
	DeletedAt   *string   `db:"deleted_at"`
}

type Session struct {
	ID           uuid.UUID  `db:"id"`
	ClinicID     uuid.UUID  `db:"clinic_id"`
	RefreshToken string     `db:"refresh_token"`
	UserAgent    *string    `db:"user_agent"`
	IPAddress    *string    `db:"ip_address"`
	ExpiresAt    time.Time  `db:"expires_at"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
	DeletedAt    *time.Time `db:"deleted_at"`
}

type VerificationToken struct {
	ID        uuid.UUID `db:"id"`
	ClinicID  uuid.UUID `db:"clinic_id"`
	Role      *string   `db:"role"`
	Status    string    `db:"status"`
	CreatedAt time.Time `db:"created_at"`
	ExpiresAt time.Time `db:"expires_at"`
}

// ==========================================
// RESPONSE STRUCTURES
// ==========================================

type RsToken struct {
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token"`
	Role         *string `json:"role"`
}

type RsClinicDetail struct {
	ID          uuid.UUID         `json:"id"`
	ClinicName  string            `json:"clinic_name"`
	Description *string           `json:"description,omitempty"`
	Email       string            `json:"email"`
	Role        *string           `json:"role"`
	Verified    bool              `json:"verified"`
	DocumentID  *string           `json:"document_id,omitempty"`
	ABN         *string           `json:"abn,omitempty"`
	ACN         *string           `json:"acn,omitempty"`
	Addresses   []RsClinicAddress `json:"addresses"`
	Contacts    []RsClinicContact `json:"contacts"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   *string           `json:"updated_at,omitempty"`
}

type RsClinicAddress struct {
	ID        uuid.UUID `json:"id"`
	Address   string    `json:"address"`
	City      string    `json:"city"`
	State     string    `json:"state"`
	Postcode  string    `json:"postcode"`
	IsPrimary bool      `json:"is_primary"`
}

type RsClinicContact struct {
	ID          uuid.UUID `json:"id"`
	ContactType string    `json:"contact_type"`
	Value       string    `json:"value"`
	Label       *string   `json:"label,omitempty"`
	IsPrimary   bool      `json:"is_primary"`
}
