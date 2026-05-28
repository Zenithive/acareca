package invoice

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/item"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type IRepository interface {
	Create(ctx context.Context, invoice *Invoice) error
	Update(ctx context.Context, invoice *Invoice) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*Invoice, error)
	List(ctx context.Context, filter common.Filter) ([]*Invoice, error)
}

type Repository struct {
	db       *sqlx.DB
	itemRepo item.IRepository
}

func NewRepository(db *sqlx.DB) IRepository {
	return &Repository{
		db:       db,
		itemRepo: item.NewRepository(db),
	}
}

// Create implements [IRepository].
func (r *Repository) Create(ctx context.Context, invoice *Invoice) error {
	return util.RunInTransaction(ctx, r.db, func(ctx context.Context, tx *sqlx.Tx) error {
		if invoice.ID == uuid.Nil {
			invoice.ID = uuid.New()
		}

		subtotal, taxTotal, grandTotal := calculateTotals(invoice.Items)

		_, err := tx.ExecContext(ctx, `
			INSERT INTO tbl_invoice (
				id,
				clinic_id,
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
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		`,
			invoice.ID,
			invoice.ClinicID,
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
			return errors.New("invoice not found")
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
		return nil, err
	}

	items, err := r.itemRepo.GetByInvoiceID(ctx, nil, invoice.ID)
	if err != nil {
		return nil, err
	}
	invoice.Items = items

	return &invoice, nil
}

// List implements [IRepository].
func (r *Repository) List(ctx context.Context, filter common.Filter) ([]*Invoice, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id,
			clinic_id,
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
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	invoices := make([]*Invoice, 0)
	for rows.Next() {
		var invoice Invoice
		if err := rows.Scan(
			&invoice.ID,
			&invoice.ClinicID,
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
			return nil, err
		}

		invoice.Items, err = r.itemRepo.GetByInvoiceID(ctx, r.db, invoice.ID)
		if err != nil {
			return nil, err
		}

		invoices = append(invoices, &invoice)
	}

	return invoices, rows.Err()
}

// Update implements [IRepository].
func (r *Repository) Update(ctx context.Context, invoice *Invoice) error {
	return util.RunInTransaction(ctx, r.db, func(ctx context.Context, tx *sqlx.Tx) error {
		subtotal, taxTotal, grandTotal := calculateTotals(invoice.Items)

		result, err := tx.ExecContext(ctx, `
			UPDATE tbl_invoice
			SET
				template_id = $1,
				name = $2,
				invoice_number = $3,
				reference = $4,
				payment_method = $5,
				tax_method = $6,
				issue_date = $7,
				due_date = $8,
				subtotal = $9,
				tax_total = $10,
				grand_total = $11,
				status = $12,
				updated_at = NOW()
			WHERE id = $13
			AND deleted_at IS NULL
		`,
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
			return errors.New("invoice not found")
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
