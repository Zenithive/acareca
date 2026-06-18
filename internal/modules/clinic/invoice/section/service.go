package section

import (
	"context"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/item"
)

type ISection interface {
	Build(ctx context.Context, invoiceId *uuid.UUID) (Section, error)
}

type CalculationStatement struct {
	DocumentNumber   string
	TaxMethod        *TaxMethod
	PaymentMethod    *string
	AccountName      *string
	Bsb              *string
	AccountNumber    *string
	PaymentDate      *string
	PaymentReference *string
	Entries          []*item.Item
}

type SfaInvoice struct {
	DocumentNumber   string
	TaxMethod        *TaxMethod
	PaymentMethod    *string
	AccountName      *string
	Bsb              *string
	AccountNumber    *string
	PaymentDate      *string
	PaymentReference *string
	Entries          []*item.Item
}

type RemittanceInvoice struct {
	DocumentNumber   string
	TaxMethod        *TaxMethod
	PaymentMethod    *string
	AccountName      *string
	Bsb              *string
	AccountNumber    *string
	PaymentDate      *string
	PaymentReference *string
	Entries          []*item.Item
}

func (ct CalculationStatement) Build(ctx context.Context, invoiceId *uuid.UUID, calculatedDocNum string) (Section, error) {
	sectionID := uuid.New()

	// Set default document number if not provided
	docNumber := ct.DocumentNumber
	if docNumber == "" {
		if calculatedDocNum != "" {
			docNumber = calculatedDocNum
		} else {
			// Fallback if database indexer isn't ready
			docNumber = "CS-" + uuid.New().String()[:8]
		}
	}

	// Link entries to this section
	entries := ct.Entries
	if entries == nil {
		entries = []*item.Item{}
	}
	for _, entry := range entries {
		entry.InvoiceSectionID = &sectionID
	}

	return Section{
		ID:               sectionID,
		InvoiceID:        invoiceId,
		InvoiceSection:   CALCULATIONSTATEMENT,
		DocumentNumber:   docNumber,
		TaxMethod:        ct.TaxMethod,
		PaymentMethod:    ct.PaymentMethod,
		AccountName:      ct.AccountName,
		Bsb:              ct.Bsb,
		AccountNumber:    ct.AccountNumber,
		PaymentDate:      ct.PaymentDate,
		PaymentReference: ct.PaymentReference,
		Entries:          entries,
	}, nil
}

func (ct SfaInvoice) Build(ctx context.Context, invoiceId *uuid.UUID, calculatedDocNum string) (Section, error) {
	sectionID := uuid.New()

	// Set default document number if not provided
	docNumber := ct.DocumentNumber
	if docNumber == "" {
		if calculatedDocNum != "" {
			docNumber = calculatedDocNum
		} else {
			docNumber = "SFA-" + uuid.New().String()[:8]
		}
	}

	// Link entries to this section
	entries := ct.Entries
	if entries == nil {
		entries = []*item.Item{}
	}
	for _, entry := range entries {
		entry.InvoiceSectionID = &sectionID
	}

	return Section{
		ID:               sectionID,
		InvoiceID:        invoiceId,
		InvoiceSection:   SFAINVOICE,
		DocumentNumber:   docNumber,
		TaxMethod:        ct.TaxMethod,
		PaymentMethod:    ct.PaymentMethod,
		AccountName:      ct.AccountName,
		Bsb:              ct.Bsb,
		AccountNumber:    ct.AccountNumber,
		PaymentDate:      ct.PaymentDate,
		PaymentReference: ct.PaymentReference,
		Entries:          entries,
	}, nil
}

func (ct RemittanceInvoice) Build(ctx context.Context, invoiceId *uuid.UUID, calculatedDocNum string) (Section, error) {
	sectionID := uuid.New()

	// Set default document number if not provided
	docNumber := ct.DocumentNumber
	if docNumber == "" {
		if calculatedDocNum != "" {
			docNumber = calculatedDocNum
		} else {
			docNumber = "REM-" + uuid.New().String()[:8]
		}
	}

	// Link entries to this section
	entries := ct.Entries
	if entries == nil {
		entries = []*item.Item{}
	}
	for _, entry := range entries {
		entry.InvoiceSectionID = &sectionID
	}

	return Section{
		ID:               sectionID,
		InvoiceID:        invoiceId,
		InvoiceSection:   REMITTANCEINVOICE,
		DocumentNumber:   docNumber,
		TaxMethod:        ct.TaxMethod,
		PaymentMethod:    ct.PaymentMethod,
		AccountName:      ct.AccountName,
		Bsb:              ct.Bsb,
		AccountNumber:    ct.AccountNumber,
		PaymentDate:      ct.PaymentDate,
		PaymentReference: ct.PaymentReference,
		Entries:          entries,
	}, nil
}
