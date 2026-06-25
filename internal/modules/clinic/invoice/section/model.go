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
	InvoiceID        *uuid.UUID      `json:"invoiceId,omitempty"`
	TemplateID       uuid.UUID       `json:"templateId,omitempty"`
	SectionType      SectionType     `json:"sectionType" validate:"required,oneof=CALCULATION_STATEMENT SFA_INVOICE REMITTANCE_INVOICE"`
	DocumentNumber   string          `json:"documentNumber" validate:"required"`
	TaxMethod        *TaxMethod      `json:"taxMethod,omitempty" validate:"omitempty,oneof=INCLUSIVE EXCLUSIVE NO_TAX"`
	PaymentMethod    *string         `json:"paymentMethod,omitempty"`
	AccountName      *string         `json:"accountName,omitempty"`
	Bsb              *string         `json:"bsb,omitempty"`
	AccountNumber    *string         `json:"accountNumber,omitempty"`
	PaymentDate      *string         `json:"paymentDate"`
	PaymentReference *string         `json:"paymentReference,omitempty"`
	Entries          []*item.RqEntry `json:"entries,omitempty" validate:"omitempty,dive"`
}

// RqUpdateSection represents the request payload for updating a section
type RqUpdateSection struct {
	ID               *uuid.UUID            `json:"id,omitempty"`
	InvoiceID        *uuid.UUID            `json:"invoiceId,omitempty"`
	TemplateID       *uuid.UUID            `json:"templateId,omitempty"`
	SectionType      *SectionType          `json:"SectionType,omitempty" validate:"omitempty,oneof=CALCULATION_STATEMENT SFA_INVOICE REMITTANCE_INVOICE"`
	DocumentNumber   *string               `json:"documentNumber,omitempty"`
	TaxMethod        *TaxMethod            `json:"taxMethod,omitempty" validate:"omitempty,oneof=INCLUSIVE EXCLUSIVE NO_TAX"`
	PaymentMethod    *string               `json:"paymentMethod,omitempty"`
	AccountName      *string               `json:"accountName,omitempty"`
	Bsb              *string               `json:"bsb,omitempty"`
	AccountNumber    *string               `json:"accountNumber,omitempty"`
	PaymentDate      *string               `json:"paymentDate"`
	PaymentReference *string               `json:"paymentReference,omitempty"`
	Entries          []*item.RqUpdateEntry `json:"entries,omitempty" validate:"omitempty,dive"`
	DeleteEntries    []uuid.UUID           `json:"deleteEntries,omitempty"`
}

// ToSection converts request to domain model
func (rq *RqSection) ToSection() *Section {
	entries := make([]*item.Item, 0, len(rq.Entries))
	for _, entry := range rq.Entries {
		entries = append(entries, entry.ToItem())
	}

	return &Section{
		ID:               uuid.New(),
		InvoiceID:        rq.InvoiceID,
		TemplateID:       rq.TemplateID,
		InvoiceSection:   rq.SectionType,
		DocumentNumber:   rq.DocumentNumber,
		TaxMethod:        rq.TaxMethod,
		PaymentMethod:    rq.PaymentMethod,
		AccountName:      rq.AccountName,
		Bsb:              rq.Bsb,
		AccountNumber:    rq.AccountNumber,
		PaymentDate:      rq.PaymentDate,
		PaymentReference: rq.PaymentReference,
		Entries:          entries,
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

	if rq.TemplateID != nil {
		section.TemplateID = *rq.TemplateID
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

	if rq.PaymentMethod != nil {
		section.PaymentMethod = rq.PaymentMethod
	}

	if rq.AccountName != nil {
		section.AccountName = rq.AccountName
	}

	if rq.Bsb != nil {
		section.Bsb = rq.Bsb
	}

	if rq.AccountNumber != nil {
		section.AccountNumber = rq.AccountNumber
	}

	if rq.PaymentDate != nil {
		section.PaymentDate = rq.PaymentDate
	}

	if rq.PaymentReference != nil {
		section.PaymentReference = rq.PaymentReference
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

	return section
}

// Section represents the domain model for an invoice section
type Section struct {
	ID               uuid.UUID    `db:"id"`
	InvoiceID        *uuid.UUID   `db:"invoice_id"`
	TemplateID       uuid.UUID    `db:"template_id"`
	InvoiceSection   SectionType  `db:"invoice_section"`
	DocumentNumber   string       `db:"document_number"`
	TaxMethod        *TaxMethod   `db:"tax_method"`
	PaymentMethod    *string      `db:"payment_method"`
	AccountName      *string      `db:"account_name"`
	Bsb              *string      `db:"bsb"`
	AccountNumber    *string      `db:"account_number"`
	PaymentDate      *string      `db:"payment_date"`
	PaymentReference *string      `db:"payment_reference"`
	Entries          []*item.Item `db:"-"`
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

	return &RsSection{
		ID:               s.ID,
		InvoiceID:        s.InvoiceID,
		TemplateID:       s.TemplateID,
		SectionType:      s.InvoiceSection,
		DocumentNumber:   s.DocumentNumber,
		TaxMethod:        s.TaxMethod,
		PaymentMethod:    s.PaymentMethod,
		AccountName:      s.AccountName,
		Bsb:              s.Bsb,
		AccountNumber:    s.AccountNumber,
		PaymentDate:      s.PaymentDate,
		PaymentReference: s.PaymentReference,
		Entries:          rsEntries,
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
	PaymentMethod    *string         `json:"paymentMethod,omitempty"`
	AccountName      *string         `json:"accountName,omitempty"`
	Bsb              *string         `json:"bsb,omitempty"`
	AccountNumber    *string         `json:"accountNumber,omitempty"`
	PaymentDate      *string         `json:"paymentDate,omitempty"`
	PaymentReference *string         `json:"paymentReference,omitempty"`
	Entries          []*item.RsEntry `json:"entries"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        *time.Time      `json:"updatedAt"`
}
