package invoice

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/item"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type IRepository interface {
	Create(ctx context.Context, invoice *Invoice) error
	Update(ctx context.Context, invoice *Invoice) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*Invoice, error)
	List(ctx context.Context) ([]*Invoice, error)
}

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) IRepository {
	return &Repository{
		db: db,
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
				grand_total
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
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
		)
		if err != nil {
			return err
		}

		return r.insertItemsTx(ctx, tx, invoice.ID, invoice.Items)
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
		&invoice.IssueDate,
		&invoice.DueDate,
		&invoice.CreatedAt,
		&invoice.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	items, err := r.getItemsByInvoiceID(ctx, invoice.ID)
	if err != nil {
		return nil, err
	}
	invoice.Items = items

	return &invoice, nil
}

// List implements [IRepository].
func (r *Repository) List(ctx context.Context) ([]*Invoice, error) {
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
			&invoice.IssueDate,
			&invoice.DueDate,
			&invoice.CreatedAt,
			&invoice.UpdatedAt,
		); err != nil {
			return nil, err
		}

		invoice.Items, err = r.getItemsByInvoiceID(ctx, invoice.ID)
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
				updated_at = NOW()
			WHERE id = $12
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

		return r.insertItemsTx(ctx, tx, invoice.ID, invoice.Items)
	})
}

func (r *Repository) insertItemsTx(ctx context.Context, tx *sqlx.Tx, invoiceID uuid.UUID, items []*item.Item) error {
	for idx, invoiceItem := range items {
		if invoiceItem.ID == uuid.Nil {
			invoiceItem.ID = uuid.New()
		}

		_, err := tx.ExecContext(ctx, `
			INSERT INTO tbl_invoice_item (
				id,
				invoice_id,
				name,
				description,
				quantity,
				unit_price,
				discount,
				tax_rate,
				tax_amount,
				total_amount,
				sort_order
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		`,
			invoiceItem.ID,
			invoiceID,
			invoiceItem.Name,
			invoiceItem.Description,
			invoiceItem.Quantity,
			invoiceItem.UnitPrice,
			invoiceItem.Discount,
			invoiceItem.TaxRate,
			invoiceItem.TaxAmount,
			invoiceItem.TotalAmount,
			idx,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Repository) getItemsByInvoiceID(ctx context.Context, invoiceID uuid.UUID) ([]*item.Item, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id,
			name,
			description,
			quantity,
			unit_price,
			discount,
			tax_rate,
			tax_amount,
			total_amount
		FROM tbl_invoice_item
		WHERE invoice_id = $1
		AND deleted_at IS NULL
		ORDER BY sort_order ASC, created_at ASC
	`, invoiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*item.Item, 0)
	for rows.Next() {
		var invoiceItem item.Item
		if err := rows.Scan(
			&invoiceItem.ID,
			&invoiceItem.Name,
			&invoiceItem.Description,
			&invoiceItem.Quantity,
			&invoiceItem.UnitPrice,
			&invoiceItem.Discount,
			&invoiceItem.TaxRate,
			&invoiceItem.TaxAmount,
			&invoiceItem.TotalAmount,
		); err != nil {
			return nil, err
		}
		items = append(items, &invoiceItem)
	}

	return items, rows.Err()
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
