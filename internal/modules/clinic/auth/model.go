package auth

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

const (
	TokenStatusPending = "PENDING"
	TokenStatusUsed    = "USED"
	TokenStatusExpired = "EXPIRED"
	TokenStatusResent  = "RESENT"
)

type RqRegister struct {
	ClinicName  string      `json:"clinic_name" validate:"required,min=2,max=255"`
	Email       string      `json:"email" validate:"required,email"`
	Password    *string     `json:"password,omitempty" validate:"required,min=8"`
	Role        *string     `json:"role,omitempty" validate:"required,oneof=CLINIC"`
	Description *string     `json:"description,omitempty"`
	DocumentID  *string     `json:"document_id,omitempty"`
	ABN         *string     `json:"abn,omitempty" validate:"required,len=11"`
	ACN         *string     `json:"acn,omitempty" validate:"omitempty,len=9"`
	Addresses   []RqAddress `json:"addresses" validate:"omitempty,min=1,dive"`
	Contacts    []RqContact `json:"contacts" validate:"omitempty,min=1,dive"`
}

type RqLogin struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type RqLogout struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type RqChangePassword struct {
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

type RqUpdate struct {
	Id          uuid.UUID            `json:"id" validate:"omitempty"`
	ClinicName  *string              `json:"clinic_name" validate:"omitempty,min=2,max=255"`
	Description *string              `json:"description" validate:"omitempty"`
	DocumentID  *uuid.UUID           `json:"document_id" validate:"omitempty"`
	ABN         *string              `json:"abn" validate:"omitempty"`
	ACN         *string              `json:"acn" validate:"omitempty"`
	Addresses   *RqBulkUpdateAddress `json:"addresses" validate:"omitempty"`
	Contacts    *RqBulkUpdateContact `json:"contacts" validate:"omitempty"`
	UpdatedAt   time.Time            `json:"update_at"`
}

type RqBulkUpdateAddress struct {
	Create []RqAddress       `json:"create" validate:"omitempty,dive"`
	Update []RqUpdateAddress `json:"update" validate:"omitempty,dive"`
	Delete []uuid.UUID       `json:"delete"`
}

type RqBulkUpdateContact struct {
	Create []RqContact       `json:"create" validate:"omitempty,dive"`
	Update []RqUpdateContact `json:"update" validate:"omitempty,dive"`
	Delete []uuid.UUID       `json:"delete"`
}

type RqUpdateAddress struct {
	ID        uuid.UUID `json:"id" validate:"required,uuid"`
	Address   *string   `json:"address" validate:"omitempty"`
	City      *string   `json:"city" validate:"omitempty"`
	State     *string   `json:"state" validate:"omitempty"`
	Postcode  *string   `json:"postcode" validate:"omitempty"`
	IsPrimary *bool     `json:"is_primary" validate:"omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RqUpdateContact struct {
	ID          uuid.UUID `json:"id" validate:"required,uuid"`
	ContactType *string   `json:"contact_type" validate:"oneof=PHONE WEBSITE"`
	Value       *string   `json:"value"`
	Label       *string   `json:"label,omitempty"`
	IsPrimary   *bool     `json:"is_primary"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type RqForgotPassword struct {
	Email string `json:"email" validate:"required,email"`
}

type RqResetPassword struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

type RqAddress struct {
	Address   string `json:"address"`
	City      string `json:"city"`
	State     string `json:"state"`
	Postcode  string `json:"postcode"`
	IsPrimary bool   `json:"is_primary"`
}

type RqContact struct {
	ContactType string  `json:"contact_type" validate:"oneof=PHONE WEBSITE"`
	Value       string  `json:"value"`
	Label       *string `json:"label,omitempty"`
	IsPrimary   bool    `json:"is_primary"`
}

type RqClinic struct {
	DocumentId *uuid.UUID `json:"document_id"`
	Name       string     `json:"name" validate:"required, min=3, max=100"`
	Email      string     `json:"email" validate:"required,email"`
	Password   string     `json:"password" validate:"required,min=8"`
}

// DB
type Clinic struct {
	ID          uuid.UUID  `db:"id"`
	DocumentID  *uuid.UUID `db:"document_id"`
	ClinicName  string     `db:"clinic_name"`
	Description *string    `db:"description"`
	Email       string     `db:"email"`
	Password    *string    `db:"password"`
	Role        *string    `db:"role"`
	Verified    bool       `db:"verified"`
	ABN         *string    `db:"abn"`
	ACN         *string    `db:"acn"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   *time.Time `db:"updated_at"`
}

func (r *RqRegister) MapToDB(hashedPassword string) Clinic {
	var documentID *uuid.UUID
	if r.DocumentID != nil {
		if id, err := uuid.Parse(*r.DocumentID); err == nil {
			documentID = &id
		}
	}

	return Clinic{
		ID:          uuid.New(),
		DocumentID:  documentID,
		ClinicName:  r.ClinicName,
		Description: r.Description,
		Email:       r.Email,
		Password:    &hashedPassword,
		Role:        r.Role,
		Verified:    false,
		ABN:         r.ABN,
		ACN:         r.ACN,
		CreatedAt:   time.Now(),
	}
}

func (r *RqUpdate) MapToDB(existing Clinic) Clinic {
	updated := existing
	updated.ID = r.Id

	if r.ClinicName != nil {
		updated.ClinicName = *r.ClinicName
	}
	if r.Description != nil {
		updated.Description = r.Description
	}
	if r.DocumentID != nil {
		updated.DocumentID = r.DocumentID
	}
	if r.ABN != nil {
		updated.ABN = r.ABN
	}
	if r.ACN != nil {
		updated.ACN = r.ACN
	}

	now := r.UpdatedAt
	if now.IsZero() {
		now = time.Now()
	}
	updated.UpdatedAt = &now

	return updated
}

type Address struct {
	ID        uuid.UUID  `db:"id"`
	ClinicID  uuid.UUID  `db:"clinic_id"`
	Address   string     `db:"address"`
	City      string     `db:"city"`
	State     string     `db:"state"`
	Postcode  string     `db:"postcode"`
	IsPrimary bool       `db:"is_primary"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
}

func (r *RqAddress) MapToDB(clinicID uuid.UUID) Address {
	return Address{
		ID:        uuid.New(),
		ClinicID:  clinicID,
		Address:   r.Address,
		City:      r.City,
		State:     r.State,
		Postcode:  r.Postcode,
		IsPrimary: r.IsPrimary,
		CreatedAt: time.Now(),
	}
}

func (r *RqUpdateAddress) MapToDB(existing Address) Address {
	updated := existing
	updated.ID = r.ID

	if r.Address != nil {
		updated.Address = *r.Address
	}
	if r.City != nil {
		updated.City = *r.City
	}
	if r.State != nil {
		updated.State = *r.State
	}
	if r.Postcode != nil {
		updated.Postcode = *r.Postcode
	}
	if r.IsPrimary != nil {
		updated.IsPrimary = *r.IsPrimary
	}

	now := r.UpdatedAt
	if now.IsZero() {
		now = time.Now()
	}
	updated.UpdatedAt = &now

	return updated
}

type Contact struct {
	ID          uuid.UUID  `db:"id"`
	ClinicID    uuid.UUID  `db:"clinic_id"`
	ContactType string     `db:"contact_type"`
	Value       string     `db:"value"`
	Label       *string    `db:"label"`
	IsPrimary   bool       `db:"is_primary"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   *time.Time `db:"updated_at"`
}

func (r *RqContact) MapToDB(clinicID uuid.UUID) Contact {
	return Contact{
		ID:          uuid.New(),
		ClinicID:    clinicID,
		ContactType: r.ContactType,
		Value:       r.Value,
		Label:       r.Label,
		IsPrimary:   r.IsPrimary,
		CreatedAt:   time.Now(),
	}
}

func (r *RqUpdateContact) MapToDB(existing Contact) Contact {
	updated := existing
	updated.ID = r.ID

	if r.ContactType != nil {
		updated.ContactType = *r.ContactType
	}
	if r.Value != nil {
		updated.Value = *r.Value
	}
	if r.Label != nil {
		updated.Label = r.Label
	}
	if r.IsPrimary != nil {
		updated.IsPrimary = *r.IsPrimary
	}

	now := r.UpdatedAt
	if now.IsZero() {
		now = time.Now()
	}
	updated.UpdatedAt = &now

	return updated
}

type Session struct {
	ID           uuid.UUID  `db:"id"`
	ClinicID     uuid.UUID  `db:"clinic_id"`
	RefreshToken string     `db:"refresh_token"`
	UserAgent    *string    `db:"user_agent"`
	IPAddress    *string    `db:"ip_address"`
	ExpiresAt    time.Time  `db:"expires_at"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    *time.Time `db:"updated_at"`
}

type VerificationToken struct {
	ID        uuid.UUID `db:"id"`
	ClinicID  uuid.UUID `db:"clinic_id"`
	Role      *string   `db:"role"`
	Status    string    `db:"status"`
	CreatedAt time.Time `db:"created_at"`
	ExpiresAt time.Time `db:"expires_at"`
}

// Response
type RsToken struct {
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token"`
	Role         *string `json:"role"`
}

type RsClinic struct {
	ID          uuid.UUID          `json:"id"`
	ClinicName  string             `json:"clinic_name"`
	Description *string            `json:"description,omitempty"`
	Email       string             `json:"email"`
	Role        *string            `json:"role"`
	Verified    bool               `json:"verified"`
	Document    *common.RsDocument `json:"document,omitempty"`
	ABN         *string            `json:"abn,omitempty"`
	ACN         *string            `json:"acn,omitempty"`
	Addresses   []RsAddress        `json:"addresses"`
	Contacts    []RsContact        `json:"contacts"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   *time.Time         `json:"updated_at,omitempty"`
}

func (c *Clinic) MapToRs(addresses []Address, contacts []Contact, document *common.RsDocument) RsClinic {
	rsAddresses := make([]RsAddress, 0, len(addresses))
	for _, a := range addresses {
		rsAddresses = append(rsAddresses, a.MapToRs())
	}

	rsContacts := make([]RsContact, 0, len(contacts))
	for _, ct := range contacts {
		rsContacts = append(rsContacts, ct.MapToRs())
	}

	return RsClinic{
		ID:          c.ID,
		ClinicName:  c.ClinicName,
		Description: c.Description,
		Email:       c.Email,
		Role:        c.Role,
		Verified:    c.Verified,
		ABN:         c.ABN,
		ACN:         c.ACN,
		Addresses:   rsAddresses,
		Document:    document,
		Contacts:    rsContacts,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

type RsAddress struct {
	ID        uuid.UUID  `json:"id"`
	Address   string     `json:"address"`
	City      string     `json:"city"`
	State     string     `json:"state"`
	Postcode  string     `json:"postcode"`
	IsPrimary bool       `json:"is_primary"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}

func (a *Address) MapToRs() RsAddress {
	return RsAddress{
		ID:        a.ID,
		Address:   a.Address,
		City:      a.City,
		State:     a.State,
		Postcode:  a.Postcode,
		IsPrimary: a.IsPrimary,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
}

type RsContact struct {
	ID          uuid.UUID  `json:"id"`
	ContactType string     `json:"contact_type"`
	Value       string     `json:"value"`
	Label       *string    `json:"label,omitempty"`
	IsPrimary   bool       `json:"is_primary"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at"`
}

func (c *Contact) MapToRs() RsContact {
	return RsContact{
		ID:          c.ID,
		ContactType: c.ContactType,
		Value:       c.Value,
		Label:       c.Label,
		IsPrimary:   c.IsPrimary,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}
