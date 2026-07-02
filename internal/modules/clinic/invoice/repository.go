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
	List(ctx context.Context, filter common.Filter) ([]*Invoice, int64, error)
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
			id, clinic_id, contact_id, name,
			billing_period_from, billing_period_to, invoice_frequency,
			issue_date, due_date, status, invoice_method
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

		_, err := tx.ExecContext(ctx, query,
			invoice.ID,
			invoice.ClinicID,
			invoice.ContactID,
			invoice.Name,
			invoice.BillingPeriodFrom,
			invoice.BillingPeriodTo,
			invoice.InvoiceFrequency,
			invoice.IssueDate,
			invoice.DueDate,
			invoice.Status,
			invoice.InvoiceMethod,
		)
		if err != nil {
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
		SET contact_id = $1, name = $2,
			billing_period_from = $3, billing_period_to = $4,
			invoice_frequency = $5, issue_date = $6, due_date = $7,
			status = $8, invoice_method = $9, updated_at = NOW()
		WHERE id = $10 AND deleted_at IS NULL
	`

		result, err := tx.ExecContext(ctx, query,
			invoice.ContactID,
			invoice.Name,
			invoice.BillingPeriodFrom,
			invoice.BillingPeriodTo,
			invoice.InvoiceFrequency,
			invoice.IssueDate,
			invoice.DueDate,
			invoice.Status,
			invoice.InvoiceMethod,
			invoice.ID,
		)
		if err != nil {
			return err
		}

		if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
			return ErrNotFound
		}

		// Get existing sections
		existingSections, err := r.sectionRepo.ListByInvoiceID(ctx, invoice.ID)
		if err != nil {
			return fmt.Errorf("failed to get existing sections: %w", err)
		}

		if len(existingSections) > 0 {
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

		sections, err := r.sectionRepo.ListByInvoiceID(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to get sections for deletion: %w", err)
		}

		for _, sec := range sections {
			if err := r.sectionRepo.Delete(ctx, tx, sec.ID); err != nil {
				return fmt.Errorf("failed to delete section: %w", err)
			}
		}

		return nil
	})
}

// List implements [IRepository].
func (r *Repository) List(ctx context.Context, filter common.Filter) ([]*Invoice, int64, error) {
	total, err := r.countInvoices(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	allowedColumns := r.getAllowedFilterColumns()
	searchCols := []string{}

	selectQuery := `
		SELECT 
			id, clinic_id, contact_id, name,
			billing_period_from, billing_period_to,
			invoice_frequency, status, issue_date, due_date, invoice_method,
			created_at, updated_at
		FROM tbl_invoice
		WHERE deleted_at IS NULL
	`

	query, args := common.BuildQuery(selectQuery, filter, allowedColumns, searchCols, false)
	invoices := make([]*Invoice, 0)
	err = r.db.SelectContext(ctx, &invoices, sqlx.Rebind(sqlx.DOLLAR, query), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("select invoices failed: %w", err)
	}

	for _, invoice := range invoices {
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
			id, clinic_id, contact_id::text, name,
			billing_period_from::text, billing_period_to::text,
			invoice_frequency, status, issue_date, due_date, invoice_method,
			created_at, updated_at
		FROM tbl_invoice
		WHERE id = $1 AND deleted_at IS NULL
	`

	var invoice Invoice
	err := q.QueryRowxContext(ctx, query, id).Scan(
		&invoice.ID,
		&invoice.ClinicID,
		&invoice.ContactID,
		&invoice.Name,
		&invoice.BillingPeriodFrom,
		&invoice.BillingPeriodTo,
		&invoice.InvoiceFrequency,
		&invoice.Status,
		&invoice.IssueDate,
		&invoice.DueDate,
		&invoice.InvoiceMethod,
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
func (r *Repository) countInvoices(ctx context.Context, filter common.Filter) (int64, error) {
	allowedColumns := r.getAllowedFilterColumns()
	searchCols := []string{}

	baseQuery := `FROM tbl_invoice WHERE deleted_at IS NULL`

	countQuery, countArgs := common.BuildQuery(baseQuery, filter, allowedColumns, searchCols, true)
	var total int64
	if err := r.db.GetContext(ctx, &total, sqlx.Rebind(sqlx.DOLLAR, countQuery), countArgs...); err != nil {
		return 0, fmt.Errorf("count invoices failed: %w", err)
	}

	return total, nil
}

// getAllowedFilterColumns returns the allowed columns for filtering
func (r *Repository) getAllowedFilterColumns() map[string]string {
	return map[string]string{
		"id":                "id",
		"name":              "name",
		"status":            "status",
		"contact_id":        "contact_id",
		"invoice_frequency": "invoice_frequency",
		"issue_date":        "issue_date",
		"due_date":          "due_date",
		"invoice_method":    "invoice_method",
		"clinic_id":         "clinic_id",
		"deleted_at":        "deleted_at",
		"date_range_start":  "issue_date",
		"date_range_end":    "issue_date",
		"created_at":        "created_at",
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

		allItems := make([]*item.Item, 0)
		for i := range sections {
			allItems = append(allItems, sections[i].Entries...)
		}

		if len(allItems) > 0 {
			if err := r.itemRepo.EvaluateFormulas(ctx, allItems); err != nil {
				return fmt.Errorf("formula evaluation failed: %w", err)
			}
		}

		query := `
		UPDATE tbl_invoice
		SET contact_id = $1,  name = $2,
			billing_period_from = $3, billing_period_to = $4,
			invoice_frequency = $5, issue_date = $6, due_date = $7,
			status = $8, invoice_method = $9, updated_at = NOW()
		WHERE id = $10 AND deleted_at IS NULL
	`

		result, err := tx.ExecContext(ctx, query,
			invoice.ContactID,
			invoice.Name,
			invoice.BillingPeriodFrom,
			invoice.BillingPeriodTo,
			invoice.InvoiceFrequency,
			invoice.IssueDate,
			invoice.DueDate,
			invoice.Status,
			invoice.InvoiceMethod,
			invoice.ID,
		)
		if err != nil {
			return err
		}

		if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
			return ErrNotFound
		}

		if err := r.sectionRepo.UpsertSections(ctx, tx, invoice.ID, sections, deleteSectionIDs, deleteItemIDs); err != nil {
			return fmt.Errorf("failed to upsert sections: %w", err)
		}

		return nil
	})
}
