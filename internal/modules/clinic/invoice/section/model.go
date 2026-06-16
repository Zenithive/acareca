package section

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/item"
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
	SFAINVOICE           SectionType = "SFA_INVOICE"
	REMITTANCEINVOICE    SectionType = "REMITTANCE_INVOICE"
)

// RqSection represents the request payload for creating a section
type RqSection struct {
	InvoiceID      *uuid.UUID      `json:"invoiceId,omitempty"`
	SectionType    SectionType     `json:"sectionType" validate:"required,oneof=CALCULATION_STATEMENT SFA_INVOICE REMITTANCE_INVOICE"`
	DocumentNumber string          `json:"documentNumber" validate:"required"`
	TaxMethod      *TaxMethod      `json:"taxMethod,omitempty" validate:"omitempty,oneof=INCLUSIVE EXCLUSIVE NO_TAX"`
	Entries        []*item.RqEntry `json:"entries,omitempty" validate:"omitempty,dive"`
}

// RqUpdateSection represents the request payload for updating a section
type RqUpdateSection struct {
	ID             *uuid.UUID            `json:"id,omitempty"`
	InvoiceID      *uuid.UUID            `json:"invoiceId,omitempty"`
	InvoiceSection *SectionType          `json:"invoiceSection,omitempty" validate:"omitempty,oneof=CALCULATION_STATEMENT SFA_INVOICE REMITTANCE_INVOICE"`
	DocumentNumber *string               `json:"documentNumber,omitempty"`
	TaxMethod      *TaxMethod            `json:"taxMethod,omitempty" validate:"omitempty,oneof=INCLUSIVE EXCLUSIVE NO_TAX"`
	Entries        []*item.RqUpdateEntry `json:"entries,omitempty" validate:"omitempty,dive"`
}

// ToSection converts request to domain model
func (rq *RqSection) ToSection() *Section {
	entries := make([]*item.Item, 0, len(rq.Entries))
	for _, entry := range rq.Entries {
		entries = append(entries, entry.ToItem())
	}

	return &Section{
		ID:             uuid.New(),
		InvoiceID:      rq.InvoiceID,
		InvoiceSection: rq.SectionType,
		DocumentNumber: rq.DocumentNumber,
		TaxMethod:      rq.TaxMethod,
		Entries:        entries,
	}
}

// ToSection converts update request to domain model
func (rq *RqUpdateSection) ToSection() *Section {
	section := &Section{}

	// Parse and set ID if provided
	if rq.ID != nil {
		section.ID = *rq.ID
	}

	// Parse and set InvoiceID if provided
	if rq.InvoiceID != nil {
		section.InvoiceID = rq.InvoiceID
	}

	// Set section type if provided
	if rq.InvoiceSection != nil {
		section.InvoiceSection = *rq.InvoiceSection
	}

	// Set document number if provided
	if rq.DocumentNumber != nil {
		section.DocumentNumber = *rq.DocumentNumber
	}

	// Set tax method if provided
	if rq.TaxMethod != nil {
		section.TaxMethod = rq.TaxMethod
	}

	// Convert entries if provided
	if rq.Entries != nil {
		entries := make([]*item.Item, 0, len(rq.Entries))
		for _, entryUpdate := range rq.Entries {
			entry := &item.Item{
				ID: entryUpdate.ID,
			}
			entryUpdate.ApplyToItem(entry)
			entries = append(entries, entry)
		}
		section.Entries = entries
	}

	return section
}

// Section represents the domain model for an invoice section
type Section struct {
	ID             uuid.UUID    `db:"id"`
	InvoiceID      *uuid.UUID   `db:"invoice_id"`
	InvoiceSection SectionType  `db:"invoice_section"`
	DocumentNumber string       `db:"document_number"`
	TaxMethod      *TaxMethod   `db:"tax_method"`
	Entries        []*item.Item `db:"-"`
	CreatedAt      time.Time    `db:"created_at"`
	UpdatedAt      *time.Time   `db:"updated_at"`
	DeletedAt      *time.Time   `db:"deleted_at"`
}

// ToRsSection converts domain model to response
func (s *Section) ToRsSection() *RsSection {
	rsEntries := make([]*item.RsEntry, 0, len(s.Entries))
	for _, entry := range s.Entries {
		rsEntries = append(rsEntries, entry.ToRsEntry())
	}

	return &RsSection{
		ID:             s.ID,
		InvoiceID:      s.InvoiceID,
		SectionType:    s.InvoiceSection,
		DocumentNumber: s.DocumentNumber,
		TaxMethod:      s.TaxMethod,
		Entries:        rsEntries,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}
}

// RsSection represents the response payload for a section
type RsSection struct {
	ID             uuid.UUID       `json:"id"`
	InvoiceID      *uuid.UUID      `json:"invoiceId"`
	SectionType    SectionType     `json:"sectionType"`
	DocumentNumber string          `json:"documentNumber"`
	TaxMethod      *TaxMethod      `json:"taxMethod,omitempty"`
	Entries        []*item.RsEntry `json:"entries"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      *time.Time      `json:"updatedAt"`
}
