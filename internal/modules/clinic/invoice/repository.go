package invoice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	contactpkg "github.com/iamarpitzala/acareca/internal/modules/clinic/contact"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/invoice/section"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/item"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

var (
	ErrNotFound         = errors.New("invoice not found")
	ErrInvalidContactID = errors.New("contact_id is required")
	ErrContactNotFound  = errors.New("contact not found")
)

type IRepository interface {
	Create(ctx context.Context, invoice *Invoice) error
	Update(ctx context.Context, invoice *Invoice) error
	UpdateWithSections(ctx context.Context, invoice *Invoice, sections []section.Section, deleteSectionIDs []uuid.UUID, deleteItemIDs map[uuid.UUID][]uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, clinicID uuid.UUID, filter common.Filter) ([]*Invoice, int64, error)
	GetByID(ctx context.Context, db sqlx.QueryerContext, id uuid.UUID) (*Invoice, error)
	GetSavedClinicMailTemplate(ctx context.Context, clinicID uuid.UUID) (string, string, error)
	SaveClinicMailTemplate(ctx context.Context, clinicID uuid.UUID, subject, body string) error
}

type Repository struct {
	db          *sqlx.DB
	itemRepo    item.IRepository
	contactRepo contactpkg.Repository
	sectionRepo section.IRepository
}

func NewRepository(db *sqlx.DB) IRepository {
	itemRepo := item.NewRepository(db)
	return &Repository{
		db:          db,
		itemRepo:    itemRepo,
		contactRepo: contactpkg.NewRepository(db),
		sectionRepo: section.NewRepository(db, itemRepo),
	}
}

