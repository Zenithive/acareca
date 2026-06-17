package section

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/item"
	"github.com/jmoiron/sqlx"
)

var (
	ErrSectionNotFound      = errors.New("section not found")
	ErrInvalidSectionData   = errors.New("invalid section data")
	ErrSectionAlreadyExists = errors.New("section already exists for this invoice")
)

// IRepository defines the contract for section data operations
type IRepository interface {
	Create(ctx context.Context, tx *sqlx.Tx, sections []Section) error
	Update(ctx context.Context, tx *sqlx.Tx, sections []Section) error
	Delete(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error
	GetByID(ctx context.Context, invoiceID, sectionID uuid.UUID) (*Section, error)
	ListByInvoiceID(ctx context.Context, invoiceID uuid.UUID) ([]Section, error)
	GetByType(ctx context.Context, invoiceID uuid.UUID, sectionType SectionType) (*Section, error)
	UpsertSections(ctx context.Context, tx *sqlx.Tx, invoiceID uuid.UUID, sections []Section, deleteSectionIDs []uuid.UUID, deleteItemIDs map[uuid.UUID][]uuid.UUID) error
}

type Repository struct {
	db       *sqlx.DB
	itemRepo item.IRepository
}

// NewRepository creates a new section repository instance
func NewRepository(db *sqlx.DB, itemRepo item.IRepository) IRepository {
	return &Repository{
		db:       db,
		itemRepo: itemRepo,
	}
}

// Create inserts a new section into the database
func (r *Repository) Create(ctx context.Context, tx *sqlx.Tx, sections []Section) error {
	if len(sections) == 0 {
		return nil
	}

	for i := range sections {
		if sections[i].ID == uuid.Nil {
			sections[i].ID = uuid.New()
		}

		query := `
		INSERT INTO tbl_map_invoice_section (
			id, invoice_id, invoice_section, document_number, tax_method
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at
	`

		err := tx.QueryRowContext(
			ctx,
			query,
			sections[i].ID,
			sections[i].InvoiceID,
			sections[i].InvoiceSection,
			sections[i].DocumentNumber,
			sections[i].TaxMethod,
		).Scan(&sections[i].CreatedAt)

		if err != nil {
			return fmt.Errorf("failed to create section: %w", err)
		}

		// Link all entries to this section
		if len(sections[i].Entries) > 0 {
			for _, entry := range sections[i].Entries {
				entry.InvoiceSectionID = &sections[i].ID
			}
			if err := r.itemRepo.Create(ctx, tx, *sections[i].InvoiceID, sections[i].Entries); err != nil {
				return fmt.Errorf("failed to create section entries: %w", err)
			}
		}
	}
	return nil
}

// Update modifies an existing section
func (r *Repository) Update(ctx context.Context, tx *sqlx.Tx, sections []Section) error {
	if len(sections) == 0 {
		return ErrInvalidSectionData
	}
	for _, section := range sections {
		query := `
		UPDATE tbl_map_invoice_section
		SET 
			document_number = $1,
			tax_method = $2,
			updated_at = NOW()
		WHERE id = $3 AND deleted_at IS NULL
		RETURNING updated_at
	`

		err := tx.QueryRowContext(
			ctx,
			query,
			section.DocumentNumber,
			section.TaxMethod,
			section.ID,
		).Scan(&section.UpdatedAt)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrSectionNotFound
			}
			return fmt.Errorf("failed to update section: %w", err)
		}

		// Update section entries if provided
		if section.Entries != nil {
			// Delete existing entries
			_, err := tx.ExecContext(ctx, `
			UPDATE tbl_invoice_item
			SET deleted_at = NOW(), updated_at = NOW()
			WHERE invoice_section_id = $1 AND deleted_at IS NULL
		`, section.ID)
			if err != nil {
				return fmt.Errorf("failed to delete old entries: %w", err)
			}

			// Create new entries
			if len(section.Entries) > 0 {
				for _, entry := range section.Entries {
					entry.InvoiceSectionID = &section.ID
				}
				if err := r.itemRepo.Create(ctx, tx, *section.InvoiceID, section.Entries); err != nil {
					return fmt.Errorf("failed to update section entries: %w", err)
				}
			}
		}
	}
	return nil
}

