package section

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/item"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

// TaxMethod represents the tax calculation method
type TaxMethod string

const (
	NoTax     TaxMethod = "NO_TAX"
	Inclusive TaxMethod = "INCLUSIVE"
	Exclusive TaxMethod = "EXCLUSIVE"
)

// SectionType represents the type of invoice section
type SectionType string

const (
	CALCULATIONSTATEMENT SectionType = "CALCULATION_STATEMENT"
	TAXINVOICE           SectionType = "TAX_INVOICE"
	REMITTANCEINVOICE    SectionType = "REMITTANCE_ADVICE"
	RCTI                 SectionType = "RCTI"
)

// RqSection represents the request payload for creating a section
type RqSection struct {
	InvoiceID        *uuid.UUID      `json:"invoiceId,omitempty"`
	TemplateID       uuid.UUID       `json:"templateId,omitempty"`
	SectionType      SectionType     `json:"sectionType" `
	DocumentNumber   string          `json:"documentNumber" validate:"required"`
	TaxMethod        *TaxMethod      `json:"taxMethod,omitempty" validate:"omitempty,oneof=INCLUSIVE EXCLUSIVE NO_TAX"`
	PaymentDate      *string         `json:"paymentDate"`
	PaymentReference *string         `json:"paymentReference,omitempty"`
	Entries          []*item.RqEntry `json:"entries,omitempty" validate:"omitempty,dive"`
	Sections         []*RqSection    `json:"sections,omitempty" validate:"omitempty,dive"`
	ParentSectionID  *uuid.UUID      `json:"parentSectionId,omitempty"`
}

// RqUpdateSection represents the request payload for updating a section
type RqUpdateSection struct {
	ID               *uuid.UUID            `json:"id,omitempty"`
	InvoiceID        *uuid.UUID            `json:"invoiceId,omitempty"`
	TemplateID       *uuid.UUID            `json:"templateId,omitempty"`
	SectionType      *SectionType          `json:"SectionType,omitempty"`
	DocumentNumber   *string               `json:"documentNumber,omitempty"`
	TaxMethod        *TaxMethod            `json:"taxMethod,omitempty" validate:"omitempty,oneof=INCLUSIVE EXCLUSIVE NO_TAX"`
	PaymentDate      *string               `json:"paymentDate"`
	PaymentReference *string               `json:"paymentReference,omitempty"`
	Entries          []*item.RqUpdateEntry `json:"entries,omitempty" validate:"omitempty,dive"`
	DeleteEntries    []uuid.UUID           `json:"deleteEntries,omitempty"`
	Sections         []*RqUpdateSection    `json:"sections,omitempty" validate:"omitempty,dive"`
	ParentSectionID  *uuid.UUID            `json:"parentSectionId,omitempty"`
}

// ToSection converts request to domain model
func (rq *RqSection) ToSection() *Section {
	entries := make([]*item.Item, 0, len(rq.Entries))
	for _, entry := range rq.Entries {
		entries = append(entries, entry.ToItem())
	}

	sectionID := uuid.New()
	sections := make([]*Section, 0, len(rq.Sections))
	for _, childSec := range rq.Sections {
		childSection := childSec.ToSection()
		childSection.ParentSectionID = &sectionID
		sections = append(sections, childSection)
	}

	return &Section{
		ID:               sectionID,
		InvoiceID:        rq.InvoiceID,
		InvoiceSection:   rq.SectionType,
		DocumentNumber:   rq.DocumentNumber,
		TaxMethod:        rq.TaxMethod,
		PaymentDate:      rq.PaymentDate,
		PaymentReference: rq.PaymentReference,
		Entries:          entries,
		Sections:         sections,
		ParentSectionID:  rq.ParentSectionID,
	}
}

