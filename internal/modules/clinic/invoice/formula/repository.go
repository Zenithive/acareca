package formula

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type IRepository interface {
	Create(ctx context.Context, tx *sqlx.Tx, formula *Formula) error
	Update(ctx context.Context, tx *sqlx.Tx, formula *Formula) error
	GetById(ctx context.Context, id uuid.UUID) (*Formula, error)
	GetBySectionID(ctx context.Context, invoiceSectionID uuid.UUID) ([]*Formula, error)
	GetByInvoiceID(ctx context.Context, invoiceID uuid.UUID) ([]*Formula, error)
	Delete(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error
	DeleteBySectionID(ctx context.Context, tx *sqlx.Tx, invoiceSectionID uuid.UUID) error
	CopyBySectionID(ctx context.Context, tx *sqlx.Tx, sourceSectionID, destSectionID uuid.UUID, destInvoiceID uuid.UUID) error
}

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) IRepository {
	return &Repository{
		db: db,
	}
}

// Create inserts a formula and all its nodes within the given transaction.
func (r *Repository) Create(ctx context.Context, tx *sqlx.Tx, formula *Formula) error {
	if formula.ID == uuid.Nil {
		formula.ID = uuid.New()
	}

	_, err := tx.ExecContext(ctx, `
		INSERT INTO tbl_invoice_formula (
			id, invoice_id, invoice_section_id,
			field_key, field_type, label, sort_order
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		formula.ID,
		formula.InvoiceID,
		formula.InvoiceSectionID,
		formula.FieldKey,
		formula.FieldType,
		formula.Label,
		formula.SortOrder,
	)
	if err != nil {
		return err
	}

	if err := r.insertNodes(ctx, tx, formula.ID, nil, formula.Nodes); err != nil {
		return err
	}

	return nil
}

// Update replaces the formula metadata and rebuilds all nodes (delete + insert).
func (r *Repository) Update(ctx context.Context, tx *sqlx.Tx, formula *Formula) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE tbl_invoice_formula
		SET
			field_key = $2,
			field_type = $3,
			label = $4,
			sort_order = $5,
			updated_at = NOW()
		WHERE id = $1
	`, formula.ID, formula.FieldKey, formula.FieldType, formula.Label, formula.SortOrder)
	if err != nil {
		return err
	}

	// Delete all existing nodes (will cascade to children via self-referencing FK).
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM tbl_invoice_formula_node
		WHERE invoice_formula_id = $1
	`, formula.ID); err != nil {
		return err
	}

	// Re-insert nodes from the updated formula.
	if err := r.insertNodes(ctx, tx, formula.ID, nil, formula.Nodes); err != nil {
		return err
	}

	return nil
}

// GetById fetches a single formula with all its nodes.
func (r *Repository) GetById(ctx context.Context, id uuid.UUID) (*Formula, error) {
	formula, err := r.scanFormula(ctx, r.db, id)
	if err != nil {
		return nil, err
	}

	nodes, err := r.fetchNodes(ctx, r.db, formula.ID)
	if err != nil {
		return nil, err
	}

	formula.Nodes = nodes
	return formula, nil
}

// GetBySectionID returns all formulas for a given invoice section, ordered by sort_order.
func (r *Repository) GetBySectionID(ctx context.Context, invoiceSectionID uuid.UUID) ([]*Formula, error) {
	formulas, err := r.scanFormulasBySection(ctx, r.db, invoiceSectionID)
	if err != nil {
		return nil, err
	}

	for _, f := range formulas {
		nodes, err := r.fetchNodes(ctx, r.db, f.ID)
		if err != nil {
			return nil, err
		}
		f.Nodes = nodes
	}

	return formulas, nil
}

// GetByInvoiceID returns all formulas for a given invoice, grouped by section.
func (r *Repository) GetByInvoiceID(ctx context.Context, invoiceID uuid.UUID) ([]*Formula, error) {
	formulas, err := r.scanFormulasByInvoice(ctx, r.db, invoiceID)
	if err != nil {
		return nil, err
	}

	for _, f := range formulas {
		nodes, err := r.fetchNodes(ctx, r.db, f.ID)
		if err != nil {
			return nil, err
		}
		f.Nodes = nodes
	}

	return formulas, nil
}

// Delete soft-deletes the formula and its nodes (cascade handled by FK).
func (r *Repository) Delete(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error {
	result, err := tx.ExecContext(ctx, `
		DELETE FROM tbl_invoice_formula_node
		WHERE invoice_formula_id = $1
	`, id)
	if err != nil {
		return err
	}
	_ = result

	result, err = tx.ExecContext(ctx, `
		DELETE FROM tbl_invoice_formula
		WHERE id = $1
	`, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// DeleteBySectionID deletes formulas for a given invoice section.
func (r *Repository) DeleteBySectionID(ctx context.Context, tx *sqlx.Tx, invoiceSectionID uuid.UUID) error {
	// Children deleted via FK cascade; still explicitly delete for clarity.
	_, err := tx.ExecContext(ctx, `
		DELETE FROM tbl_invoice_formula_node
		WHERE invoice_formula_id IN (
			SELECT id FROM tbl_invoice_formula WHERE invoice_section_id = $1
		)
	`, invoiceSectionID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		DELETE FROM tbl_invoice_formula
		WHERE invoice_section_id = $1
	`, invoiceSectionID)
	return err
}

