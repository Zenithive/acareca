package field

import (
	"github.com/google/uuid"
)

const (
	SectionTypeCollection = "COLLECTION"
	SectionTypeCost       = "COST"
	SectionTypeOtherCost  = "OTHER_COST"
)

const (
	PaymentResponsibilityOwner  = "OWNER"
	PaymentResponsibilityClinic = "CLINIC"
)

const (
	TaxTypeInclusive = "INCLUSIVE"
	TaxTypeExclusive = "EXCLUSIVE"
	TaxTypeManual    = "MANUAL"
)

type RqFormField struct {
	Label                 string  `json:"label" validate:"required,max=255"`
	SectionType           string  `json:"section_type" validate:"required,oneof=COLLECTION COST OTHER_COST"`
	PaymentResponsibility *string `json:"payment_responsibility" validate:"omitempty,oneof=OWNER CLINIC"`
	TaxType               *string `json:"tax_type" validate:"omitempty,oneof=INCLUSIVE EXCLUSIVE MANUAL"`
	CoaID                 string  `json:"coa_id" validate:"required,uuid"`
}

type RqUpdateFormField struct {
	ID                    uuid.UUID `json:"id" validate:"required,uuid"`
	Label                 *string   `json:"label" validate:"omitempty,max=255"`
	SectionType           *string   `json:"section_type" validate:"omitempty,oneof=COLLECTION COST OTHER_COST"`
	PaymentResponsibility *string   `json:"payment_responsibility" validate:"omitempty,oneof=OWNER CLINIC"`
	TaxType               *string   `json:"tax_type" validate:"omitempty,oneof=INCLUSIVE EXCLUSIVE MANUAL"`
	CoaID                 *string   `json:"coa_id" validate:"omitempty,uuid"`
}

type FormField struct {
	ID                    uuid.UUID `db:"id"`
	FormVersionID         uuid.UUID `db:"form_version_id"`
	Label                 string    `db:"label"`
	SectionType           string    `db:"section_type"`
	PaymentResponsibility *string   `db:"payment_responsibility"`
	TaxType               *string   `db:"tax_type"`
	CoaID                 uuid.UUID `db:"coa_id"`
	CreatedAt             string    `db:"created_at"`
	UpdatedAt             string    `db:"updated_at"`
}

func (r *RqFormField) ToDB(formVersionID uuid.UUID) *FormField {
	coaID, _ := uuid.Parse(r.CoaID)
	return &FormField{
		ID:                    uuid.New(),
		FormVersionID:         formVersionID,
		Label:                 r.Label,
		SectionType:           r.SectionType,
		PaymentResponsibility: r.PaymentResponsibility,
		TaxType:               r.TaxType,
		CoaID:                 coaID,
	}
}

func (d *FormField) ToRs() *RsFormField {
	return &RsFormField{
		ID:                    d.ID,
		FormVersionID:         d.FormVersionID,
		Label:                 d.Label,
		SectionType:           d.SectionType,
		PaymentResponsibility: d.PaymentResponsibility,
		TaxType:               d.TaxType,
		CoaID:                 d.CoaID,
		CreatedAt:             d.CreatedAt,
		UpdatedAt:             d.UpdatedAt,
	}
}

type RsCoaDetail struct {
	ID            uuid.UUID `json:"id"`
	Code          int16     `json:"code"`
	Name          string    `json:"name"`
	AccountTypeID int16     `json:"account_type_id"`
	AccountTaxID  int16     `json:"account_tax_id"`
}

type RsFormField struct {
	ID                    uuid.UUID    `json:"id"`
	FormVersionID         uuid.UUID    `json:"form_version_id"`
	Label                 string       `json:"label"`
	SectionType           string       `json:"section_type"`
	PaymentResponsibility *string      `json:"payment_responsibility"`
	TaxType               *string      `json:"tax_type"`
	CoaID                 uuid.UUID    `json:"coa_id"`
	Coa                   *RsCoaDetail `json:"coa,omitempty"`
	CreatedAt             string       `json:"created_at"`
	UpdatedAt             string       `json:"updated_at"`
}