// ToSection converts update request to domain model
func (rq *RqUpdateSection) ToSection() *Section {
	section := &Section{}

	if rq.ID != nil {
		section.ID = *rq.ID
	} else {
		section.ID = uuid.New()
	}

	if rq.InvoiceID != nil {
		section.InvoiceID = rq.InvoiceID
	}

	if rq.SectionType != nil {
		section.InvoiceSection = *rq.SectionType
	}

	if rq.DocumentNumber != nil {
		section.DocumentNumber = *rq.DocumentNumber
	}

	if rq.TaxMethod != nil {
		section.TaxMethod = rq.TaxMethod
	}

	if rq.PaymentDate != nil {
		section.PaymentDate = rq.PaymentDate
	}

	if rq.PaymentReference != nil {
		section.PaymentReference = rq.PaymentReference
	}

	if rq.ParentSectionID != nil {
		section.ParentSectionID = rq.ParentSectionID
	}

	if rq.Entries != nil {
		entries := make([]*item.Item, 0, len(rq.Entries))
		for _, entryUpdate := range rq.Entries {
			if entryUpdate.ID == nil || *entryUpdate.ID == uuid.Nil {
				newEntry := &item.Item{
					ID: uuid.New(),
				}
				entryUpdate.ApplyToItem(newEntry)
				entries = append(entries, newEntry)
			} else {
				entry := &item.Item{
					ID: *entryUpdate.ID,
				}
				entryUpdate.ApplyToItem(entry)
				entries = append(entries, entry)
			}
		}
		section.Entries = entries
	}

	if rq.Sections != nil {
		sections := make([]*Section, 0, len(rq.Sections))
		for _, childSecUpdate := range rq.Sections {
			childSection := childSecUpdate.ToSection()
			childSection.ParentSectionID = &section.ID
			sections = append(sections, childSection)
		}
		section.Sections = sections
	}

	return section
}

// Section represents the domain model for an invoice section
type Section struct {
	ID               uuid.UUID    `db:"id"`
	InvoiceID        *uuid.UUID   `db:"invoice_id"`
	InvoiceSection   SectionType  `db:"invoice_section"`
	DocumentNumber   string       `db:"document_number"`
	TaxMethod        *TaxMethod   `db:"tax_method"`
	PaymentDate      *string      `db:"payment_date"`
	PaymentReference *string      `db:"payment_reference"`
	Entries          []*item.Item `db:"-"`
	Sections         []*Section   `db:"-"`
	ParentSectionID  *uuid.UUID   `db:"parent_section_id"`
	CreatedAt        time.Time    `db:"created_at"`
	UpdatedAt        *time.Time   `db:"updated_at"`
	DeletedAt        *time.Time   `db:"deleted_at"`
}

// ToRsSection converts domain model to response
func (s *Section) ToRsSection() *RsSection {
	rsEntries := make([]*item.RsEntry, 0, len(s.Entries))
	for _, entry := range s.Entries {
		rsEntries = append(rsEntries, entry.ToRsEntry())
	}

	rsSections := make([]*RsSection, 0, len(s.Sections))
	for _, childSec := range s.Sections {
		rsSections = append(rsSections, childSec.ToRsSection())
	}

	return &RsSection{
		ID:               s.ID,
		InvoiceID:        s.InvoiceID,
		SectionType:      s.InvoiceSection,
		DocumentNumber:   s.DocumentNumber,
		TaxMethod:        s.TaxMethod,
		PaymentDate:      s.PaymentDate,
		PaymentReference: s.PaymentReference,
		Entries:          rsEntries,
		Sections:         rsSections,
		ParentSectionID:  s.ParentSectionID,
		CreatedAt:        s.CreatedAt,
		UpdatedAt:        s.UpdatedAt,
	}
}

// RsSection represents the response payload for a section
type RsSection struct {
	ID               uuid.UUID       `json:"id"`
	InvoiceID        *uuid.UUID      `json:"invoiceId"`
	TemplateID       uuid.UUID       `json:"templateId,omitempty"`
	SectionType      SectionType     `json:"sectionType"`
	DocumentNumber   string          `json:"documentNumber"`
	TaxMethod        *TaxMethod      `json:"taxMethod,omitempty"`
	PaymentDate      *string         `json:"paymentDate,omitempty"`
	PaymentReference *string         `json:"paymentReference,omitempty"`
	Entries          []*item.RsEntry `json:"entries"`
	Sections         []*RsSection    `json:"sections,omitempty"`
	ParentSectionID  *uuid.UUID      `json:"parentSectionId,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        *time.Time      `json:"updatedAt"`
}

type DocumentBase struct {
	DocumentNumber   string
	TaxMethod        *TaxMethod
	PaymentDate      *string
	PaymentReference *string
	Entries          []*item.Item
}

type CalculationStatement struct {
	DocumentBase
}

type SfaInvoice struct {
	DocumentBase
	InvoiceMethod util.InvoiceType
}

type RctiInvoice struct {
	DocumentBase
	CommissionRate float64
}

type RemittanceInvoice struct {
	DocumentBase
	InvoiceMethod util.InvoiceType
}
