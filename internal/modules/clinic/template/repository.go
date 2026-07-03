package template

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/file"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var ErrNotFound = errors.New("template not found")
var ErrInvoiceNotFound = errors.New("invoice record not found")
var ErrUnauthorized = errors.New("unauthorized access to template or invoice")

type IRepository interface {
	Create(ctx context.Context, t *Template) error
	BulkCreate(ctx context.Context, t []Template) error
	Update(ctx context.Context, t *Template) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*Template, error)
	List(ctx context.Context, method string) (*util.RsList, error)
	UpdateSetting(ctx context.Context, st *Setting, invoiceId uuid.UUID) error
	CreateSetting(ctx context.Context, st *Setting) error
	GetDocumentByID(ctx context.Context, id uuid.UUID) (*file.Document, error)
	GetInvoice(ctx context.Context, clinicId uuid.UUID, invoiceId uuid.UUID) (*InvoiceResponse, error)
	GetInvoiceSectionMeta(ctx context.Context, invoiceId uuid.UUID) ([]InvoiceSectionMeta, error)
	GetSavedClinicMailTemplate(ctx context.Context, clinicID uuid.UUID) (string, string, error)
	SaveClinicMailTemplate(ctx context.Context, clinicID uuid.UUID, subject, body string) error
	GetInvoiceSetting(ctx context.Context, invoiceId uuid.UUID) (*Setting, error)
	ValidateTemplateAccess(ctx context.Context, templateIds []uuid.UUID) error
}

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) IRepository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, t *Template) error {
	const q = `
		INSERT INTO tbl_template (name, description, html, css, is_default, is_active)
		VALUES (:name, :description, :html, :css, :is_default, :is_active)
		RETURNING id, created_at`
	rows, err := r.db.NamedQueryContext(ctx, q, t)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return rows.StructScan(t)
	}
	return nil
}

func (r *Repository) Update(ctx context.Context, t *Template) error {
	const q = `
		UPDATE tbl_template
		SET name = :name, html = :html, css = :css, is_default = :is_default, is_active = :is_active, updated_at = NOW()
		WHERE id = :id AND deleted_at IS NULL`
	_, err := r.db.NamedExecContext(ctx, q, t)
	return err
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE tbl_template SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.db.ExecContext(ctx, q, id)
	return err
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*Template, error) {
	const q = `SELECT id, name, description, html, css, is_default, is_active, created_at, updated_at, deleted_at FROM tbl_template WHERE id = $1 AND deleted_at IS NULL`
	var t Template
	if err := r.db.GetContext(ctx, &t, q, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get template: %w", err)
	}
	return &t, nil
}

func (r *Repository) List(ctx context.Context, method string) (*util.RsList, error) {
	var query string
	var args []interface{}
	var err error

	// Determine template names in their strict presentation order
	var templateNames []string
	switch method {
	case "SFA_CLINIC_COLLECTS":
		// Method A - Page 1: Calculation Statement, Page 2: Tax Invoice, Page 3: Remittance Advice
		templateNames = []string{"Calculation Statement", "Tax Invoice", "Remittance Advice"}
	case "SFA_DENTIST_COLLECTS":
		// Method B - Page 1: Calculation Statement, Page 2: Tax Invoice
		templateNames = []string{"Calculation Statement", "Tax Invoice"}
	case "INDEPENDENT_CONTRACTOR":
		// Method C - Page 1: Calculation Statement, Page 2: RCTI, Page 3: Remittance Advice
		templateNames = []string{"Calculation Statement", "Recipient Created Tax Invoice", "Remittance Advice"}
	}

	if len(templateNames) > 0 {
		query, args, err = sqlx.In(`
			SELECT id, name, description, html, css, is_default, is_active, created_at, updated_at, deleted_at 
			FROM tbl_template 
			WHERE deleted_at IS NULL 
			  AND name IN (?)`, templateNames)
		if err != nil {
			return nil, fmt.Errorf("failed to build query: %w", err)
		}
		query = r.db.Rebind(query)
	} else {
		// Fallback: If no method filter, return everything grouped alphabetically
		query = `
			SELECT id, name, description, html, css, is_default, is_active, created_at, updated_at, deleted_at 
			FROM tbl_template 
			WHERE deleted_at IS NULL 
			ORDER BY name ASC, created_at DESC`
	}

	var items []Template
	if err := r.db.SelectContext(ctx, &items, query, args...); err != nil {
		return nil, fmt.Errorf("failed to scan templates: %w", err)
	}

	// If a specific method was chosen, sort the database items to exactly match the templateNames slice order
	if len(templateNames) > 0 {
		orderMap := make(map[string]int)
		for i, name := range templateNames {
			orderMap[name] = i
		}

		// Sort the retrieved slice in-place based on our ordered map index positioning
		sort.Slice(items, func(i, j int) bool {
			return orderMap[items[i].Name] < orderMap[items[j].Name]
		})
	}

	rs := make([]RsTemplate, len(items))
	for i, t := range items {
		rsView := t.ToRs()
		rsView.Html = base64.StdEncoding.EncodeToString(t.Html)
		rsView.Css = base64.StdEncoding.EncodeToString(t.Css)
		rs[i] = rsView
	}
	return &util.RsList{Items: rs, Total: len(rs)}, nil
}

