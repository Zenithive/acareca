package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/file"
	"github.com/jmoiron/sqlx"
)

// IDocumentRepository handles document and invoice related data access
type IDocumentRepository interface {
	GetDocumentByID(ctx context.Context, id uuid.UUID) (*file.Document, error)
	GetInvoice(ctx context.Context, clinicId uuid.UUID, invoiceId uuid.UUID) (*InvoiceResponse, error)
	GetInvoiceSectionMeta(ctx context.Context, invoiceId uuid.UUID) ([]InvoiceSectionMeta, error)
	GetSavedClinicMailTemplate(ctx context.Context, clinicID uuid.UUID) (string, string, error)
	SaveClinicMailTemplate(ctx context.Context, clinicID uuid.UUID, subject, body string) error
}

// DocumentRepository implements document data access
type DocumentRepository struct {
	db *sqlx.DB
}

// NewDocumentRepository creates a new document repository
func NewDocumentRepository(db *sqlx.DB) IDocumentRepository {
	return &DocumentRepository{db: db}
}

// GetDocumentByID retrieves a document by its ID
func (r *DocumentRepository) GetDocumentByID(ctx context.Context, id uuid.UUID) (*file.Document, error) {
	const q = `SELECT * FROM tbl_document WHERE id = $1 AND deleted_at IS NULL`
	var doc file.Document
	if err := r.db.GetContext(ctx, &doc, q, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}

// invoiceRow is a database model for invoice queries
type invoiceRow struct {
	Id                uuid.UUID `db:"id"`
	ClinicId          uuid.UUID `db:"clinic_id"`
	BillingPeriodFrom string    `db:"billing_period_from"`
	BillingPeriodTo   string    `db:"billing_period_to"`
	InvoiceFrequency  string    `db:"invoice_frequency"`
	IssueDate         string    `db:"issue_date"`
	DueDate           string    `db:"due_date"`
	Status            string    `db:"status"`
	FName             string    `db:"fname"`
	LName             string    `db:"lname"`
	Email             string    `db:"email"`
	Phone             string    `db:"phone"`
	ABN               string    `db:"abn"`
	ClinicName        string    `db:"clinic_name"`
	AddressLine1      string    `db:"address_line1"`
	City              string    `db:"city"`
	State             string    `db:"state"`
	PostalCode        string    `db:"postal_code"`
	Country           string    `db:"country"`
}

// InvoiceContact represents contact information
type InvoiceContact struct {
	FName   string
	LName   string
	Email   string
	Phone   string
	ABN     string
	Address []string
}

// InvoiceItem represents an invoice line item
type InvoiceItem struct {
	ID          uuid.UUID `db:"id"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
	Amount      float64   `db:"amount"`
	BASCode     string    `db:"bas_code"`
	EntryType   string    `db:"entry_type"`
	SectionType string    `db:"section_type"`
	FieldKey    *string   `db:"field_key"`
	Expression  *string   `db:"expression"`
	SortOrder   int       `db:"sort_order"`
	IsFinal     bool      `db:"is_final"`
}

// InvoiceResponse represents the complete invoice data
type InvoiceResponse struct {
	ID                uuid.UUID
	ClinicID          uuid.UUID
	TemplateID        uuid.UUID
	BillingPeriodFrom string
	BillingPeriodTo   string
	InvoiceFrequency  string
	IssueDate         string
	DueDate           string
	Status            string
	ClinicName        string
	InvoiceNumber     string
	SentBy            InvoiceContact
	SentTo            InvoiceContact
	Items             []InvoiceItem
}

// GetInvoice retrieves complete invoice data with related entities
func (r *DocumentRepository) GetInvoice(ctx context.Context, clinicId uuid.UUID, invoiceId uuid.UUID) (*InvoiceResponse, error) {
	const q = `
        SELECT
            i.id, i.clinic_id, 
            i.billing_period_from::text, i.billing_period_to::text,
            i.invoice_frequency, i.issue_date::text, i.due_date::text,
            i.status,
            COALESCE(cp.fname, '') as fname, 
            COALESCE(cp.lname, '') as lname, 
            COALESCE(cp.email, '') as email, 
            COALESCE(cp.phone, '') as phone, 
            COALESCE(cp.abn, '') as abn,
            COALESCE(cl.clinic_name, '') as clinic_name,
            COALESCE(a.address_line1, '') as address_line1,
            COALESCE(a.city, '') as city,
            COALESCE(a.state, '') as state,
            COALESCE(a.postal_code, '') as postal_code,
            COALESCE(a.country, '') as country,
            COALESCE((
                SELECT document_number 
                FROM tbl_map_invoice_section 
                WHERE invoice_id = i.id AND deleted_at IS NULL 
                ORDER BY created_at ASC LIMIT 1
            ), '') as section_document_number
        FROM tbl_invoice i
        LEFT JOIN tbl_invoice_clinic cl ON cl.id = i.clinic_id AND cl.deleted_at IS NULL
        LEFT JOIN tbl_clinic_contact_person cp ON cp.clinic_id = i.clinic_id AND cp.deleted_at IS NULL
        LEFT JOIN tbl_clinic_contact_person_address a ON a.contact_id = cp.id AND a.is_primary = TRUE AND a.deleted_at IS NULL
        WHERE i.id = $2 AND i.clinic_id = $1 AND i.deleted_at IS NULL`

	var row struct {
		invoiceRow
		SectionDocumentNumber string `db:"section_document_number"`
	}

	if err := r.db.GetContext(ctx, &row, q, clinicId, invoiceId); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("invoice not found")
		}
		return nil, err
	}

	// Query items from tbl_invoice_item
	const itemQ = `
        SELECT 
            ii.id, 
            ii.name, 
            COALESCE(ii.description, '') AS description, 
            ii.amount, 
            ii.bas_code, 
            COALESCE(ii.entry_type, '') AS entry_type,
            COALESCE(s.invoice_section::text, '') AS section_type,
            ii.field_key,
			ii.expression,
			ii.sort_order,
            COALESCE(ii.is_final, false) AS is_final
        FROM tbl_invoice_item ii
        INNER JOIN tbl_map_invoice_section s ON ii.invoice_section_id = s.id
        WHERE s.invoice_id = $1 AND ii.deleted_at IS NULL AND s.deleted_at IS NULL
        ORDER BY ii.sort_order ASC, ii.created_at ASC`
	
	var items []InvoiceItem
	if err := r.db.SelectContext(ctx, &items, itemQ, row.Id); err != nil {
		return nil, err
	}

	// Build address
	address := []string{}
	if row.AddressLine1 != "" {
		addr := row.AddressLine1
		if row.City != "" {
			addr += ", " + row.City
		}
		if row.State != "" {
			addr += ", " + row.State
		}
		if row.PostalCode != "" {
			addr += " " + row.PostalCode
		}
		if row.Country != "" {
			addr += ", " + row.Country
		}
		address = append(address, addr)
	}

	// Use section document number as invoice number
	invoiceNumberName := row.SectionDocumentNumber
	if invoiceNumberName == "" {
		invoiceNumberName = row.Id.String()[:8]
	}

	return &InvoiceResponse{
		ID:                row.Id,
		ClinicID:          row.ClinicId,
		TemplateID:        uuid.Nil,
		BillingPeriodFrom: row.BillingPeriodFrom,
		BillingPeriodTo:   row.BillingPeriodTo,
		InvoiceFrequency:  row.InvoiceFrequency,
		IssueDate:         row.IssueDate,
		DueDate:           row.DueDate,
		Status:            row.Status,
		ClinicName:        row.ClinicName,
		InvoiceNumber:     invoiceNumberName,
		SentBy: InvoiceContact{
			FName:   row.FName,
			LName:   row.LName,
			Email:   row.Email,
			Phone:   row.Phone,
			ABN:     row.ABN,
			Address: address,
		},
		SentTo: InvoiceContact{
			FName:   row.FName,
			LName:   row.LName,
			Email:   row.Email,
			Phone:   row.Phone,
			ABN:     row.ABN,
			Address: address,
		},
		Items: items,
	}, nil
}

// InvoiceSectionMeta represents invoice section metadata
type InvoiceSectionMeta struct {
	ID               uuid.UUID `db:"id"`
	SectionType      string    `db:"section_type"`
	DocumentNumber   string    `db:"document_number"`
	PaymentMethod    *string   `db:"payment_method"`
	AccountName      *string   `db:"account_name"`
	BSBNumber        *string   `db:"bsb_number"`
	AccountNumber    *string   `db:"account_number"`
	PaymentDate      *string   `db:"payment_date"`
	PaymentReference *string   `db:"payment_reference"`
}

// GetInvoiceSectionMeta retrieves invoice section metadata
func (r *DocumentRepository) GetInvoiceSectionMeta(ctx context.Context, invoiceId uuid.UUID) ([]InvoiceSectionMeta, error) {
	const q = `
		SELECT 
			id,
			COALESCE(invoice_section::text, '') AS section_type,
			COALESCE(document_number, '') AS document_number,
			payment_method,
			account_name,
			bsb_number,
			account_number,
			payment_date::text AS payment_date,
			payment_reference
		FROM tbl_map_invoice_section
		WHERE invoice_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC`

	var sections []InvoiceSectionMeta
	if err := r.db.SelectContext(ctx, &sections, q, invoiceId); err != nil {
		return nil, fmt.Errorf("failed to fetch invoice section metadata: %w", err)
	}
	return sections, nil
}

// GetSavedClinicMailTemplate retrieves clinic mail template
func (r *DocumentRepository) GetSavedClinicMailTemplate(ctx context.Context, clinicID uuid.UUID) (string, string, error) {
	var subject, body string

	err := r.db.QueryRowContext(ctx, `
		SELECT mail_subject, mail_body 
		FROM tbl_clinic_invoice_mail_templates 
		WHERE clinic_id = $1
	`, clinicID).Scan(&subject, &body)

	if err != nil {
		return "", "", err
	}

	return subject, body, nil
}

// SaveClinicMailTemplate saves or updates clinic mail template
func (r *DocumentRepository) SaveClinicMailTemplate(ctx context.Context, clinicID uuid.UUID, subject, body string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO tbl_clinic_invoice_mail_templates (clinic_id, mail_subject, mail_body)
		VALUES ($1, $2, $3)
		ON CONFLICT (clinic_id) 
		DO UPDATE SET 
			mail_subject = EXCLUDED.mail_subject,
			mail_body = EXCLUDED.mail_body,
			updated_at = NOW()
	`, clinicID, subject, strings.TrimSpace(body))

	return err
}
