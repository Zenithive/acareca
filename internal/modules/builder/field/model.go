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
	Label                 string `json:"label" validate:"required,max=255"`
	SectionType           string `json:"section_type" validate:"required,oneof=COLLECTION COST OTHER_COST"`
	PaymentResponsibility string `json:"payment_responsibility" validate:"required,oneof=OWNER CLINIC"`
	TaxType               string `json:"tax_type" validate:"required,oneof=INCLUSIVE EXCLUSIVE MANUAL"`
	CoaID                 string `json:"coa_id" validate:"required,uuid"`
	SortOrder             *int   `json:"sort_order" validate:"omitempty,min=0"`
}

type RqUpdateFormField struct {
	ID                    uuid.UUID `json:"id" validate:"required,uuid"`
	Label                 *string   `json:"label" validate:"omitempty,max=255"`
	SectionType           *string   `json:"section_type" validate:"omitempty,oneof=COLLECTION COST OTHER_COST"`
	PaymentResponsibility *string   `json:"payment_responsibility" validate:"omitempty,oneof=OWNER CLINIC"`
	TaxType               *string   `json:"tax_type" validate:"omitempty,oneof=INCLUSIVE EXCLUSIVE MANUAL"`
	CoaID                 *string   `json:"coa_id" validate:"omitempty,uuid"`
	SortOrder             *int      `json:"sort_order" validate:"omitempty,min=0"`
}

type FormField struct {
	ID                    uuid.UUID `db:"id" json:"id"`
	FormVersionID         uuid.UUID `db:"form_version_id" json:"form_version_id"`
	Label                 string    `db:"label" json:"label"`
	SectionType           string    `db:"section_type" json:"section_type"`
	PaymentResponsibility string    `db:"payment_responsibility" json:"payment_responsibility"`
	TaxType               string    `db:"tax_type" json:"tax_type"`
	CoaID                 uuid.UUID `db:"coa_id" json:"coa_id"`
	SortOrder             int       `db:"sort_order" json:"sort_order"`
	CreatedAt             string    `db:"created_at" json:"created_at"`
	UpdatedAt             string    `db:"updated_at" json:"updated_at"`
}

func (r *RqFormField) ToDB(formVersionID uuid.UUID) *FormField {
	coaID, _ := uuid.Parse(r.CoaID)
	sortOrder := 0
	if r.SortOrder != nil {
		sortOrder = *r.SortOrder
	}
	return &FormField{
		ID:                    uuid.New(),
		FormVersionID:         formVersionID,
		Label:                 r.Label,
		SectionType:           r.SectionType,
		PaymentResponsibility: r.PaymentResponsibility,
		TaxType:               r.TaxType,
		CoaID:                 coaID,
		SortOrder:             sortOrder,
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
		SortOrder:             d.SortOrder,
		CreatedAt:             d.CreatedAt,
		UpdatedAt:             d.UpdatedAt,
	}
}

type RsFormField struct {
	ID                    uuid.UUID `json:"id"`
	FormVersionID         uuid.UUID `json:"form_version_id"`
	Label                 string    `json:"label"`
	SectionType           string    `json:"section_type"`
	PaymentResponsibility string    `json:"payment_responsibility"`
	TaxType               string    `json:"tax_type"`
	CoaID                 uuid.UUID `json:"coa_id"`
	SortOrder             int       `json:"sort_order"`
	CreatedAt             string    `json:"created_at"`
	UpdatedAt             string    `json:"updated_at"`
}