// GetInvoiceSetting looks up an invoice-specific row; if empty or missing, it falls back to the system default row (where invoice_id IS NULL)
func (r *Repository) GetInvoiceSetting(ctx context.Context, invoiceId uuid.UUID) (*Setting, error) {
	var st Setting

	// Case 1: An invoice ID was provided. Look for explicit custom settings first.
	if invoiceId != uuid.Nil {
		const qSpecific = `
			SELECT 
				id, invoice_id, primary_color, accent_color, body_font_family, header_font_family, is_logo, logo_id, 
				terms_text, payment_terms, is_watermark, watermark_text, table_style, created_at, updated_at, deleted_at
			FROM tbl_template_setting 
			WHERE invoice_id = $1 
			  AND deleted_at IS NULL 
			LIMIT 1;`

		err := r.db.GetContext(ctx, &st, qSpecific, invoiceId)
		if err == nil {
			return &st, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("failed fetching specific invoice setting for id %s: %w", invoiceId, err)
		}
	}

	// Case 2: No invoice ID provided OR no specific row was found. Fallback to global defaults.
	const qDefault = `
		SELECT 
			id, invoice_id, primary_color, accent_color, body_font_family, header_font_family, is_logo, logo_id, 
			terms_text, payment_terms, is_watermark, watermark_text, table_style, created_at, updated_at, deleted_at
		FROM tbl_template_setting 
		WHERE invoice_id IS NULL 
		  AND deleted_at IS NULL 
		LIMIT 1;`

	if err := r.db.GetContext(ctx, &st, qDefault); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed fetching fallback global default invoice settings: %w", err)
	}

	return &st, nil
}

func (r *Repository) UpdateSetting(ctx context.Context, st *Setting, invoiceId uuid.UUID) error {
	const q = `
		INSERT INTO tbl_template_setting (
			id, invoice_id, primary_color, accent_color, body_font_family, header_font_family,
			is_logo, logo_id, terms_text, payment_terms, is_watermark, watermark_text, table_style
		) VALUES (
			:id, :invoice_id, :primary_color, :accent_color, :body_font_family, :header_font_family,
			:is_logo, :logo_id, :terms_text, :payment_terms, :is_watermark, :watermark_text, :table_style
		)
		ON CONFLICT (id) DO UPDATE SET
			invoice_id         = EXCLUDED.invoice_id,
			primary_color      = EXCLUDED.primary_color,
			accent_color       = EXCLUDED.accent_color,
			body_font_family   = EXCLUDED.body_font_family,
			header_font_family = EXCLUDED.header_font_family,
			is_logo            = EXCLUDED.is_logo,
			logo_id            = EXCLUDED.logo_id,
			terms_text         = EXCLUDED.terms_text,
			payment_terms      = EXCLUDED.payment_terms,
			is_watermark       = EXCLUDED.is_watermark,
			watermark_text     = EXCLUDED.watermark_text,
			table_style        = EXCLUDED.table_style,
			updated_at         = NOW()
		RETURNING id, created_at, updated_at`

	rows, err := r.db.NamedQueryContext(ctx, q, st)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return rows.StructScan(st)
	}
	return nil
}

