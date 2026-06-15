package invoice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	contactpkg "github.com/iamarpitzala/acareca/internal/modules/clinic/contact"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/item"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("invoice not found")

type IRepository interface {
	Create(ctx context.Context, invoice *Invoice) error
	Update(ctx context.Context, invoice *Invoice) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*Invoice, error)
	List(ctx context.Context, clinicID uuid.UUID, filter common.Filter) ([]*Invoice, int64, error)
	GetSavedClinicMailTemplate(ctx context.Context, clinicID uuid.UUID) (string, string, error)
	SaveClinicMailTemplate(ctx context.Context, clinicID uuid.UUID, subject, body string) error
}

type Repository struct {
	db          *sqlx.DB
	itemRepo    item.IRepository
	contactRepo contactpkg.Repository
}

func NewRepository(db *sqlx.DB) IRepository {
	return &Repository{
		db:          db,
		itemRepo:    item.NewRepository(db),
		contactRepo: contactpkg.NewRepository(db),
	}
}

// Create implements [IRepository].
func (r *Repository) Create(ctx context.Context, invoice *Invoice) error {
	return util.RunInTransaction(ctx, r.db, func(ctx context.Context, tx *sqlx.Tx) error {
		if invoice.ID == uuid.Nil {
			invoice.ID = uuid.New()
		}
		if err := r.validateContactTo(ctx, invoice); err != nil {
			return err
		}

		_, err := tx.ExecContext(ctx, `
			INSERT INTO tbl_invoice (
				id,
				clinic_id,
				contact_id,
				template_id,
				name,
				billing_period_from,
				billing_period_to,
				invoice_frequency,
				issue_date,
				due_date,
				status
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		`,
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

		// Insert invoice sections and build a map of section type to section ID
		sectionIDMap := make(map[string]uuid.UUID)
		if len(invoice.Sections) > 0 {
			for _, section := range invoice.Sections {
				sectionID := uuid.New()

				// Calculate totals before saving
				section.CalculateTotals()

				_, err := tx.ExecContext(ctx, `
					INSERT INTO tbl_map_invoice_section (id, invoice_id, invoice_section, document_number, tax_method, tax_rate)
					VALUES ($1, $2, $3, $4, $5, $6)
					ON CONFLICT (invoice_id, invoice_section) DO UPDATE SET 
						document_number = EXCLUDED.document_number,
						tax_method = EXCLUDED.tax_method,
						tax_rate = EXCLUDED.tax_rate,
						updated_at = NOW()
					RETURNING id
				`, sectionID, invoice.ID, section.SectionType, section.DocumentNumber, section.TaxMethod, section.TaxRate)
				if err != nil {
					return err
				}
				sectionIDMap[section.SectionType] = sectionID

				// Link entries from this section to the section ID
				for _, entry := range section.Entries {
					entry.InvoiceSectionID = &sectionID
				}
			}
		} else {
			// If no sections provided, create a default CALCULATION_STATEMENT section
			sectionID := uuid.New()
			defaultTaxMethod := "NO_TAX"
			defaultTaxRate := 0.0
			_, err := tx.ExecContext(ctx, `
				INSERT INTO tbl_map_invoice_section (id, invoice_id, invoice_section, document_number, tax_method, tax_rate)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, sectionID, invoice.ID, "CALCULATION_STATEMENT", invoice.ID.String()[:8], defaultTaxMethod, defaultTaxRate)
			if err != nil {
				return err
			}
			sectionIDMap["CALCULATION_STATEMENT"] = sectionID
		}

		// Ensure all items in invoice.Items have a section assigned
		for _, item := range invoice.Items {
			if item.InvoiceSectionID == nil && len(sectionIDMap) > 0 {
				// Get first section ID from map
				for _, secID := range sectionIDMap {
					item.InvoiceSectionID = &secID
					break
				}
			}
		}

		return r.itemRepo.Create(ctx, tx, invoice.ID, invoice.Items)
	})
}

// Delete implements [IRepository].
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	return util.RunInTransaction(ctx, r.db, func(ctx context.Context, tx *sqlx.Tx) error {
		result, err := tx.ExecContext(ctx, `
			UPDATE tbl_invoice
			SET deleted_at = NOW(), updated_at = NOW()
			WHERE id = $1
			AND deleted_at IS NULL
		`, id)
		if err != nil {
			return err
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rowsAffected == 0 {
			return ErrNotFound
		}

		// Soft delete all invoice sections
		_, err = tx.ExecContext(ctx, `
			UPDATE tbl_map_invoice_section
			SET deleted_at = NOW(), updated_at = NOW()
			WHERE invoice_id = $1
			AND deleted_at IS NULL
		`, id)
		if err != nil {
			return err
		}

		// Soft delete all invoice items
		_, err = tx.ExecContext(ctx, `
			UPDATE tbl_invoice_item
			SET deleted_at = NOW(), updated_at = NOW()
			WHERE invoice_section_id IN (
				SELECT id FROM tbl_map_invoice_section WHERE invoice_id = $1
			)
			AND deleted_at IS NULL
		`, id)
		return err
	})
}

// Get implements [IRepository].
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*Invoice, error) {
	var invoice Invoice

	err := r.db.QueryRowContext(ctx, `
		SELECT
			id,
			clinic_id,
			contact_id::text,
			template_id,
			name,
			billing_period_from::text,
			billing_period_to::text,
			invoice_frequency,
			status,
			issue_date::text,
			due_date::text,
			created_at::text,
			updated_at::text
		FROM tbl_invoice
		WHERE id = $1
		AND deleted_at IS NULL
	`, id).Scan(
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

	if invoice.ContactID != nil {
		contact, err := r.contactRepo.Get(ctx, *invoice.ContactID)
		if err != nil {
			return nil, err
		}

		invoice.ContactTo = &contact
	}

	// Get invoice sections
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, invoice_section, document_number, tax_method, tax_rate
		FROM tbl_map_invoice_section
		WHERE invoice_id = $1
		AND deleted_at IS NULL
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sections := make([]InvoiceSection, 0)
	sectionIDMap := make(map[uuid.UUID]int) // Map section ID to index in sections slice
	for rows.Next() {
		var section InvoiceSection
		var sectionID uuid.UUID
		if err := rows.Scan(&sectionID, &section.SectionType, &section.DocumentNumber, &section.TaxMethod, &section.TaxRate); err != nil {
			return nil, err
		}
		section.Entries = make([]*item.Item, 0)
		sectionIDMap[sectionID] = len(sections)
		sections = append(sections, section)
	}
	invoice.Sections = sections

	items, err := r.itemRepo.GetByInvoiceID(ctx, nil, invoice.ID)
	if err != nil {
		return nil, err
	}
	invoice.Items = items

	// Group items by section
	for _, itm := range items {
		if itm.InvoiceSectionID != nil {
			if idx, ok := sectionIDMap[*itm.InvoiceSectionID]; ok {
				invoice.Sections[idx].Entries = append(invoice.Sections[idx].Entries, itm)
			}
		}
	}

	// Calculate totals for each section
	for i := range invoice.Sections {
		invoice.Sections[i].CalculateTotals()
	}

	return &invoice, nil
}

// List implements [IRepository].
func (r *Repository) List(ctx context.Context, clinicID uuid.UUID, filter common.Filter) ([]*Invoice, int64, error) {
	allowedColumns := map[string]string{
		"id":               "id",
		"name":             "name",
		"status":           "status",
		"contact_id":       "contact_id",
		"amount":           "amount",
		"date_range_start": "issue_date",
		"date_range_end":   "issue_date",
		"created_at":       "created_at",
	}

	searchCols := []string{"name"}

	baseQuery := `FROM tbl_invoice WHERE deleted_at IS NULL AND clinic_id = ?`
	baseArgs := []interface{}{clinicID}

	countQueryPart, countArgsPart := common.BuildQuery(baseQuery, filter, allowedColumns, searchCols, true)
	countArgs := append(baseArgs, countArgsPart...)

	var total int64
	if err := r.db.GetContext(ctx, &total, sqlx.Rebind(sqlx.DOLLAR, countQueryPart), countArgs...); err != nil {
		return nil, 0, fmt.Errorf("count invoices failed: %w", err)
	}

	selectQueryBase := `SELECT 
			id,
			clinic_id,
			contact_id::text,
			template_id,
			name,
			billing_period_from::text,
			billing_period_to::text,
			invoice_frequency,
			status,
			issue_date::text,
			due_date::text,
			created_at::text,
			updated_at::text ` + baseQuery

	itemsQuery, itemsArgsPart := common.BuildQuery(selectQueryBase, filter, allowedColumns, searchCols, false)
	itemsArgs := append(baseArgs, itemsArgsPart...)

	rows, err := r.db.QueryContext(ctx, sqlx.Rebind(sqlx.DOLLAR, itemsQuery), itemsArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("select invoices failed: %w", err)
	}
	defer rows.Close()

	invoices := make([]*Invoice, 0)
	for rows.Next() {
		var invoice Invoice

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
			return nil, 0, err
		}

		// Get invoice sections for this invoice
		sectionRows, err := r.db.QueryContext(ctx, `
			SELECT id, invoice_section, document_number, tax_method, tax_rate
			FROM tbl_map_invoice_section
			WHERE invoice_id = $1
			AND deleted_at IS NULL
		`, invoice.ID)
		if err != nil {
			return nil, 0, err
		}

		sections := make([]InvoiceSection, 0)
		sectionIDMap := make(map[uuid.UUID]int) // Map section ID to index in sections slice
		for sectionRows.Next() {
			var section InvoiceSection
			var sectionID uuid.UUID
			if err := sectionRows.Scan(&sectionID, &section.SectionType, &section.DocumentNumber, &section.TaxMethod, &section.TaxRate); err != nil {
				sectionRows.Close()
				return nil, 0, err
			}
			section.Entries = make([]*item.Item, 0)
			sectionIDMap[sectionID] = len(sections)
			sections = append(sections, section)
		}
		sectionRows.Close()
		invoice.Sections = sections

		invoice.Items, err = r.itemRepo.GetByInvoiceID(ctx, r.db, invoice.ID)
		if err != nil {
			return nil, 0, err
		}

		// Group items by section
		for _, itm := range invoice.Items {
			if itm.InvoiceSectionID != nil {
				if idx, ok := sectionIDMap[*itm.InvoiceSectionID]; ok {
					invoice.Sections[idx].Entries = append(invoice.Sections[idx].Entries, itm)
				}
			}
		}

		// Calculate totals for each section
		for i := range invoice.Sections {
			invoice.Sections[i].CalculateTotals()
		}

		invoices = append(invoices, &invoice)
	}

	return invoices, total, rows.Err()
}

// Update implements [IRepository].
func (r *Repository) Update(ctx context.Context, invoice *Invoice) error {
	return util.RunInTransaction(ctx, r.db, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := r.validateContactTo(ctx, invoice); err != nil {
			return err
		}

		result, err := tx.ExecContext(ctx, `
			UPDATE tbl_invoice
			SET
				contact_id = $1,
				template_id = $2,
				name = $3,
				billing_period_from = $4,
				billing_period_to = $5,
				invoice_frequency = $6,
				issue_date = $7,
				due_date = $8,
				status = $9,
				updated_at = NOW()
			WHERE id = $10
			AND deleted_at IS NULL
		`,
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

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rowsAffected == 0 {
			return ErrNotFound
		}

		// Update invoice sections
		// First, get existing section IDs for this invoice
		existingSections := make(map[string]uuid.UUID) // sectionType -> sectionID
		rows, err := tx.QueryContext(ctx, `
			SELECT id, invoice_section FROM tbl_map_invoice_section 
			WHERE invoice_id = $1 AND deleted_at IS NULL
		`, invoice.ID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var sectionID uuid.UUID
			var sectionType string
			if err := rows.Scan(&sectionID, &sectionType); err != nil {
				rows.Close()
				return err
			}
			existingSections[sectionType] = sectionID
		}
		rows.Close()

		// Build new section map and update/insert sections
		sectionIDMap := make(map[string]uuid.UUID)
		requestedSectionTypes := make(map[string]bool)

		if len(invoice.Sections) > 0 {
			for _, section := range invoice.Sections {
				requestedSectionTypes[section.SectionType] = true

				// Calculate totals before saving
				section.CalculateTotals()

				if existingID, exists := existingSections[section.SectionType]; exists {
					_, err := tx.ExecContext(ctx, `
						UPDATE tbl_map_invoice_section 
						SET document_number = $1, tax_method = $2, tax_rate = $3, updated_at = NOW()
						WHERE id = $4
					`, section.DocumentNumber, section.TaxMethod, section.TaxRate, existingID)
					if err != nil {
						return err
					}
					sectionIDMap[section.SectionType] = existingID
				} else {
					// Insert new section
					sectionID := uuid.New()
					_, err := tx.ExecContext(ctx, `
						INSERT INTO tbl_map_invoice_section (id, invoice_id, invoice_section, document_number, tax_method, tax_rate)
						VALUES ($1, $2, $3, $4, $5, $6)
					`, sectionID, invoice.ID, section.SectionType, section.DocumentNumber, section.TaxMethod, section.TaxRate)
					if err != nil {
						return err
					}
					sectionIDMap[section.SectionType] = sectionID
				}
			}
		} else {
			// If no sections provided, ensure default CALCULATION_STATEMENT section exists
			if existingID, exists := existingSections["CALCULATION_STATEMENT"]; exists {
				sectionIDMap["CALCULATION_STATEMENT"] = existingID
			} else {
				sectionID := uuid.New()
				defaultTaxMethod := "NO_TAX"
				defaultTaxRate := 0.0
				_, err := tx.ExecContext(ctx, `
					INSERT INTO tbl_map_invoice_section (id, invoice_id, invoice_section, document_number, tax_method, tax_rate)
					VALUES ($1, $2, $3, $4, $5, $6)
				`, sectionID, invoice.ID, "CALCULATION_STATEMENT", invoice.ID.String()[:8], defaultTaxMethod, defaultTaxRate)
				if err != nil {
					return err
				}
				sectionIDMap["CALCULATION_STATEMENT"] = sectionID
			}
			requestedSectionTypes["CALCULATION_STATEMENT"] = true
		}

		// Soft delete sections that are no longer in the request
		for sectionType, sectionID := range existingSections {
			if !requestedSectionTypes[sectionType] {
				_, err := tx.ExecContext(ctx, `
					UPDATE tbl_map_invoice_section 
					SET deleted_at = NOW(), updated_at = NOW()
					WHERE id = $1
				`, sectionID)
				if err != nil {
					return err
				}
			}
		}

		// Soft delete all existing items for sections that are being updated
		for sectionType := range requestedSectionTypes {
			if sectionID, ok := sectionIDMap[sectionType]; ok {
				_, err := tx.ExecContext(ctx, `
					UPDATE tbl_invoice_item
					SET deleted_at = NOW(), updated_at = NOW()
					WHERE invoice_section_id = $1 AND deleted_at IS NULL
				`, sectionID)
				if err != nil {
					return err
				}
			}
		}

		// Link items to their sections or default section
		for _, item := range invoice.Items {
			item.ID = uuid.New()
			if item.InvoiceSectionID == nil && len(sectionIDMap) > 0 {
				for _, secID := range sectionIDMap {
					item.InvoiceSectionID = &secID
					break
				}
			}
		}

		return r.itemRepo.Create(ctx, tx, invoice.ID, invoice.Items)
	})
}

func (r *Repository) validateContactTo(ctx context.Context, invoice *Invoice) error {
	if invoice.ContactID == nil || *invoice.ContactID == uuid.Nil {
		return errors.New("contact_id is required")
	}

	contactTo, err := r.contactRepo.Get(ctx, *invoice.ContactID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("contact not found")
		}
		return err
	}
	if contactTo.ClinicId != invoice.ClinicID {
		return fmt.Errorf("contact %s does not belong to clinic %s", invoice.ContactID.String(), invoice.ClinicID.String())
	}

	return nil
}

func (r *Repository) GetSavedClinicMailTemplate(ctx context.Context, clinicID uuid.UUID) (string, string, error) {
	var subject, body string

	err := r.db.QueryRowContext(ctx, `
		SELECT mail_subject, mail_body 
		FROM tbl_clinic_invoice_mail_templates 
		WHERE clinic_id = $1
	`, clinicID).Scan(&subject, &body)

	if err != nil {
		return "", "", err // Service defaults automatically capture sql.ErrNoRows fallbacks gracefully
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
