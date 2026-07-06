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

type IRepository interface {
	Create(ctx context.Context, tx *sqlx.Tx, invoiceID uuid.UUID, sections []Section) error
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
func (r *Repository) Create(ctx context.Context, tx *sqlx.Tx, invoiceID uuid.UUID, sections []Section) error {
	if len(sections) == 0 {
		return nil
	}

	for i := range sections {
		if err := r.createSectionRecursive(ctx, tx, invoiceID, &sections[i]); err != nil {
			return err
		}
	}
	return nil
}

// createSectionRecursive creates a section and its nested children recursively
func (r *Repository) createSectionRecursive(ctx context.Context, tx *sqlx.Tx, invoiceID uuid.UUID, section *Section) error {
	if section.ID == uuid.Nil {
		section.ID = uuid.New()
	}
	section.InvoiceID = &invoiceID

	query := `
		INSERT INTO tbl_map_invoice_section (
			id, invoice_id, invoice_section, document_number, tax_method, payment_date, payment_reference, parent_section_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at
	`

	err := tx.QueryRowContext(
		ctx,
		query,
		section.ID,
		section.InvoiceID,
		section.InvoiceSection,
		section.DocumentNumber,
		section.TaxMethod,
		section.PaymentDate,
		section.PaymentReference,
		section.ParentSectionID,
	).Scan(&section.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create section: %w", err)
	}

	// Create entries for this section
	if len(section.Entries) > 0 {
		explicitEntries := make([]*item.Item, len(section.Entries))

		for j, entry := range section.Entries {
			if entry != nil {
				itemCopy := *entry
				itemCopy.InvoiceSectionID = &section.ID
				itemCopy.InvoiceID = invoiceID

				explicitEntries[j] = &itemCopy
			}
		}

		if err := r.itemRepo.Create(ctx, tx, invoiceID, explicitEntries); err != nil {
			return fmt.Errorf("failed to create section entries: %w", err)
		}

		section.Entries = explicitEntries
	}

	// Recursively create nested child sections
	if len(section.Sections) > 0 {
		for j := range section.Sections {
			if section.Sections[j] != nil {
				section.Sections[j].ParentSectionID = &section.ID
				if err := r.createSectionRecursive(ctx, tx, invoiceID, section.Sections[j]); err != nil {
					return fmt.Errorf("failed to create nested section: %w", err)
				}
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
	for i := range sections {
		query := `
		UPDATE tbl_map_invoice_section
		SET 
			document_number = $1,
			tax_method = $2,
			payment_date = $3,
			payment_reference = $4,
			updated_at = NOW()
		WHERE id = $5 AND deleted_at IS NULL
		RETURNING updated_at
	`

		err := tx.QueryRowContext(
			ctx,
			query,
			sections[i].DocumentNumber,
			sections[i].TaxMethod,
			sections[i].PaymentDate,
			sections[i].PaymentReference,
			sections[i].ID,
		).Scan(&sections[i].UpdatedAt)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrSectionNotFound
			}
			return fmt.Errorf("failed to update section: %w", err)
		}

		if sections[i].Entries != nil {
			_, err := tx.ExecContext(ctx, `
			UPDATE tbl_invoice_item
			SET deleted_at = NOW(), updated_at = NOW()
			WHERE invoice_section_id = $1 AND deleted_at IS NULL
		`, sections[i].ID)
			if err != nil {
				return fmt.Errorf("failed to delete old entries: %w", err)
			}

			if len(sections[i].Entries) > 0 {
				var invID uuid.UUID
				if sections[i].InvoiceID != nil {
					invID = *sections[i].InvoiceID
				}

				for j := range sections[i].Entries {
					if sections[i].Entries[j] != nil {
						sections[i].Entries[j].InvoiceSectionID = &sections[i].ID
						sections[i].Entries[j].InvoiceID = invID
					}
				}
				if err := r.itemRepo.Create(ctx, tx, invID, sections[i].Entries); err != nil {
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
			id, invoice_id, invoice_section, document_number, tax_method, payment_date::text, payment_reference,
			created_at, updated_at
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
		&section.PaymentDate,
		&section.PaymentReference,
		&section.CreatedAt,
		&section.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSectionNotFound
		}
		return nil, fmt.Errorf("failed to get section: %w", err)
	}

	items, err := r.itemRepo.GetByInvoiceID(ctx, r.db, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to load section items: %w", err)
	}

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
			id, invoice_id, invoice_section, document_number, tax_method, payment_date::text, payment_reference,
			parent_section_id, created_at, updated_at
		FROM tbl_map_invoice_section
		WHERE invoice_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list sections: %w", err)
	}
	defer rows.Close()

	allSections := make(map[uuid.UUID]*Section)

	for rows.Next() {
		section := &Section{}
		err := rows.Scan(
			&section.ID,
			&section.InvoiceID,
			&section.InvoiceSection,
			&section.DocumentNumber,
			&section.TaxMethod,
			&section.PaymentDate,
			&section.PaymentReference,
			&section.ParentSectionID,
			&section.CreatedAt,
			&section.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan section: %w", err)
		}

		section.Entries = make([]*item.Item, 0)
		section.Sections = make([]*Section, 0)
		allSections[section.ID] = section
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Build hierarchy: attach child sections to their parents
	for _, section := range allSections {
		if section.ParentSectionID != nil {
			if parent, ok := allSections[*section.ParentSectionID]; ok {
				parent.Sections = append(parent.Sections, section)
			}
		}
	}

	// Load all items for this invoice
	items, err := r.itemRepo.GetByInvoiceID(ctx, r.db, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to load items: %w", err)
	}

	// Build item hierarchy and assign to sections
	itemMap := make(map[uuid.UUID]*item.Item)
	for _, itm := range items {
		itemMap[itm.ID] = itm
	}

	// Attach children to parent items
	for _, itm := range items {
		if itm.ParentID != nil {
			if parent, ok := itemMap[*itm.ParentID]; ok {
				if parent.Children == nil {
					parent.Children = make([]*item.Item, 0)
				}
				parent.Children = append(parent.Children, itm)
			}
		}
	}

	// Map items directly to their section structures
	for _, itm := range items {
		if itm.InvoiceSectionID != nil && itm.ParentID == nil {
			if section, ok := allSections[*itm.InvoiceSectionID]; ok {
				section.Entries = append(section.Entries, itm)
			}
		}
	}

	// Build response using the fully updated pointer map references
	var topLevelSections []Section
	for _, section := range allSections {
		if section.ParentSectionID == nil {
			topLevelSections = append(topLevelSections, *section)
		}
	}

	return topLevelSections, nil
}

// GetByType retrieves a section by invoice ID and section type
func (r *Repository) GetByType(ctx context.Context, invoiceID uuid.UUID, sectionType SectionType) (*Section, error) {
	if invoiceID == uuid.Nil {
		return nil, ErrInvalidSectionData
	}

	query := `
		SELECT 
			id, invoice_id, invoice_section, document_number, tax_method, payment_date::text, payment_reference,
			created_at, updated_at
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
		&section.PaymentDate,
		&section.PaymentReference,
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
				return fmt.Errorf("failed to create new section: field 'section_type' cannot be empty for new records")
			}

			if err := r.Create(ctx, tx, invoiceID, []Section{section}); err != nil {
				return fmt.Errorf("failed to create new section: %w", err)
			}
		} else {
			var dbSection Section
			err := tx.QueryRowxContext(ctx, `
				SELECT id, invoice_section, invoice_id FROM tbl_map_invoice_section WHERE id = $1 AND deleted_at IS NULL
			`, section.ID).StructScan(&dbSection)

			if err == nil {
				if section.InvoiceSection == "" {
					section.InvoiceSection = dbSection.InvoiceSection
				}
				if section.InvoiceID == nil {
					section.InvoiceID = dbSection.InvoiceID
				}

				if err := r.updateSection(ctx, tx, section); err != nil {
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
					return fmt.Errorf("failed to create section with ID %s: 'section_type' is a required enum value and cannot be blank", section.ID)
				}

				if err := r.Create(ctx, tx, invoiceID, []Section{section}); err != nil {
					return fmt.Errorf("failed to create section with ID %s: %w", section.ID, err)
				}
			} else {
				return fmt.Errorf("failed to check section existence structural mappings: %w", err)
			}
		}
	}

	return nil
}

// updateSection updates only the section metadata without touching items
func (r *Repository) updateSection(ctx context.Context, tx *sqlx.Tx, section Section) error {
	query := `
		UPDATE tbl_map_invoice_section
		SET 
			document_number = $1,
			tax_method = $2,
			invoice_section = $3,
			payment_date = $4,
			payment_reference = $5,
			parent_section_id = $6,
			updated_at = NOW()
		WHERE id = $7 AND deleted_at IS NULL
	`

	_, err := tx.ExecContext(
		ctx,
		query,
		section.DocumentNumber,
		section.TaxMethod,
		section.InvoiceSection,
		section.PaymentDate,
		section.PaymentReference,
		section.ParentSectionID,
		section.ID,
	)

	return err
}
