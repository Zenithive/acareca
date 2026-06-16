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
			for i := range invoice.Sections {
				section := &invoice.Sections[i]
				sectionID := uuid.New()

				// Calculate totals before saving
				section.CalculateTotals()

				_, err := tx.ExecContext(ctx, `
					INSERT INTO tbl_map_invoice_section (id, invoice_id, invoice_section, document_number, tax_method, tax_rate,
						payment_method, account_name, bsb_number, account_number, payment_date, payment_reference)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
					ON CONFLICT (invoice_id, invoice_section) DO UPDATE SET 
						document_number = EXCLUDED.document_number,
						tax_method = EXCLUDED.tax_method,
						tax_rate = EXCLUDED.tax_rate,
						payment_method = EXCLUDED.payment_method,
						account_name = EXCLUDED.account_name,
						bsb_number = EXCLUDED.bsb_number,
						account_number = EXCLUDED.account_number,
						payment_date = EXCLUDED.payment_date,
						payment_reference = EXCLUDED.payment_reference,
						updated_at = NOW()
					RETURNING id`,
					sectionID, invoice.ID, section.SectionType, section.DocumentNumber, section.TaxMethod, section.TaxRate,
					section.PaymentMethod, section.AccountName, section.BSBNumber, section.AccountNumber, section.PaymentDate, section.PaymentReference)
				if err != nil {
					return err
				}
				sectionIDMap[section.SectionType] = sectionID

				// Link entries from this section to the section ID
				for _, entry := range section.Entries {
					entryID := uuid.New()
					_, err := tx.ExecContext(ctx, `
						INSERT INTO tbl_invoice_item (id, invoice_id, invoice_section_id, name, quantity, unit_price, total_amount, bas_code)
						VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
					`, entryID, invoice.ID, sectionID, entry.Name, entry.Quantity, entry.UnitPrice, entry.TotalAmount, entry.BASCode)
					if err != nil {
						return err
					}
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

		return nil
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
		SELECT id, invoice_section, document_number, tax_method, tax_rate,
		payment_method, account_name, bsb_number, account_number, payment_date::text, payment_reference
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

		if err := rows.Scan(&sectionID, &section.SectionType, &section.DocumentNumber, &section.TaxMethod, &section.TaxRate,
			&section.PaymentMethod, &section.AccountName, &section.BSBNumber, &section.AccountNumber, &section.PaymentDate, &section.PaymentReference,
		); err != nil {
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

	// Group items by section securely by accessing via index slice references
	for i := range items {
		if items[i].InvoiceSectionID != nil {
			if idx, ok := sectionIDMap[*items[i].InvoiceSectionID]; ok {
				// Force assign parent invoice ID if itemRepo returned zero-value UUIDs
				if items[i].InvoiceID == uuid.Nil {
					items[i].InvoiceID = invoice.ID
				}
				invoice.Sections[idx].Entries = append(invoice.Sections[idx].Entries, items[i])
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
			SELECT id, invoice_section, document_number, tax_method, tax_rate,
			payment_method, account_name, bsb_number, account_number, payment_date::text, payment_reference
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

			if err := sectionRows.Scan(&sectionID, &section.SectionType, &section.DocumentNumber, &section.TaxMethod, &section.TaxRate,
				&section.PaymentMethod, &section.AccountName, &section.BSBNumber, &section.AccountNumber, &section.PaymentDate, &section.PaymentReference,
			); err != nil {
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
		for i := range invoice.Items {
			if invoice.Items[i].InvoiceSectionID != nil {
				if idx, ok := sectionIDMap[*invoice.Items[i].InvoiceSectionID]; ok {
					// Force assign parent invoice ID if itemRepo returned zero-value UUIDs
					if invoice.Items[i].InvoiceID == uuid.Nil {
						invoice.Items[i].InvoiceID = invoice.ID
					}
					invoice.Sections[idx].Entries = append(invoice.Sections[idx].Entries, invoice.Items[i])
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
			for i := range invoice.Sections {
				section := &invoice.Sections[i]
				requestedSectionTypes[section.SectionType] = true

				// Calculate totals before saving
				section.CalculateTotals()

				var currentSectionID uuid.UUID
				if existingID, exists := existingSections[section.SectionType]; exists {
					_, err := tx.ExecContext(ctx, `
						UPDATE tbl_map_invoice_section 
						SET document_number = $1, tax_method = $2, tax_rate = $3, 
							payment_method = $4, account_name = $5, bsb_number = $6, account_number = $7, 
							payment_date = $8, payment_reference = $9, updated_at = NOW()
						WHERE id = $10`,
						section.DocumentNumber, section.TaxMethod, section.TaxRate,
						section.PaymentMethod, section.AccountName, section.BSBNumber, section.AccountNumber,
						section.PaymentDate, section.PaymentReference, existingID)
					if err != nil {
						return err
					}
					currentSectionID = existingID
				} else {
					// Insert new section
					currentSectionID = uuid.New()
					_, err := tx.ExecContext(ctx, `
						INSERT INTO tbl_map_invoice_section (id, invoice_id, invoice_section, document_number, tax_method, tax_rate,
							payment_method, account_name, bsb_number, account_number, payment_date, payment_reference)
						VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
						currentSectionID, invoice.ID, section.SectionType, section.DocumentNumber, section.TaxMethod, section.TaxRate,
						section.PaymentMethod, section.AccountName, section.BSBNumber, section.AccountNumber, section.PaymentDate, section.PaymentReference)
					if err != nil {
						return err
					}
				}
				sectionIDMap[section.SectionType] = currentSectionID
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

		// Directly write newly sent items to the database within updates
		if len(invoice.Sections) > 0 {
			for i := range invoice.Sections {
				section := &invoice.Sections[i]
				if currentSectionID, ok := sectionIDMap[section.SectionType]; ok {
					for _, entry := range section.Entries {
						entryID := uuid.New()
						_, err := tx.ExecContext(ctx, `
							INSERT INTO tbl_invoice_item (id, invoice_id, invoice_section_id, name, quantity, unit_price, total_amount, bas_code)
							VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
						`, entryID, invoice.ID, currentSectionID, entry.Name, entry.Quantity, entry.UnitPrice, entry.TotalAmount, entry.BASCode)
						if err != nil {
							return err
						}
					}
				}
			}
		}

		return nil
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
