package item

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type IRepository interface {
	Create(ctx context.Context, tx *sqlx.Tx, invoiceID uuid.UUID, items []*Item) error
	GetByInvoiceID(ctx context.Context, db *sqlx.DB, invoiceID uuid.UUID) ([]*Item, error)
	Update(ctx context.Context, tx *sqlx.Tx, invoiceID uuid.UUID, items []*Item) error
	Delete(ctx context.Context, tx *sqlx.Tx, itemIDs []uuid.UUID) error
	UpsertItems(ctx context.Context, tx *sqlx.Tx, invoiceID uuid.UUID, items []*Item, deleteIDs []uuid.UUID) error
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
	for _, invoiceItem := range items {
		if invoiceItem.ID == uuid.Nil {
			invoiceItem.ID = uuid.New()
		}

		_, err := tx.ExecContext(ctx, `
			INSERT INTO tbl_invoice_item (
				id,
				name,
				description,
				entry_type,
				bas_code,
				field_key
				amount,
				invoice_section_id,
				sort_order
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		`,
			invoiceItem.ID,
			invoiceItem.Name,
			invoiceItem.Description,
			invoiceItem.EntryType,
			invoiceItem.BASCode,
			invoiceItem.FieldKey,
			invoiceItem.Amount,
			invoiceItem.InvoiceSectionID,
			invoiceItem.SortOrder,
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
			entry_type,
			bas_code,
			field_key
			amount,
			invoice_section_id,
			sort_order
		FROM tbl_invoice_item
		WHERE invoice_section_id IN (
			SELECT id FROM tbl_map_invoice_section WHERE invoice_id = $1 AND deleted_at IS NULL
		)
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
			&invoiceItem.EntryType,
			&invoiceItem.BASCode,
			&invoiceItem.FieldKey,
			&invoiceItem.Amount,
			&invoiceItem.InvoiceSectionID,
			&invoiceItem.SortOrder,
		); err != nil {
			return nil, err
		}
		items = append(items, &invoiceItem)
	}

	return items, rows.Err()
}

// Update implements [IRepository].
func (r *Repository) Update(ctx context.Context, tx *sqlx.Tx, invoiceID uuid.UUID, items []*Item) error {
	for _, invoiceItem := range items {
		if invoiceItem.ID == uuid.Nil {
			invoiceItem.ID = uuid.New()
		}

		_, err := tx.ExecContext(ctx, `
			UPDATE tbl_invoice_item
			SET
				name = $2,
				description = $3,
				entry_type = $4,
				bas_code = $5,
				field_key = $6
				amount = $7,
				invoice_section_id = $8,
				sort_order = $9,
				updated_at = NOW()
			WHERE id = $1
			AND deleted_at IS NULL
		`,
			invoiceItem.ID,
			invoiceItem.Name,
			invoiceItem.Description,
			invoiceItem.EntryType,
			invoiceItem.BASCode,
			invoiceItem.FieldKey,
			invoiceItem.Amount,
			invoiceItem.InvoiceSectionID,
			invoiceItem.SortOrder,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// Delete implements [IRepository].
func (r *Repository) Delete(ctx context.Context, tx *sqlx.Tx, itemIDs []uuid.UUID) error {
	if len(itemIDs) == 0 {
		return nil
	}

	query := `
		UPDATE tbl_invoice_item
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = ANY($1) AND deleted_at IS NULL
	`
	_, err := tx.ExecContext(ctx, query, itemIDs)
	return err
}

// UpsertItems handles create, update, and delete operations in a single transaction
func (r *Repository) UpsertItems(ctx context.Context, tx *sqlx.Tx, invoiceID uuid.UUID, items []*Item, deleteIDs []uuid.UUID) error {
	if len(deleteIDs) > 0 {
		if err := r.Delete(ctx, tx, deleteIDs); err != nil {
			return err
		}
	}

	for _, invoiceItem := range items {
		if invoiceItem.ID == uuid.Nil {
			invoiceItem.ID = uuid.New()
			if err := r.Create(ctx, tx, invoiceID, []*Item{invoiceItem}); err != nil {
				return err
			}
		} else {
			var exists bool
			err := tx.QueryRowContext(ctx, `
				SELECT EXISTS(SELECT 1 FROM tbl_invoice_item WHERE id = $1 AND deleted_at IS NULL)
			`, invoiceItem.ID).Scan(&exists)
			if err != nil {
				return err
			}

			if exists {
				if err := r.Update(ctx, tx, invoiceID, []*Item{invoiceItem}); err != nil {
					return err
				}
			} else {
				if err := r.Create(ctx, tx, invoiceID, []*Item{invoiceItem}); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