func (r *Repository) Create(ctx context.Context, invoice *Invoice) error {
	return util.RunInTransaction(ctx, r.db, func(ctx context.Context, tx *sqlx.Tx) error {
		if invoice.ID == uuid.Nil {
			invoice.ID = uuid.New()
		}

		if err := r.validateContact(ctx, invoice.ClinicID, invoice.ContactID); err != nil {
			return err
		}

		query := `
		INSERT INTO tbl_invoice (
			id, clinic_id, contact_id, template_id, name,
			billing_period_from, billing_period_to, invoice_frequency,
			issue_date, due_date, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

		_, err := tx.ExecContext(ctx, query,
			invoice.ID,
			invoice.ClinicID,
			invoice.ContactID,
			invoice.TemplateID,
			invoice.Name,
			invoice.BillingPeriodFrom,
			invoice.BillingPeriodTo,
			invoice.InvoiceFrequency,
			invoice.IssueDate,
			invoice.DueDate,
			invoice.Status,
		)
		if err != nil {
			return err
		}

		// If the create request payload explicitly contains settings customization overrides, apply them
		if err := r.upsertInvoiceSettingsOverride(ctx, tx, invoice.ID, invoice.TemplateID, invoice.ClinicID, invoice.Settings); err != nil {
			return err
		}

		// Create sections using section repository
		if err = r.sectionRepo.Create(ctx, tx, invoice.ID, invoice.Sections); err != nil {
			return fmt.Errorf("failed to create sections: %w", err)
		}

		return nil
	})
}

func (r *Repository) Update(ctx context.Context, invoice *Invoice) error {
	return util.RunInTransaction(ctx, r.db, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := r.validateContact(ctx, invoice.ClinicID, invoice.ContactID); err != nil {
			return err
		}

		query := `
		UPDATE tbl_invoice
		SET contact_id = $1, template_id = $2, name = $3,
			billing_period_from = $4, billing_period_to = $5,
			invoice_frequency = $6, issue_date = $7, due_date = $8,
			status = $9, updated_at = NOW()
		WHERE id = $10 AND deleted_at IS NULL
	`

		result, err := tx.ExecContext(ctx, query,
			invoice.ContactID,
			invoice.TemplateID,
			invoice.Name,
			invoice.BillingPeriodFrom,
			invoice.BillingPeriodTo,
			invoice.InvoiceFrequency,
			invoice.IssueDate,
			invoice.DueDate,
			invoice.Status,
			invoice.ID,
		)
		if err != nil {
			return err
		}

		if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
			return ErrNotFound
		}

		// Evaluate and upsert integrated configurations
		if err := r.upsertInvoiceSettingsOverride(ctx, tx, invoice.ID, invoice.TemplateID, invoice.ClinicID, invoice.Settings); err != nil {
			return err
		}

		// Get existing sections
		existingSections, err := r.sectionRepo.ListByInvoiceID(ctx, invoice.ID)
		if err != nil {
			return fmt.Errorf("failed to get existing sections: %w", err)
		}

		if len(existingSections) > 0 {
			// Update existing section
			if err := r.sectionRepo.Update(ctx, tx, invoice.Sections); err != nil {
				return fmt.Errorf("failed to update section: %w", err)
			}
		} else {
			// Create new section
			if err := r.sectionRepo.Create(ctx, tx, invoice.ID, invoice.Sections); err != nil {
				return fmt.Errorf("failed to create section: %w", err)
			}
		}

		return nil
	})
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	return util.RunInTransaction(ctx, r.db, func(ctx context.Context, tx *sqlx.Tx) error {
		// Soft delete invoice
		result, err := tx.ExecContext(ctx, `
			UPDATE tbl_invoice
			SET deleted_at = NOW(), updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`, id)
		if err != nil {
			return err
		}

		if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
			return ErrNotFound
		}

		// Get all sections for this invoice
		sections, err := r.sectionRepo.ListByInvoiceID(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to get sections for deletion: %w", err)
		}

		// Delete each section (cascades to items)
		for _, sec := range sections {
			if err := r.sectionRepo.Delete(ctx, tx, sec.ID); err != nil {
				return fmt.Errorf("failed to delete section: %w", err)
			}
		}

		return nil
	})
}

func (r *Repository) List(ctx context.Context, clinicID uuid.UUID, filter common.Filter) ([]*Invoice, int64, error) {
	// Count total records
	total, err := r.countInvoices(ctx, clinicID, filter)
	if err != nil {
		return nil, 0, err
	}

	// Fetch invoices
	allowedColumns := r.getAllowedFilterColumns()
	searchCols := []string{"name"}

	selectQuery := `
		SELECT 
			id, clinic_id, contact_id::text, template_id, name,
			billing_period_from::text, billing_period_to::text,
			invoice_frequency, status, issue_date::text, due_date::text,
			created_at::text, updated_at::text
		FROM tbl_invoice 
		WHERE deleted_at IS NULL AND clinic_id = ?
	`

	query, args := common.BuildQuery(selectQuery, filter, allowedColumns, searchCols, false)
	args = append([]interface{}{clinicID}, args...)

	rows, err := r.db.QueryContext(ctx, sqlx.Rebind(sqlx.DOLLAR, query), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("select invoices failed: %w", err)
	}
	defer rows.Close()

	invoices := make([]*Invoice, 0)
	for rows.Next() {
		invoice := &Invoice{}
		if err := rows.Scan(
			&invoice.ID,
			&invoice.ClinicID,
			&invoice.ContactID,
			&invoice.TemplateID,
			&invoice.Name,
			&invoice.BillingPeriodFrom,
			&invoice.BillingPeriodTo,
			&invoice.InvoiceFrequency,
			&invoice.Status,
			&invoice.IssueDate,
			&invoice.DueDate,
			&invoice.CreatedAt,
			&invoice.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan invoice failed: %w", err)
		}
		invoices = append(invoices, invoice)
	}

	for _, invoice := range invoices {
		sections, err := r.sectionRepo.ListByInvoiceID(ctx, invoice.ID)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to load sections for invoice %s: %w", invoice.ID, err)
		}
		invoice.Sections = sections

		if err := r.getInvoiceContact(ctx, invoice); err != nil {
			return nil, 0, fmt.Errorf("failed to load contact for invoice %s: %w", invoice.ID, err)
		}
	}

	return invoices, total, nil
}

// GetByID retrieves the base invoice record with sections and contact information
func (r *Repository) GetByID(ctx context.Context, q sqlx.QueryerContext, id uuid.UUID) (*Invoice, error) {
	query := `
		SELECT
			id, clinic_id, contact_id::text, template_id, name,
			billing_period_from::text, billing_period_to::text,
			invoice_frequency, status, issue_date::text, due_date::text,
			created_at::text, updated_at::text
		FROM tbl_invoice
		WHERE id = $1 AND deleted_at IS NULL
	`

	var invoice Invoice
	err := q.QueryRowxContext(ctx, query, id).Scan(
		&invoice.ID,
		&invoice.ClinicID,
		&invoice.ContactID,
		&invoice.TemplateID,
		&invoice.Name,
		&invoice.BillingPeriodFrom,
		&invoice.BillingPeriodTo,
		&invoice.InvoiceFrequency,
		&invoice.Status,
		&invoice.IssueDate,
		&invoice.DueDate,
		&invoice.CreatedAt,
		&invoice.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	sections, err := r.sectionRepo.ListByInvoiceID(ctx, invoice.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load sections: %w", err)
	}
	invoice.Sections = sections

	if err := r.getInvoiceContact(ctx, &invoice); err != nil {
		return nil, fmt.Errorf("failed to load contact: %w", err)
	}

	return &invoice, nil
}

// GetSavedClinicMailTemplate fetches customized layout configuration overrides.
func (r *Repository) GetSavedClinicMailTemplate(ctx context.Context, clinicID uuid.UUID) (string, string, error) {
	var subject, body string
	query := `SELECT mail_subject, mail_body FROM tbl_clinic_invoice_mail_templates WHERE clinic_id = $1`
	err := r.db.QueryRowContext(ctx, query, clinicID).Scan(&subject, &body)
	if err != nil {
		return "", "", err
	}
	return subject, body, nil
}

// SaveClinicMailTemplate updates or upserts customized layout variations.
func (r *Repository) SaveClinicMailTemplate(ctx context.Context, clinicID uuid.UUID, subject, body string) error {
	query := `
		INSERT INTO tbl_clinic_invoice_mail_templates (clinic_id, mail_subject, mail_body, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (clinic_id) DO UPDATE 
		SET mail_subject = EXCLUDED.mail_subject, 
		    mail_body = EXCLUDED.mail_body, 
		    updated_at = NOW()
	`
	_, err := r.db.ExecContext(ctx, query, clinicID, subject, body)
	return err
}

// countInvoices counts total invoices matching filter
func (r *Repository) countInvoices(ctx context.Context, clinicID uuid.UUID, filter common.Filter) (int64, error) {
	allowedColumns := r.getAllowedFilterColumns()
	searchCols := []string{"name"}

	baseQuery := `FROM tbl_invoice WHERE deleted_at IS NULL AND clinic_id = ?`
	baseArgs := []interface{}{clinicID}

	countQuery, countArgs := common.BuildQuery(baseQuery, filter, allowedColumns, searchCols, true)
	countArgs = append(baseArgs, countArgs...)

	var total int64
	if err := r.db.GetContext(ctx, &total, sqlx.Rebind(sqlx.DOLLAR, countQuery), countArgs...); err != nil {
		return 0, fmt.Errorf("count invoices failed: %w", err)
	}

	return total, nil
}

// getAllowedFilterColumns returns the allowed columns for filtering
func (r *Repository) getAllowedFilterColumns() map[string]string {
	return map[string]string{
		"id":               "id",
		"name":             "name",
		"status":           "status",
		"contact_id":       "contact_id",
		"amount":           "amount",
		"date_range_start": "issue_date",
		"date_range_end":   "issue_date",
		"created_at":       "created_at",
	}
}

// validateContact validates that the contact belongs to the clinic
func (r *Repository) validateContact(ctx context.Context, clinicID uuid.UUID, contactID *uuid.UUID) error {
	if contactID == nil || *contactID == uuid.Nil {
		return ErrInvalidContactID
	}

	contact, err := r.contactRepo.Get(ctx, *contactID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrContactNotFound
		}
		return fmt.Errorf("failed to validate contact: %w", err)
	}

	if contact.ClinicId != clinicID {
		return fmt.Errorf("contact %s does not belong to clinic %s", contactID, clinicID)
	}

	return nil
}

// getInvoiceContact loads contact information for an invoice
func (r *Repository) getInvoiceContact(ctx context.Context, invoice *Invoice) error {
	if invoice.ContactID == nil {
		return nil
	}

	contact, err := r.contactRepo.Get(ctx, *invoice.ContactID)
	if err != nil {
		return fmt.Errorf("failed to load contact: %w", err)
	}

	invoice.ContactTo = &contact
	return nil
}

// UpdateWithSections updates invoice tracking matrix data elements along with array layouts
func (r *Repository) UpdateWithSections(ctx context.Context, invoice *Invoice, sections []section.Section, deleteSectionIDs []uuid.UUID, deleteItemIDs map[uuid.UUID][]uuid.UUID) error {
	return util.RunInTransaction(ctx, r.db, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := r.validateContact(ctx, invoice.ClinicID, invoice.ContactID); err != nil {
			return err
		}

		query := `
		UPDATE tbl_invoice
		SET contact_id = $1, template_id = $2, name = $3,
			billing_period_from = $4, billing_period_to = $5,
			invoice_frequency = $6, issue_date = $7, due_date = $8,
			status = $9, updated_at = NOW()
		WHERE id = $10 AND deleted_at IS NULL
	`

		result, err := tx.ExecContext(ctx, query,
			invoice.ContactID,
			invoice.TemplateID,
			invoice.Name,
			invoice.BillingPeriodFrom,
			invoice.BillingPeriodTo,
			invoice.InvoiceFrequency,
			invoice.IssueDate,
			invoice.DueDate,
			invoice.Status,
			invoice.ID,
		)
		if err != nil {
			return err
		}

		if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
			return ErrNotFound
		}

		// Evaluate and apply layout overrides
		if err := r.upsertInvoiceSettingsOverride(ctx, tx, invoice.ID, invoice.TemplateID, invoice.ClinicID, invoice.Settings); err != nil {
			return err
		}

		if err := r.sectionRepo.UpsertSections(ctx, tx, invoice.ID, sections, deleteSectionIDs, deleteItemIDs); err != nil {
			return fmt.Errorf("failed to upsert sections: %w", err)
		}

		return nil
	})
}

// Internal transactional assistant to isolate or save per-invoice configuration records dynamically
func (r *Repository) upsertInvoiceSettingsOverride(ctx context.Context, tx *sqlx.Tx, invoiceID, templateID, clinicID uuid.UUID, settings *RqInvoiceSetting) error {
	if settings == nil {
		return nil // No customizations, fallback to global settings
	}

	var existingSettingID uuid.UUID
	checkQuery := `
		SELECT setting_id 
		FROM tbl_invoice_template_mapping 
		WHERE invoice_id = $1 AND template_id = $2 AND clinic_id = $3 AND deleted_at IS NULL 
		LIMIT 1`

	err := tx.QueryRowContext(ctx, checkQuery, invoiceID, templateID, clinicID).Scan(&existingSettingID)

	if err == nil {
		// CASE A: Custom record exists. Update values.
		updateQuery := `
			UPDATE tbl_template_setting
			SET primary_color = COALESCE($1, primary_color),
				accent_color = COALESCE($2, accent_color),
				body_font_family = COALESCE($3, body_font_family),
				header_font_family = COALESCE($4, header_font_family),
				is_logo = COALESCE($5, is_logo),
				logo_id = COALESCE($6, logo_id),
				letterhead_id = COALESCE($7, letterhead_id),
				footer_id = COALESCE($8, footer_id),
				terms_text = COALESCE($9, terms_text),
				is_watermark = COALESCE($10, is_watermark),
				watermark_text = COALESCE($11, watermark_text),
				is_tax = COALESCE($12, is_tax),
				table_style = COALESCE($13, table_style),
				updated_at = NOW()
			WHERE id = $14`

		_, err = tx.ExecContext(ctx, updateQuery,
			settings.PrimaryColor, settings.AccentColor, settings.BodyFontFamily, settings.HeaderFontFamily,
			settings.IsLogo, settings.LogoID, settings.LetterheadID, settings.FooterID, settings.TermsText,
			settings.IsWatermark, settings.WatermarkText, settings.IsTax, settings.TableStyle,
			existingSettingID,
		)
		if err != nil {
			return fmt.Errorf("failed upgrading existing custom invoice option values profiles: %w", err)
		}
		return nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		// CASE B: First customization. Initialize new records.
		newMappingID := uuid.New()
		newSettingID := uuid.New()

		insertSettingQuery := `
			INSERT INTO tbl_template_setting (
				id, mapping_id, primary_color, accent_color, body_font_family, header_font_family,
				is_logo, logo_id, letterhead_id, footer_id, terms_text, is_watermark, watermark_text, is_tax, table_style,
				created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, NOW(), NOW())`

		_, err = tx.ExecContext(ctx, insertSettingQuery,
			newSettingID, newMappingID,
			settings.PrimaryColor, settings.AccentColor, settings.BodyFontFamily, settings.HeaderFontFamily,
			settings.IsLogo, settings.LogoID, settings.LetterheadID, settings.FooterID, settings.TermsText,
			settings.IsWatermark, settings.WatermarkText, settings.IsTax, settings.TableStyle,
		)
		if err != nil {
			return fmt.Errorf("failed provisioning custom settings payload block: %w", err)
		}

		insertMappingQuery := `
			INSERT INTO tbl_invoice_template_mapping (id, invoice_id, template_id, setting_id, clinic_id, created_at)
			VALUES ($1, $2, $3, $4, $5, NOW())`

		_, err = tx.ExecContext(ctx, insertMappingQuery, newMappingID, invoiceID, templateID, newSettingID, clinicID)
		if err != nil {
			return fmt.Errorf("failed linking custom blueprint overrides profile records: %w", err)
		}
		return nil
	}

	return err
}