// CopyBySectionID deep-copies all formulas (with new IDs) from sourceSectionID
// to destSectionID, assigning the new destInvoiceID.
func (r *Repository) CopyBySectionID(ctx context.Context, tx *sqlx.Tx, sourceSectionID, destSectionID uuid.UUID, destInvoiceID uuid.UUID) error {
	formulas, err := r.scanFormulasBySection(ctx, tx, sourceSectionID)
	if err != nil {
		return err
	}

	for _, srcFormula := range formulas {
		dstFormula := &Formula{
			ID:               uuid.New(),
			InvoiceID:        destInvoiceID,
			InvoiceSectionID: destSectionID,
			FieldKey:         srcFormula.FieldKey,
			FieldType:        srcFormula.FieldType,
			Label:            srcFormula.Label,
			SortOrder:        srcFormula.SortOrder,
		}

		if err := r.createFormulaRow(ctx, tx, dstFormula); err != nil {
			return err
		}

		// Fetch and deep-copy nodes.
		srcNodes, err := r.fetchNodes(ctx, tx, srcFormula.ID)
		if err != nil {
			return err
		}

		if err := r.copyNodes(ctx, tx, srcNodes, dstFormula.ID, nil); err != nil {
			return err
		}
	}

	return nil
}

// --- internal helpers ---------------------------------------------------------

