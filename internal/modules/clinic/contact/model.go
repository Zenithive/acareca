package contact

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/samber/lo"
)

type RqContact struct {
	ClinicID uuid.UUID    `json:"clinic_id" validate:"required"`
	Fname    string       `json:"fname" validate:"required"`
	Lname    string       `json:"lname" validate:"required"`
	Phone    string       `json:"phone" validate:"required"`
	Email    string       `json:"email" validate:"required,email"`
	Website  string       `json:"website" validate:"omitempty,url"`
	ABN      string       `json:"abn" validate:"omitempty"`
	Note     string       `json:"note" validate:"omitempty"`
	Address  []*RqAddress `json:"address" validate:"omitempty,dive"`
}

func (r *RqContact) ToContact() Contact {

	addresses := make([]*Address, 0, len(r.Address))

	for _, addr := range r.Address {
		addresses = append(addresses, lo.ToPtr(addr.ToAddress()))
	}

	return Contact{
		ClinicId: r.ClinicID,
		Fname:    r.Fname,
		Lname:    r.Lname,
		Phone:    r.Phone,
		Email:    r.Email,
		Website:  r.Website,
		ABN:      r.ABN,
		Note:     r.Note,
		Address:  addresses,
	}
}

type RqAddress struct {
	Id           *uuid.UUID `json:"id,omitempty"`
	IsPrimary    bool       `json:"primary"`
	AddressLine1 string     `json:"address1" validate:"required"`
	AddressLine2 *string    `json:"address2"`
	City         string     `json:"city" validate:"required"`
	State        string     `json:"state" validate:"required"`
	Postcode     string     `json:"postcode" validate:"required"`
	Country      string     `json:"country" validate:"required"`
}

func (r *RqAddress) ToAddress() Address {

	var id uuid.UUID

	if r.Id != nil {
		id = *r.Id
	}

	return Address{
		Id:           id,
		AddressLine1: r.AddressLine1,
		AddressLine2: r.AddressLine2,
		City:         r.City,
		State:        r.State,
		PostalCode:   r.Postcode,
		Country:      r.Country,
		IsPrimary:    r.IsPrimary,
	}
}

type RqUpdateContact struct {
	ID       uuid.UUID    `json:"id" validate:"-"`
	ClinicID *uuid.UUID   `json:"clinic_id,omitempty"`
	Fname    *string      `json:"fname,omitempty"`
	Lname    *string      `json:"lname,omitempty"`
	Phone    *string      `json:"phone,omitempty"`
	Email    *string      `json:"email,omitempty" validate:"omitempty,email"`
	Website  *string      `json:"website" validate:"omitempty,url"`
	ABN      *string      `json:"abn" validate:"omitempty"`
	Note     *string      `json:"note" validate:"omitempty"`
	Address  []*RqAddress `json:"address" validate:"omitempty,dive"`
}

func (r *RqUpdateContact) ApplyToContact(contact Contact) Contact {
	contact.ID = r.ID
	if r.ClinicID != nil {
		contact.ClinicId = *r.ClinicID
	}
	if r.Fname != nil {
		contact.Fname = *r.Fname
	}
	if r.Lname != nil {
		contact.Lname = *r.Lname
	}
	if r.Phone != nil {
		contact.Phone = *r.Phone
	}
	if r.Email != nil {
		contact.Email = *r.Email
	}
	if r.Website != nil {
		contact.Website = *r.Website
	}
	if r.ABN != nil {
		contact.ABN = *r.ABN
	}
	if r.Note != nil {
		contact.Note = *r.Note
	}

	if r.Address != nil {
		addresses := make([]*Address, 0, len(r.Address))
		for _, addr := range r.Address {
			addresses = append(addresses, lo.ToPtr(addr.ToAddress()))
		}
		contact.Address = addresses
	}

	return contact
}

type Contact struct {
	ID        uuid.UUID  `db:"id"`
	ClinicId  uuid.UUID  `db:"clinic_id"`
	Fname     string     `db:"fname"`
	Lname     string     `db:"lname"`
	Phone     string     `db:"phone"`
	Email     string     `db:"email"`
	Website   string     `db:"website"`
	ABN       string     `db:"abn"`
	Note      string     `db:"note"`
	Address   []*Address `db:"address"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

type Address struct {
	Id           uuid.UUID  `db:"id"`
	ContactID    uuid.UUID  `db:"contact_id"`
	AddressLine1 string     `db:"address_line1"`
	AddressLine2 *string    `db:"address_line2"`
	City         string     `db:"city"`
	State        string     `db:"state"`
	PostalCode   string     `db:"postal_code"`
	Country      string     `db:"country"`
	IsPrimary    bool       `db:"is_primary"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
	DeletedAt    *time.Time `db:"deleted_at"`
}

type RsContact struct {
	ID        uuid.UUID    `json:"id"`
	ClinicId  uuid.UUID    `json:"clinic_id"`
	Fname     string       `json:"fname"`
	Lname     string       `json:"lname"`
	Phone     string       `json:"phone"`
	Email     string       `json:"email"`
	Website   string       `json:"website"`
	ABN       string       `json:"abn"`
	Note      string       `json:"note"`
	Address   []*RsAddress `json:"address"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

func (c *Contact) ToRsContact() RsContact {

	addresses := make([]*RsAddress, 0, len(c.Address))

	for _, addr := range c.Address {
		addresses = append(addresses, lo.ToPtr(addr.ToRsAddress()))
	}

	return RsContact{
		ID:        c.ID,
		ClinicId:  c.ClinicId,
		Fname:     c.Fname,
		Lname:     c.Lname,
		Phone:     c.Phone,
		Email:     c.Email,
		Website:   c.Website,
		ABN:       c.ABN,
		Note:      c.Note,
		Address:   addresses,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

type RsAddress struct {
	Id           uuid.UUID `json:"id"`
	IsPrimary    bool      `json:"primary"`
	AddressLine1 string    `json:"street"`
	AddressLine2 *string   `json:"street2"`
	City         string    `json:"city"`
	State        string    `json:"state"`
	PostalCode   string    `json:"postcode"`
	Country      string    `json:"country"`
}

func (c *Address) ToRsAddress() RsAddress {

	return RsAddress{
		Id:           c.Id,
		IsPrimary:    c.IsPrimary,
		AddressLine1: c.AddressLine1,
		AddressLine2: c.AddressLine2,
		City:         c.City,
		State:        c.State,
		PostalCode:   c.PostalCode,
		Country:      c.Country,
	}
}

type Filter struct {
	ClinicID *uuid.UUID `form:"clinic_id"`
	Fname    *string    `form:"fname"`
	Email    *string    `form:"email"`
	common.Filter
}

func (filter *Filter) MapToFilter() common.Filter {
	filters := map[string]interface{}{}
	operators := map[string]common.Operator{}

	// Name filter - partial match on full name
	if filter.Fname != nil && *filter.Fname != "" {
		filters["fname"] = "%" + *filter.Fname + "%"
		operators["fname"] = common.OpLike
	}
	if filter.Email != nil && *filter.Email != "" {
		filters["email"] = "%" + *filter.Email + "%"
		operators["email"] = common.OpLike
	}

	return common.NewFilter(filter.Search, filters, operators, filter.Limit, filter.Offset, filter.SortBy, filter.OrderBy)
}