func (r *Repository) BulkCreate(ctx context.Context, templates []Template) error {
	const q = `
		INSERT INTO tbl_template (name, description, html, css, is_default, is_active)
		VALUES (:name, :description, :html, :css, :is_default, :is_active)
		RETURNING id, created_at`

	rows, err := r.db.NamedQueryContext(ctx, q, templates)
	if err != nil {
		return err
	}
	defer rows.Close()

	i := 0
	for rows.Next() {
		if err := rows.StructScan(&templates[i]); err != nil {
			return err
		}
		i++
	}
	return rows.Err()
}

func (r *Repository) CreateSetting(ctx context.Context, st *Setting) error {
	const q = `
		INSERT INTO tbl_template_setting (
			id, invoice_id, primary_color, accent_color, body_font_family, header_font_family,
			is_logo, logo_id, terms_text, payment_terms, is_watermark, watermark_text, table_style
		) VALUES (
			:id, :invoice_id, :primary_color, :accent_color, :body_font_family, :header_font_family,
			:is_logo, :logo_id, :terms_text,:payment_terms, :is_watermark, :watermark_text, :table_style
		)
		RETURNING created_at`

	rows, err := r.db.NamedQueryContext(ctx, q, st)
	if err != nil {
		return err
	}
	defer rows.Close()

	if rows.Next() {
		return rows.StructScan(st)
	}
	return rows.Err()
}

func (r *Repository) GetDocumentByID(ctx context.Context, id uuid.UUID) (*file.Document, error) {
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

func (r *Repository) GetInvoice(ctx context.Context, clinicId uuid.UUID, invoiceId uuid.UUID) (*InvoiceResponse, error) {
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
			return nil, ErrInvoiceNotFound
		}
		return nil, err
	}

	// Fixed: Query items from tbl_invoice_item table joined with sections
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

	// Use section document number as fallback system invoice track name
	invoiceNumberName := row.SectionDocumentNumber
	if invoiceNumberName == "" {
		invoiceNumberName = row.Id.String()[:8]
	}

	return &InvoiceResponse{
		ID:                row.Id,
		ClinicID:          row.ClinicId,
		TemplateID:        uuid.Nil, // Templates now stored in tbl_map_invoice_section
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

func (r *Repository) GetInvoiceSectionMeta(ctx context.Context, invoiceId uuid.UUID) ([]InvoiceSectionMeta, error) {
	const q = `
		SELECT 
			id,
			COALESCE(invoice_section::text, '') AS section_type,
			COALESCE(document_number, '') AS document_number,
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

func (r *Repository) GetSavedClinicMailTemplate(ctx context.Context, clinicID uuid.UUID) (string, string, error) {
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

func (r *Repository) SaveClinicMailTemplate(ctx context.Context, clinicID uuid.UUID, subject, body string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO tbl_clinic_invoice_mail_templates (clinic_id, mail_subject, mail_body)
		VALUES ($1, $2, $3)
		ON CONFLICT (clinic_id) 
		DO UPDATE SET 
			mail_subject = EXCLUDED.mail_subject,
			mail_body = EXCLUDED.mail_body,
			updated_at = NOW()
	`, clinicID, subject, body)

	return err
}

func (r *Repository) ValidateTemplateAccess(ctx context.Context, templateIds []uuid.UUID) error {
	if len(templateIds) == 0 {
		return nil
	}

	const maxTemplateIds = 10
	if len(templateIds) > maxTemplateIds {
		return fmt.Errorf("too many template IDs provided, maximum is %d", maxTemplateIds)
	}

	// Check that all templates exist and are active
	const q = `
		SELECT COUNT(*) 
		FROM tbl_template 
		WHERE id = ANY($1) 
		  AND deleted_at IS NULL 
		  AND is_active = TRUE`

	var count int
	if err := r.db.GetContext(ctx, &count, q, pq.Array(templateIds)); err != nil {
		return fmt.Errorf("failed to validate template access: %w", err)
	}

	if count != len(templateIds) {
		return ErrUnauthorized
	}

	return nil
}
