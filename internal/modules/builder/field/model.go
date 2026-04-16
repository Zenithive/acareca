package field

import (
	"errors"

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

// RqCreateField is the unified create request supporting both raw and computed fields.
type RqCreateField struct {
	FieldKey              string  `json:"key" validate:"required,max=5"`
	Slug                  string  `json:"slug" validate:"omitempty,max=100"`
	Label                 string  `json:"label" validate:"required,max=255"`
	IsComputed            bool    `json:"is_computed"`
	SectionType           string  `json:"section_type" validate:"omitempty,oneof=COLLECTION COST OTHER_COST"`
	PaymentResponsibility *string `json:"payment_responsibility" validate:"omitempty,oneof=OWNER CLINIC"`
	TaxType               *string `json:"tax_type" validate:"omitempty"`
	CoaID                 *string `json:"coa_id" validate:"omitempty,uuid"`
	SortOrder             int     `json:"sort_order" validate:"min=0"`
	IsFormula             *bool   `json:"is_formula"`
	IsHighlighted         bool    `json:"is_highlighted"`
}

func (r *RqCreateField) Validate() error {
	if !r.IsComputed {
		if r.CoaID == nil || *r.CoaID == "" {
			return errors.New("coa_id is required for non-computed fields")
		}
		if r.SectionType == "" {
			return errors.New("section_type is required for non-computed fields")
		}
	}
	return nil
}

func (r *RqCreateField) Sanitize() {
	if r.TaxType != nil && *r.TaxType == "" {
		r.TaxType = nil
	}
	if r.PaymentResponsibility != nil && *r.PaymentResponsibility == "" {
		r.PaymentResponsibility = nil
	}
}

// ToRqFormField converts to the legacy RqFormField for non-computed fields.
func (r *RqCreateField) ToRqFormField() *RqFormField {
	coaID := ""
	if r.CoaID != nil {
		coaID = *r.CoaID
	}
	return &RqFormField{
		FieldKey:              r.FieldKey,
		Slug:                  r.Slug,
		Label:                 r.Label,
		IsComputed:            r.IsComputed,
		SectionType:           r.SectionType,
		PaymentResponsibility: r.PaymentResponsibility,
		TaxType:               r.TaxType,
		CoaID:                 coaID,
		SortOrder:             r.SortOrder,
		IsFormula:             r.IsFormula,
		IsHighlighted:         r.IsHighlighted,
	}
}

type RqFormField struct {
	FieldKey              string  `json:"key" validate:"required,max=5"`
	Slug                  string  `json:"slug" validate:"omitempty,max=100"`
	Label                 string  `json:"label" validate:"required,max=255"`
	IsComputed            bool    `json:"is_computed"`
	SectionType           string  `json:"section_type" validate:"omitempty,oneof=COLLECTION COST OTHER_COST"`
	PaymentResponsibility *string `json:"payment_responsibility" validate:"omitempty,oneof=OWNER CLINIC"`
	TaxType               *string `json:"tax_type" validate:"omitempty"`
	CoaID                 string  `json:"coa_id" validate:"omitempty,uuid"`
	SortOrder             int     `json:"sort_order" validate:"min=0"`
	IsFormula             *bool   `json:"is_formula"`
	IsHighlighted         bool    `json:"is_highlighted"`
}

func (r *RqFormField) Sanitize() {
	if r.TaxType != nil && *r.TaxType == "" {
		r.TaxType = nil
	}
	if r.PaymentResponsibility != nil && *r.PaymentResponsibility == "" {
		r.PaymentResponsibility = nil
	}

	// if r.IsFormula != nil {
	// 	r.IsFormula = nil
	// }
}

type RqUpdateFormField struct {
	ID                    uuid.UUID `json:"id" validate:"required,uuid"`
	Label                 *string   `json:"label" validate:"omitempty,max=255"`
	SectionType           *string   `json:"section_type" validate:"omitempty,oneof=COLLECTION COST OTHER_COST"`
	PaymentResponsibility *string   `json:"payment_responsibility" validate:"omitempty,oneof=OWNER CLINIC"`
	TaxType               *string   `json:"tax_type" validate:"omitempty"`
	CoaID                 *string   `json:"coa_id" validate:"omitempty,uuid"`
	SortOrder             *int      `json:"sort_order" validate:"omitempty,min=0"`
	IsHighlighted         *bool     `json:"is_highlighted"`
}

// Sanitize normalizes empty string pointer fields to nil so omitempty validation works correctly.
func (r *RqUpdateFormField) Sanitize() {
	if r.SectionType != nil && *r.SectionType == "" {
		r.SectionType = nil
	}
	if r.TaxType != nil && *r.TaxType == "" {
		r.TaxType = nil
	}
	if r.PaymentResponsibility != nil && *r.PaymentResponsibility == "" {
		r.PaymentResponsibility = nil
	}
	if r.CoaID != nil && *r.CoaID == "" {
		r.CoaID = nil
	}

}

type FormField struct {
	ID                    uuid.UUID  `db:"id"`
	FormVersionID         uuid.UUID  `db:"form_version_id"`
	FieldKey              string     `db:"field_key"`
	Slug                  *string    `db:"slug"`
	Label                 string     `db:"label"`
	IsComputed            bool       `db:"is_computed"`
	IsFormula             bool       `db:"is_formula"`
	IsHighlighted         bool       `db:"is_highlighted"`
	SectionType           *string    `db:"section_type"`
	PaymentResponsibility *string    `db:"payment_responsibility"`
	TaxType               *string    `db:"tax_type"`
	CoaID                 *uuid.UUID `db:"coa_id"`
	SortOrder             int        `db:"sort_order"`
	CreatedAt             string     `db:"created_at"`
	UpdatedAt             string     `db:"updated_at"`
}

func (r *RqFormField) ToDB(formVersionID uuid.UUID) *FormField {
	var slug *string
	if r.Slug != "" {
		slug = &r.Slug
	}
	var sectionType *string
	if r.SectionType != "" {
		sectionType = &r.SectionType
	}
	var coaID *uuid.UUID
	if r.CoaID != "" {
		id, err := uuid.Parse(r.CoaID)
		if err == nil {
			coaID = &id
		}
	}

	var formula bool
	if r.IsFormula != nil {
		formula = *r.IsFormula
	} else {
		formula = false
	}

	return &FormField{
		ID:                    uuid.New(),
		FormVersionID:         formVersionID,
		FieldKey:              r.FieldKey,
		Slug:                  slug,
		Label:                 r.Label,
		IsComputed:            r.IsComputed,
		SectionType:           sectionType,
		PaymentResponsibility: r.PaymentResponsibility,
		TaxType:               r.TaxType,
		CoaID:                 coaID,
		SortOrder:             r.SortOrder,
		IsFormula:             formula,
		IsHighlighted:         r.IsHighlighted,
	}
}

func (d *FormField) ToRs() *RsFormField {
	return &RsFormField{
		ID:                    d.ID,
		FormVersionID:         d.FormVersionID,
		FieldKey:              d.FieldKey,
		Slug:                  d.Slug,
		Label:                 d.Label,
		IsComputed:            d.IsComputed,
		IsFormula:             d.IsFormula,
		IsHighlighted:         d.IsHighlighted,
		SectionType:           d.SectionType,
		PaymentResponsibility: d.PaymentResponsibility,
		TaxType:               d.TaxType,
		CoaID:                 d.CoaID,
		SortOrder:             d.SortOrder,
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
	FieldKey              string       `json:"key"`
	Slug                  *string      `json:"slug"`
	Label                 string       `json:"label"`
	IsComputed            bool         `json:"is_computed"`
	IsFormula             bool         `json:"is_formula"`
	IsHighlighted         bool         `json:"is_highlighted"`
	SectionType           *string      `json:"section_type"`
	PaymentResponsibility *string      `json:"payment_responsibility"`
	TaxType               *string      `json:"tax_type"`
	CoaID                 *uuid.UUID   `json:"coa_id"`
	Coa                   *RsCoaDetail `json:"coa,omitempty"`
	SortOrder             int          `json:"sort_order"`
	CreatedAt             string       `json:"created_at"`
	UpdatedAt             string       `json:"updated_at"`
}
