package section

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/item"
)

type ISection interface {
	Build(ctx context.Context, invoiceId *uuid.UUID) (Section, error)
}

func (ct *CalculationStatement) Build(ctx context.Context, invoiceId *uuid.UUID) (Section, error) {
	sectionID := uuid.New()

	docNumber := ct.DocumentNumber
	if docNumber == "" {
		docNumber = "CS-" + strconv.Itoa(time.Now().Year()) + "-" + uuid.New().String()[:8]
	}

	entries := ct.Entries
	if entries == nil {
		entries = []*item.Item{}
	}
	for _, entry := range entries {
		entry.InvoiceSectionID = &sectionID
	}

	sectionType := CALCULATIONSTATEMENT
	return Section{
		ID:               sectionID,
		InvoiceID:        invoiceId,
		InvoiceSection:   &sectionType,
		DocumentNumber:   docNumber,
		TaxMethod:        ct.TaxMethod,
		PaymentDate:      ct.PaymentDate,
		PaymentReference: ct.PaymentReference,
		Entries:          entries,
	}, nil
}

func (ct *SfaInvoice) Build(ctx context.Context, invoiceId *uuid.UUID) (Section, error) {
	sectionID := uuid.New()

	docNumber := ct.DocumentNumber
	if docNumber == "" {
		docNumber = "SFA-" + strconv.Itoa(time.Now().Year()) + "-" + uuid.New().String()[:8]
	}

	entries := ct.Entries
	if entries == nil {
		entries = []*item.Item{}
	}
	for _, entry := range entries {
		entry.InvoiceSectionID = &sectionID
	}

	sectionType := TAXINVOICE
	return Section{
		ID:               sectionID,
		InvoiceID:        invoiceId,
		InvoiceSection:   &sectionType,
		DocumentNumber:   docNumber,
		TaxMethod:        ct.TaxMethod,
		PaymentDate:      ct.PaymentDate,
		PaymentReference: ct.PaymentReference,
		Entries:          entries,
	}, nil
}

func (ct *RemittanceInvoice) Build(ctx context.Context, invoiceId *uuid.UUID) (Section, error) {
	sectionID := uuid.New()

	docNumber := ct.DocumentNumber
	if docNumber == "" {
		docNumber = "REM-" + strconv.Itoa(time.Now().Year()) + "-" + uuid.New().String()[:8]
	}

	entries := ct.Entries
	if entries == nil {
		entries = []*item.Item{}
	}
	for _, entry := range entries {
		entry.InvoiceSectionID = &sectionID
	}

	sectionType := REMITTANCEINVOICE
	return Section{
		ID:               sectionID,
		InvoiceID:        invoiceId,
		InvoiceSection:   &sectionType,
		DocumentNumber:   docNumber,
		TaxMethod:        ct.TaxMethod,
		PaymentDate:      ct.PaymentDate,
		PaymentReference: ct.PaymentReference,
		Entries:          entries,
	}, nil
}

func (ct *RctiInvoice) Build(ctx context.Context, invoiceId *uuid.UUID) (Section, error) {
	sectionID := uuid.New()

	docNumber := ct.DocumentNumber
	if docNumber == "" {
		docNumber = "RCTI-" + strconv.Itoa(time.Now().Year()) + "-" + uuid.New().String()[:8]
	}

	entries := ct.Entries
	if entries == nil {
		entries = []*item.Item{}
	}
	for _, entry := range entries {
		entry.InvoiceSectionID = &sectionID
	}

	sectionType := RCTI
	return Section{
		ID:               sectionID,
		InvoiceID:        invoiceId,
		InvoiceSection:   &sectionType,
		DocumentNumber:   docNumber,
		TaxMethod:        ct.TaxMethod,
		PaymentDate:      ct.PaymentDate,
		PaymentReference: ct.PaymentReference,
		Entries:          entries,
	}, nil
}