// insertNodes recursively inserts formula nodes. parentID is nil for root nodes.
func (r *Repository) insertNodes(ctx context.Context, tx *sqlx.Tx, formulaID uuid.UUID, parentID *uuid.UUID, nodes []*FormulaNode) error {
	for _, node := range nodes {
		if node.ID == uuid.Nil {
			node.ID = uuid.New()
		}
		node.InvoiceFormulaID = formulaID
		node.ParentID = parentID

		_, err := tx.ExecContext(ctx, `
			INSERT INTO tbl_invoice_formula_node (
				id, invoice_formula_id, parent_id,
				node_type, operator, referenced_field_key,
				constant_value, position
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`,
			node.ID,
			node.InvoiceFormulaID,
			node.ParentID,
			node.NodeType,
			node.Operator,
			node.ReferencedFieldKey,
			node.ConstantValue,
			node.Position,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// createFormulaRow inserts only the formula metadata (no nodes).
func (r *Repository) createFormulaRow(ctx context.Context, exec sqlx.ExecerContext, formula *Formula) error {
	if formula.ID == uuid.Nil {
		formula.ID = uuid.New()
	}

	_, err := exec.ExecContext(ctx, `
		INSERT INTO tbl_invoice_formula (
			id, invoice_id, invoice_section_id,
			field_key, field_type, label, sort_order
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		formula.ID,
		formula.InvoiceID,
		formula.InvoiceSectionID,
		formula.FieldKey,
		formula.FieldType,
		formula.Label,
		formula.SortOrder,
	)
	return err
}

// copyNodes recursively copies nodes from srcNodes for a new formula ID.
// oldParentID maps the old parent ID to the new parent ID.
func (r *Repository) copyNodes(ctx context.Context, exec sqlx.ExecerContext, srcNodes []*FormulaNode, dstFormulaID uuid.UUID, newParentID *uuid.UUID) error {
	for _, srcNode := range srcNodes {
		dstNode := &FormulaNode{
			ID:                 uuid.New(),
			InvoiceFormulaID:   dstFormulaID,
			ParentID:           newParentID,
			NodeType:           srcNode.NodeType,
			Operator:           srcNode.Operator,
			ReferencedFieldKey: srcNode.ReferencedFieldKey,
			ConstantValue:      srcNode.ConstantValue,
			Position:           srcNode.Position,
		}

		_, err := exec.ExecContext(ctx, `
			INSERT INTO tbl_invoice_formula_node (
				id, invoice_formula_id, parent_id,
				node_type, operator, referenced_field_key,
				constant_value, position
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`,
			dstNode.ID,
			dstNode.InvoiceFormulaID,
			dstNode.ParentID,
			dstNode.NodeType,
			dstNode.Operator,
			dstNode.ReferencedFieldKey,
			dstNode.ConstantValue,
			dstNode.Position,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// scanFormula fetches a single formula row by ID.
func (r *Repository) scanFormula(ctx context.Context, q sqlx.QueryerContext, id uuid.UUID) (*Formula, error) {
	var formula Formula
	err := sqlx.GetContext(ctx, q, &formula, `
		SELECT
			id, invoice_id, invoice_section_id,
			field_key, field_type, label, sort_order,
			created_at, updated_at
		FROM tbl_invoice_formula
		WHERE id = $1
	`, id)
	if err != nil {
		return nil, err
	}

	return &formula, nil
}

// scanFormulasBySection fetches all formulas for a section, ordered by sort_order.
func (r *Repository) scanFormulasBySection(ctx context.Context, q sqlx.QueryerContext, invoiceSectionID uuid.UUID) ([]*Formula, error) {
	formulas := make([]*Formula, 0)
	err := sqlx.SelectContext(ctx, q, &formulas, `
		SELECT
			id, invoice_id, invoice_section_id,
			field_key, field_type, label, sort_order,
			created_at, updated_at
		FROM tbl_invoice_formula
		WHERE invoice_section_id = $1
		ORDER BY sort_order ASC, created_at ASC
	`, invoiceSectionID)
	if err != nil {
		return nil, err
	}

	return formulas, nil
}

// scanFormulasByInvoice fetches all formulas for an invoice, ordered by section and sort_order.
func (r *Repository) scanFormulasByInvoice(ctx context.Context, q sqlx.QueryerContext, invoiceID uuid.UUID) ([]*Formula, error) {
	formulas := make([]*Formula, 0)
	err := sqlx.SelectContext(ctx, q, &formulas, `
		SELECT
			f.id, f.invoice_id, f.invoice_section_id,
			f.field_key, f.field_type, f.label, f.sort_order,
			f.created_at, f.updated_at
		FROM tbl_invoice_formula f
		WHERE f.invoice_id = $1
		ORDER BY f.sort_order ASC, f.created_at ASC
	`, invoiceID)
	if err != nil {
		return nil, err
	}

	return formulas, nil
}

// fetchNodes retrieves all nodes for a formula, ordered by parent_id, position.
func (r *Repository) fetchNodes(ctx context.Context, q sqlx.QueryerContext, formulaID uuid.UUID) ([]*FormulaNode, error) {
	nodes := make([]*FormulaNode, 0)
	err := sqlx.SelectContext(ctx, q, &nodes, `
		SELECT
			id, invoice_formula_id, parent_id,
			node_type, operator, referenced_field_key,
			constant_value, position, created_at
		FROM tbl_invoice_formula_node
		WHERE invoice_formula_id = $1
		ORDER BY parent_id NULLS FIRST, position ASC, created_at ASC
	`, formulaID)
	if err != nil {
		return nil, err
	}

	return nodes, nil
}