// Delete soft-deletes a section and its associated items
func (r *Repository) Delete(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error {
	if id == uuid.Nil {
		return ErrInvalidSectionData
	}

	// Soft delete the section
	result, err := tx.ExecContext(ctx, `
		UPDATE tbl_map_invoice_section 
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return fmt.Errorf("failed to delete section: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrSectionNotFound
	}

	// Soft delete associated items
	_, err = tx.ExecContext(ctx, `
		UPDATE tbl_invoice_item
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE invoice_section_id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return fmt.Errorf("failed to delete section items: %w", err)
	}

	return nil
}

// GetByID retrieves a single section with its items
func (r *Repository) GetByID(ctx context.Context, invoiceID, sectionID uuid.UUID) (*Section, error) {
	if invoiceID == uuid.Nil || sectionID == uuid.Nil {
		return nil, ErrInvalidSectionData
	}

	query := `
		SELECT 
			id, invoice_id, invoice_section, document_number, 
			tax_method, created_at, updated_at
		FROM tbl_map_invoice_section
		WHERE invoice_id = $1 AND id = $2 AND deleted_at IS NULL
	`

	var section Section
	err := r.db.QueryRowContext(ctx, query, invoiceID, sectionID).Scan(
		&section.ID,
		&section.InvoiceID,
		&section.InvoiceSection,
		&section.DocumentNumber,
		&section.TaxMethod,
		&section.CreatedAt,
		&section.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSectionNotFound
		}
		return nil, fmt.Errorf("failed to get section: %w", err)
	}

	// Load section items
	items, err := r.itemRepo.GetByInvoiceID(ctx, r.db, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to load section items: %w", err)
	}

	// Filter items for this section
	section.Entries = make([]*item.Item, 0)
	for _, itm := range items {
		if itm.InvoiceSectionID != nil && *itm.InvoiceSectionID == sectionID {
			section.Entries = append(section.Entries, itm)
		}
	}

	return &section, nil
}

