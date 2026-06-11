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

		subtotal, taxTotal, grandTotal := calculateTotals(invoice.Items)

		_, err := tx.ExecContext(ctx, `
			INSERT INTO tbl_invoice (
				id,
				clinic_id,
				contact_id,
				template_id,
				name,
				invoice_number,
				reference,
				payment_method,
				tax_method,
				issue_date,
				due_date,
				subtotal,
				tax_total,
				grand_total,
				status
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		`,
			invoice.ID,
			invoice.ClinicID,
			invoice.ContactID,
			invoice.TemplateID,
			invoice.Name,
			invoice.InvoiceNumber,
			invoice.Reference,
			invoice.PaymentMethod,
			invoice.TaxMethod,
			invoice.IssueDate,
			invoice.DueDate,
			subtotal,
			taxTotal,
			grandTotal,
			invoice.Status,
		)
		if err != nil {
			return err
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

		_, err = tx.ExecContext(ctx, `
			UPDATE tbl_invoice_item
			SET deleted_at = NOW(), updated_at = NOW()
			WHERE invoice_id = $1
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
			invoice_number,
			reference,
			payment_method,
			tax_method,
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
		&invoice.InvoiceNumber,
		&invoice.Reference,
		&invoice.PaymentMethod,
		&invoice.TaxMethod,
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

	items, err := r.itemRepo.GetByInvoiceID(ctx, nil, invoice.ID)
	if err != nil {
		return nil, err
	}
	invoice.Items = items

	return &invoice, nil
}

// List implements [IRepository].
func (r *Repository) List(ctx context.Context, clinicID uuid.UUID, filter common.Filter) ([]*Invoice, int64, error) {
	allowedColumns := map[string]string{
		"id":               "id",
		"name":             "name",
		"status":           "status",
		"contact_id":       "contact_id",
		"invoice_number":   "invoice_number",
		"amount":           "amount",
		"date_range_start": "issue_date",
		"date_range_end":   "issue_date",
		"created_at":       "created_at",
	}

	searchCols := []string{"name", "invoice_number"}

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
			invoice_number,
			reference,
			payment_method,
			tax_method,
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
			&invoice.InvoiceNumber,
			&invoice.Reference,
			&invoice.PaymentMethod,
			&invoice.TaxMethod,
			&invoice.Status,
			&invoice.IssueDate,
			&invoice.DueDate,
			&invoice.CreatedAt,
			&invoice.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}

		invoice.Items, err = r.itemRepo.GetByInvoiceID(ctx, r.db, invoice.ID)
		if err != nil {
			return nil, 0, err
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

		subtotal, taxTotal, grandTotal := calculateTotals(invoice.Items)

		result, err := tx.ExecContext(ctx, `
			UPDATE tbl_invoice
			SET
				contact_id = $1,
				template_id = $2,
				name = $3,
				invoice_number = $4,
				reference = $5,
				payment_method = $6,
				tax_method = $7,
				issue_date = $8,
				due_date = $9,
				subtotal = $10,
				tax_total = $11,
				grand_total = $12,
				status = $13,
				updated_at = NOW()
			WHERE id = $14
			AND deleted_at IS NULL
		`,
			invoice.ContactID,
			invoice.TemplateID,
			invoice.Name,
			invoice.InvoiceNumber,
			invoice.Reference,
			invoice.PaymentMethod,
			invoice.TaxMethod,
			invoice.IssueDate,
			invoice.DueDate,
			subtotal,
			taxTotal,
			grandTotal,
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

		_, err = tx.ExecContext(ctx, `
			UPDATE tbl_invoice_item
			SET deleted_at = NOW(), updated_at = NOW()
			WHERE invoice_id = $1
			AND deleted_at IS NULL
		`, invoice.ID)
		if err != nil {
			return err
		}

		for _, invoiceItem := range invoice.Items {
			invoiceItem.ID = uuid.New()
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

func calculateTotals(items []*item.Item) (float64, float64, float64) {
	var taxTotal float64
	var grandTotal float64

	for _, invoiceItem := range items {
		if invoiceItem.TaxAmount != nil {
			taxTotal += *invoiceItem.TaxAmount
		}
		grandTotal += invoiceItem.TotalAmount
	}

	return grandTotal - taxTotal, taxTotal, grandTotal
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
