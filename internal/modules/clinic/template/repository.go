package template

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"

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
	List(ctx context.Context, types []string) (*util.RsList, error)
	GetSetting(ctx context.Context, templateId uuid.UUID) (*Setting, error)
	UpdateSetting(ctx context.Context, st *Setting, templateId uuid.UUID) error
	CreateSetting(ctx context.Context, st *Setting) error
	CreateMapping(ctx context.Context, m *Mapping) error
	UpdateMapping(ctx context.Context, m *Mapping) error
	GetDocumentByID(ctx context.Context, id uuid.UUID) (*file.Document, error)
	GetInvoice(ctx context.Context, clinicId uuid.UUID, invoiceId uuid.UUID) (*InvoiceResponse, error)
	GetInvoiceSectionMeta(ctx context.Context, invoiceId uuid.UUID) ([]InvoiceSectionMeta, error)
	GetSavedClinicMailTemplate(ctx context.Context, clinicID uuid.UUID) (string, string, error)
	SaveClinicMailTemplate(ctx context.Context, clinicID uuid.UUID, subject, body string) error
	GetInvoiceSetting(ctx context.Context, clinicId uuid.UUID, invoiceId uuid.UUID, templateIds []uuid.UUID) (*Setting, error)
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

func (r *Repository) List(ctx context.Context, types []string) (*util.RsList, error) {
	var query string
	var args []interface{}
	var err error

	if len(types) > 0 {
		query, args, err = sqlx.In(`
			SELECT id, name, description, html, css, is_default, is_active, created_at, updated_at, deleted_at 
			FROM tbl_template 
			WHERE deleted_at IS NULL 
			  AND name IN (?) 
			ORDER BY created_at DESC`, types)
		if err != nil {
			return nil, fmt.Errorf("failed to build query: %w", err)
		}
		query = r.db.Rebind(query)
	} else {
		query = `SELECT id, name, description, html, css, is_default, is_active, created_at, updated_at, deleted_at FROM tbl_template WHERE deleted_at IS NULL ORDER BY created_at DESC`
	}

	var items []Template
	if err := r.db.SelectContext(ctx, &items, query, args...); err != nil {
		return nil, fmt.Errorf("failed to scan templates: %w", err)
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

func (r *Repository) GetSetting(ctx context.Context, templateId uuid.UUID) (*Setting, error) {
	const q = `
		SELECT s.*, m.template_id FROM tbl_template_setting s
		INNER JOIN tbl_invoice_template_mapping m ON s.id = m.setting_id
		WHERE m.template_id = $1 
		  AND m.invoice_id IS NULL 
		  AND m.clinic_id IS NULL 
		  AND m.deleted_at IS NULL 
		  AND s.deleted_at IS NULL 
		LIMIT 1`

	var st Setting
	if err := r.db.GetContext(ctx, &st, q, templateId); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &st, nil
}

func (r *Repository) GetInvoiceSetting(ctx context.Context, clinicId, invoiceId uuid.UUID, templateIds []uuid.UUID) (*Setting, error) {
	if len(templateIds) == 0 {
		return nil, fmt.Errorf("at least one template ID is required")
	}

	const maxTemplateIds = 10
	if len(templateIds) > maxTemplateIds {
		return nil, fmt.Errorf("too many template IDs provided, maximum is %d", maxTemplateIds)
	}

	const q = `
		SELECT s.*, m.template_id 
		FROM tbl_template_setting s
		INNER JOIN tbl_invoice_template_mapping m ON s.id = m.setting_id
		WHERE m.template_id = ANY($3)
		  AND m.deleted_at IS NULL
		  AND s.deleted_at IS NULL
		  AND (
		      (m.clinic_id = $1 AND m.invoice_id = $2)
		      OR (m.clinic_id IS NULL AND m.invoice_id IS NULL)
		  )
		ORDER BY 
			CASE 
				WHEN m.clinic_id = $1 AND m.invoice_id = $2 THEN 1
				ELSE 2
			END ASC
		LIMIT 1;`

	var st Setting
	if err := r.db.GetContext(ctx, &st, q, clinicId, invoiceId, pq.Array(templateIds)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &st, nil
}

func (r *Repository) UpdateSetting(ctx context.Context, st *Setting, templateId uuid.UUID) error {
	const q = `
		INSERT INTO tbl_template_setting (
			mapping_id, primary_color, accent_color, body_font_family, header_font_family,
			is_logo, logo_id, letterhead_id, footer_id, terms_text, is_watermark, watermark_text, is_tax, table_style
		) VALUES (
			:mapping_id, :primary_color, :accent_color, :body_font_family, :header_font_family,
			:is_logo, :logo_id, :letterhead_id, :footer_id, :terms_text, :is_watermark, :watermark_text, :is_tax, :table_style
		)
		ON CONFLICT (id) DO UPDATE SET
			primary_color      = EXCLUDED.primary_color,
			accent_color       = EXCLUDED.accent_color,
			body_font_family   = EXCLUDED.body_font_family,
			header_font_family = EXCLUDED.header_font_family,
			is_logo            = EXCLUDED.is_logo,
			logo_id            = EXCLUDED.logo_id,
			letterhead_id      = EXCLUDED.letterhead_id,
			footer_id          = EXCLUDED.footer_id,
			terms_text         = EXCLUDED.terms_text,
			is_watermark       = EXCLUDED.is_watermark,
			watermark_text     = EXCLUDED.watermark_text,
			is_tax             = EXCLUDED.is_tax,
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
			id, mapping_id, primary_color, accent_color, body_font_family, header_font_family,
			is_logo, logo_id, letterhead_id, footer_id, terms_text, is_watermark, watermark_text, is_tax
		) VALUES (
			:id, :mapping_id, :primary_color, :accent_color, :body_font_family, :header_font_family,
			:is_logo, :logo_id, :letterhead_id, :footer_id, :terms_text, :is_watermark, :watermark_text, :is_tax
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

func (r *Repository) CreateMapping(ctx context.Context, m *Mapping) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}

	const q = `
		INSERT INTO tbl_invoice_template_mapping (
			id, invoice_id, template_id, setting_id, clinic_id
		) VALUES (
			:id, :invoice_id, :template_id, :setting_id, :clinic_id
		)
		RETURNING created_at, updated_at`

	rows, err := r.db.NamedQueryContext(ctx, q, m)
	if err != nil {
		return fmt.Errorf("failed to insert invoice template junction mapping: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		if err := rows.Scan(&m.CreatedAt, &m.UpdatedAt); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (r *Repository) UpdateMapping(ctx context.Context, m *Mapping) error {
	const q = `
		UPDATE tbl_invoice_template_mapping
		SET 
			invoice_id = :invoice_id,
			template_id = :template_id,
			setting_id = :setting_id,
			clinic_id = :clinic_id,
			updated_at = NOW()
		WHERE id = :id
		RETURNING updated_at`

	rows, err := r.db.NamedQueryContext(ctx, q, m)
	if err != nil {
		return fmt.Errorf("failed to update invoice template junction mapping: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		if err := rows.Scan(&m.UpdatedAt); err != nil {
			return err
		}
		return nil
	}

	return fmt.Errorf("mapping not found for update: %s", m.ID)
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
			payment_method,
			account_name,
			bsb_number,
			account_number,
			payment_date::text,
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