// ListByInvoiceID retrieves all sections for a given invoice
func (r *Repository) ListByInvoiceID(ctx context.Context, invoiceID uuid.UUID) ([]Section, error) {
	if invoiceID == uuid.Nil {
		return nil, ErrInvalidSectionData
	}

	query := `
		SELECT 
			id, invoice_id, invoice_section, document_number,
			tax_method, created_at, updated_at
		FROM tbl_map_invoice_section
		WHERE invoice_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list sections: %w", err)
	}
	defer rows.Close()

	sections := make([]Section, 0)
	sectionIDMap := make(map[uuid.UUID]int)

	for rows.Next() {
		var section Section
		err := rows.Scan(
			&section.ID,
			&section.InvoiceID,
			&section.InvoiceSection,
			&section.DocumentNumber,
			&section.TaxMethod,
			&section.CreatedAt,
			&section.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan section: %w", err)
		}

		section.Entries = make([]*item.Item, 0)
		sectionIDMap[section.ID] = len(sections)
		sections = append(sections, section)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load all items for this invoice
	items, err := r.itemRepo.GetByInvoiceID(ctx, r.db, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to load items: %w", err)
	}

	// Group items by section
	for _, itm := range items {
		if itm.InvoiceSectionID != nil {
			if idx, ok := sectionIDMap[*itm.InvoiceSectionID]; ok {
				sections[idx].Entries = append(sections[idx].Entries, itm)
			}
		}
	}

	return sections, nil
}

// GetByType retrieves a section by invoice ID and section type
func (r *Repository) GetByType(ctx context.Context, invoiceID uuid.UUID, sectionType SectionType) (*Section, error) {
	if invoiceID == uuid.Nil {
		return nil, ErrInvalidSectionData
	}

	query := `
		SELECT 
			id, invoice_id, invoice_section, document_number,
			tax_method, created_at, updated_at
		FROM tbl_map_invoice_section
		WHERE invoice_id = $1 AND invoice_section = $2 AND deleted_at IS NULL
		LIMIT 1
	`

	var section Section
	err := r.db.QueryRowContext(ctx, query, invoiceID, sectionType).Scan(
		&section.ID,
		&section.InvoiceID,
		&section.InvoiceSection,
		&section.DocumentNumber,
		&section.TaxMethod,
		&section.CreatedAt,
		&section.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSectionNotFound
		}
		return nil, fmt.Errorf("failed to get section by type: %w", err)
	}

	// Load section items
	items, err := r.itemRepo.GetByInvoiceID(ctx, r.db, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to load section items: %w", err)
	}

	// Filter items for this section
	section.Entries = make([]*item.Item, 0)
	for _, itm := range items {
		if itm.InvoiceSectionID != nil && *itm.InvoiceSectionID == section.ID {
			section.Entries = append(section.Entries, itm)
		}
	}

	return &section, nil
}

// UpsertSections handles create, update, and delete operations for sections and their items
func (r *Repository) UpsertSections(ctx context.Context, tx *sqlx.Tx, invoiceID uuid.UUID, sections []Section, deleteSectionIDs []uuid.UUID, deleteItemIDs map[uuid.UUID][]uuid.UUID) error {
	if len(deleteSectionIDs) > 0 {
		for _, sectionID := range deleteSectionIDs {
			if err := r.Delete(ctx, tx, sectionID); err != nil {
				return fmt.Errorf("failed to delete section %s: %w", sectionID, err)
			}
		}
	}

	for _, section := range sections {
		if section.ID == uuid.Nil {
			section.ID = uuid.New()
			if section.InvoiceID == nil {
				section.InvoiceID = &invoiceID
			}
			if section.InvoiceSection == "" {
				return fmt.Errorf("invoice_section is required for new section")
			}
			if err := r.Create(ctx, tx, []Section{section}); err != nil {
				return fmt.Errorf("failed to create new section: %w", err)
			}
		} else {
			var existing Section
			err := tx.QueryRowContext(ctx, `
				SELECT id, invoice_id, invoice_section, document_number, tax_method
				FROM tbl_map_invoice_section
				WHERE id = $1 AND deleted_at IS NULL
			`, section.ID).Scan(
				&existing.ID,
				&existing.InvoiceID,
				&existing.InvoiceSection,
				&existing.DocumentNumber,
				&existing.TaxMethod,
			)

			if err == nil {
				if section.InvoiceSection == "" {
					section.InvoiceSection = existing.InvoiceSection
				}
				if section.DocumentNumber == "" {
					section.DocumentNumber = existing.DocumentNumber
				}
				if section.TaxMethod == nil {
					section.TaxMethod = existing.TaxMethod
				}

				if err := r.updateSectionOnly(ctx, tx, section); err != nil {
					return fmt.Errorf("failed to update section %s: %w", section.ID, err)
				}

				if itemsToDelete, ok := deleteItemIDs[section.ID]; ok && len(itemsToDelete) > 0 {
					if err := r.itemRepo.Delete(ctx, tx, itemsToDelete); err != nil {
						return fmt.Errorf("failed to delete items for section %s: %w", section.ID, err)
					}
				}

				if section.Entries != nil {
					for _, entry := range section.Entries {
						entry.InvoiceSectionID = &section.ID
					}
					if err := r.itemRepo.UpsertItems(ctx, tx, invoiceID, section.Entries, nil); err != nil {
						return fmt.Errorf("failed to upsert items for section %s: %w", section.ID, err)
					}
				}
			} else if errors.Is(err, sql.ErrNoRows) {
				if section.InvoiceID == nil {
					section.InvoiceID = &invoiceID
				}
				if section.InvoiceSection == "" {
					return fmt.Errorf("invoice_section is required for new section %s", section.ID)
				}
				if err := r.Create(ctx, tx, []Section{section}); err != nil {
					return fmt.Errorf("failed to create section with ID %s: %w", section.ID, err)
				}
			} else {
				return fmt.Errorf("failed to load section %s: %w", section.ID, err)
			}
		}
	}

	return nil
}

// updateSectionOnly updates only the section metadata without touching items
func (r *Repository) updateSectionOnly(ctx context.Context, tx *sqlx.Tx, section Section) error {
	query := `
		UPDATE tbl_map_invoice_section
		SET 
			document_number = $1,
			tax_method = $2,
			invoice_section = $3,
			updated_at = NOW()
		WHERE id = $4 AND deleted_at IS NULL
	`

	_, err := tx.ExecContext(
		ctx,
		query,
		section.DocumentNumber,
		section.TaxMethod,
		section.InvoiceSection,
		section.ID,
	)

	return err
}
