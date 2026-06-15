package item

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type IRepository interface {
	Create(ctx context.Context, tx *sqlx.Tx, invoiceID uuid.UUID, items []*Item) error
	GetByInvoiceID(ctx context.Context, db *sqlx.DB, invoiceID uuid.UUID) ([]*Item, error)
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
func (r *Repository) Create(ctx context.Context, tx *sqlx.Tx, invoiceID uuid.UUID, items []*Item) error {
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
				total_amount,
				sort_order
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		`,
			invoiceItem.ID,
			invoiceID,
			invoiceItem.Name,
			invoiceItem.Description,
			invoiceItem.Quantity,
			invoiceItem.UnitPrice,
			invoiceItem.TotalAmount,
			idx,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetByInvoiceID implements [IRepository].
func (r *Repository) GetByInvoiceID(ctx context.Context, db *sqlx.DB, invoiceID uuid.UUID) ([]*Item, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id,
			name,
			description,
			quantity,
			unit_price,
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

	items := make([]*Item, 0)
	for rows.Next() {
		var invoiceItem Item
		if err := rows.Scan(
			&invoiceItem.ID,
			&invoiceItem.Name,
			&invoiceItem.Description,
			&invoiceItem.Quantity,
			&invoiceItem.UnitPrice,
			&invoiceItem.TotalAmount,
		); err != nil {
			return nil, err
		}
		items = append(items, &invoiceItem)
	}

	return items, rows.Err()
}
