package contact

import (
	"github.com/google/uuid"
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
		ID:       uuid.New(),
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
	IsPrimary    bool    `json:"primary" validate:"omitempty"`
	AddressLine1 string  `json:"street" validate:"required"`
	AddressLine2 *string `json:"street2" validate:"omitempty"`
	City         string  `json:"city" validate:"required"`
	State        string  `json:"state" validate:"required"`
	Postcode     string  `json:"postcode" validate:"required"`
	Country      string  `json:"country" validate:"required"`
}

func (r *RqAddress) ToAddress() Address {
	return Address{
		Id:           uuid.New(),
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
	ID       uuid.UUID    `json:"id" validate:"required"`
	ClinicID uuid.UUID    `json:"clinic_id" validate:"required"`
	Fname    *string      `json:"fname" validate:"required"`
	Lname    *string      `json:"lname" validate:"required"`
	Phone    *string      `json:"phone" validate:"required"`
	Email    *string      `json:"email" validate:"required,email"`
	Number   *string      `json:"number" validate:"required"`
	Website  *string      `json:"website" validate:"omitempty,url"`
	ABN      *string      `json:"abn" validate:"omitempty"`
	Note     *string      `json:"note" validate:"omitempty"`
	Address  []*RqAddress `json:"address" validate:"omitempty,dive"`
}

func (r *RqUpdateContact) ToContact() Contact {
	addresses := make([]*Address, 0, len(r.Address))
	for _, addr := range r.Address {
		addresses = append(addresses, lo.ToPtr(addr.ToAddress()))
	}
	return Contact{
		ID:       r.ID,
		ClinicId: r.ClinicID,
		Fname:    lo.FromPtr(r.Fname),
		Lname:    lo.FromPtr(r.Lname),
		Phone:    lo.FromPtr(r.Phone),
		Email:    lo.FromPtr(r.Email),
		Website:  lo.FromPtr(r.Website),
		ABN:      lo.FromPtr(r.ABN),
		Note:     lo.FromPtr(r.Note),
		Address:  addresses,
	}
}

type Contact struct {
	ID       uuid.UUID  `db:"id"`
	ClinicId uuid.UUID  `db:"clinic_id"`
	Fname    string     `db:"fname"`
	Lname    string     `db:"lname"`
	Phone    string     `db:"phone"`
	Email    string     `db:"email"`
	Number   string     `db:"number"`
	Website  string     `db:"website"`
	ABN      string     `db:"abn"`
	Note     string     `db:"note"`
	Address  []*Address `db:"address"`
}

type Address struct {
	Id           uuid.UUID `db:"id"`
	AddressLine1 string    `db:"address_line1"`
	AddressLine2 *string   `db:"address_line2"`
	City         string    `db:"city"`
	State        string    `db:"state"`
	PostalCode   string    `db:"postal_code"`
	Country      string    `db:"country"`
	IsPrimary    bool      `db:"is_primary"`
}

type RsContact struct {
	ID       uuid.UUID    `json:"id"`
	ClinicId uuid.UUID    `json:"clinic_id"`
	Fname    string       `json:"fname"`
	Lname    string       `json:"lname"`
	Phone    string       `json:"phone"`
	Email    string       `json:"email"`
	Number   string       `json:"number"`
	Website  string       `json:"website"`
	ABN      string       `json:"abn"`
	Note     string       `json:"note"`
	Address  []*RsAddress `json:"address"`
}

func (c *Contact) ToRsContact() RsContact {
	addresses := make([]*RsAddress, 0, len(c.Address))
	for _, addr := range c.Address {
		addresses = append(addresses, lo.ToPtr(addr.ToRsAddress()))
	}
	return RsContact{
		ID:       c.ID,
		ClinicId: c.ClinicId,
		Fname:    c.Fname,
		Lname:    c.Lname,
		Phone:    c.Phone,
		Email:    c.Email,
		Number:   c.Number,
		Website:  c.Website,
		ABN:      c.ABN,
		Note:     c.Note,
		Address:  addresses,
	}
}

type RsAddress struct {
	Id           uuid.UUID `json:"id"`
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
		AddressLine1: c.AddressLine1,
		AddressLine2: c.AddressLine2,
		City:         c.City,
		State:        c.State,
		PostalCode:   c.PostalCode,
		Country:      c.Country,
	}
}
