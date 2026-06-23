package item

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/invoice/formula"
	"github.com/jmoiron/sqlx"
)

type IRepository interface {
	Create(ctx context.Context, tx *sqlx.Tx, invoiceID uuid.UUID, items []*Item) error
	GetByInvoiceID(ctx context.Context, db *sqlx.DB, invoiceID uuid.UUID) ([]*Item, error)
	Update(ctx context.Context, tx *sqlx.Tx, invoiceID uuid.UUID, items []*Item) error
	Delete(ctx context.Context, tx *sqlx.Tx, itemIDs []uuid.UUID) error
	UpsertItems(ctx context.Context, tx *sqlx.Tx, invoiceID uuid.UUID, items []*Item, deleteIDs []uuid.UUID) error
	EvaluateFormulas(ctx context.Context, items []*Item) error
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
	for _, item := range items {
		if item.ID == uuid.Nil {
			item.ID = uuid.New()
		}
		if err := r.persistItem(ctx, tx, item, false, invoiceID); err != nil {
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
			amount,
			sort_order,
			bas_code,
			invoice_section_id,
			entry_type,
			field_key,
			expression
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
		var exprJSON []byte
		if err := rows.Scan(
			&invoiceItem.ID,
			&invoiceItem.Name,
			&invoiceItem.Description,
			&invoiceItem.Amount,
			&invoiceItem.SortOrder,
			&invoiceItem.BASCode,
			&invoiceItem.InvoiceSectionID,
			&invoiceItem.EntryType,
			&invoiceItem.FieldKey,
			&exprJSON,
		); err != nil {
			return nil, err
		}
		invoiceItem.InvoiceID = invoiceID

		if len(exprJSON) > 0 {
			if err := json.Unmarshal(exprJSON, &invoiceItem.Expression); err != nil {
				return nil, fmt.Errorf("unmarshal expression: %w", err)
			}
		}

		items = append(items, &invoiceItem)
	}

	return items, rows.Err()
}

// Update implements [IRepository].
func (r *Repository) Update(ctx context.Context, tx *sqlx.Tx, invoiceID uuid.UUID, items []*Item) error {
	for _, item := range items {
		if item.ID == uuid.Nil {
			item.ID = uuid.New()
		}
		if err := r.persistItem(ctx, tx, item, true, invoiceID); err != nil {
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

	for _, item := range items {
		if item.ID == uuid.Nil {
			item.ID = uuid.New()
		}

		var exists bool
		if item.ID != uuid.Nil {
			err := tx.QueryRowContext(ctx, `
				SELECT EXISTS(SELECT 1 FROM tbl_invoice_item WHERE id = $1 AND deleted_at IS NULL)
			`, item.ID).Scan(&exists)
			if err != nil {
				return err
			}
		}

		if err := r.persistItem(ctx, tx, item, exists, invoiceID); err != nil {
			return err
		}
	}

	return nil
}

// persistItem inserts or updates a single item based on isUpdate flag
func (r *Repository) persistItem(ctx context.Context, tx *sqlx.Tx, item *Item, isUpdate bool, invoiceID uuid.UUID) error {
	var exprJSON []byte
	var err error
	if item.Expression != nil {
		exprJSON, err = json.Marshal(item.Expression)
		if err != nil {
			return fmt.Errorf("marshal expression: %w", err)
		}
	}

	if isUpdate {
		_, err := tx.ExecContext(ctx, `
			UPDATE tbl_invoice_item
			SET name = $2, description = $3, entry_type = $4, bas_code = $5,
				field_key = $6, amount = $7, invoice_section_id = $8,
				sort_order = $9, expression = $10, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`, item.ID, item.Name, item.Description, item.EntryType, item.BASCode,
			item.FieldKey, item.Amount, item.InvoiceSectionID, item.SortOrder, exprJSON)
		return err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO tbl_invoice_item (
			id, invoice_id, name, description, entry_type, bas_code, field_key,
			amount, invoice_section_id, sort_order, expression
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, item.ID, invoiceID, item.Name, item.Description, item.EntryType, item.BASCode,
		item.FieldKey, item.Amount, item.InvoiceSectionID, item.SortOrder, exprJSON)
	return err
}

func (r *Repository) EvaluateFormulas(ctx context.Context, items []*Item) error {
	if len(items) == 0 {
		return nil
	}

	sorted, err := topologicalSort(items)
	if err != nil {
		return fmt.Errorf("dependency resolution failed: %w", err)
	}

	contextValues := make(map[string]float64)

	for _, item := range sorted {
		if item.Expression == nil {
			if item.BASCode != nil {
				contextValues[string(*item.BASCode)] = item.Amount
			}
			if item.FieldKey != nil && *item.FieldKey != "" {
				contextValues[*item.FieldKey] = item.Amount
			}
		}
	}

	formulaCtx := formula.Context{Context: ctx, Values: contextValues}

	for _, item := range sorted {
		if item.Expression != nil {
			// Validate expression is a proper JSON object
			_, ok := item.Expression.(map[string]interface{})
			if !ok {
				fieldKey := ""
				if item.FieldKey != nil {
					fieldKey = *item.FieldKey
				}
				return &FormulaError{
					ItemName:   item.Name,
					FieldKey:   fieldKey,
					Expression: fmt.Sprintf("%v", item.Expression),
					Err:        fmt.Errorf("expression is not a valid JSON object"),
					Context:    contextValues,
				}
			}

			exprJSON, err := json.Marshal(item.Expression)
			if err != nil {
				fieldKey := ""
				if item.FieldKey != nil {
					fieldKey = *item.FieldKey
				}
				return &FormulaError{
					ItemName:   item.Name,
					FieldKey:   fieldKey,
					Expression: fmt.Sprintf("%v", item.Expression),
					Err:        err,
					Context:    contextValues,
				}
			}

			result, err := formula.BuildFormula(formulaCtx, exprJSON)
			if err != nil {
				fieldKey := ""
				if item.FieldKey != nil {
					fieldKey = *item.FieldKey
				}
				return &FormulaError{
					ItemName:   item.Name,
					FieldKey:   fieldKey,
					Expression: string(exprJSON),
					Err:        err,
					Context:    contextValues,
				}
			}

			item.Amount = result
			if item.FieldKey != nil && *item.FieldKey != "" {
				contextValues[*item.FieldKey] = result
			}
			if item.BASCode != nil {
				contextValues[string(*item.BASCode)] = result
			}
		}
	}

	return nil
}

type FormulaError struct {
	ItemName   string
	FieldKey   string
	Expression string
	Err        error
	Context    map[string]float64
}

func (e *FormulaError) Error() string {
	availableKeys := make([]string, 0, len(e.Context))
	for k := range e.Context {
		availableKeys = append(availableKeys, k)
	}
	return fmt.Sprintf(
		"formula evaluation failed for '%s' (fieldKey=%s): %v\navailable context: %v",
		e.ItemName, e.FieldKey, e.Err, availableKeys,
	)
}

func topologicalSort(items []*Item) ([]*Item, error) {
	itemByFieldKey := make(map[string]*Item)
	itemByBASCode := make(map[string]*Item)
	itemsWithoutKeys := make([]*Item, 0)

	for _, item := range items {
		if item.FieldKey != nil && *item.FieldKey != "" {
			itemByFieldKey[*item.FieldKey] = item
		}
		if item.BASCode != nil {
			itemByBASCode[string(*item.BASCode)] = item
		}
		if (item.FieldKey == nil || *item.FieldKey == "") && item.BASCode == nil {
			itemsWithoutKeys = append(itemsWithoutKeys, item)
		}
	}

	allKeysToItems := make(map[string]*Item)
	for k, v := range itemByFieldKey {
		allKeysToItems[k] = v
	}
	for k, v := range itemByBASCode {
		allKeysToItems[k] = v
	}

	graph := make(map[*Item][]*Item)
	inDegree := make(map[*Item]int)

	for _, item := range items {
		inDegree[item] = 0
	}

	for _, item := range items {
		if item.Expression != nil {
			deps := extractDependencies(item.Expression)
			for _, dep := range deps {
				if depItem, exists := allKeysToItems[dep]; exists {
					graph[depItem] = append(graph[depItem], item)
					inDegree[item]++
				}
			}
		}
	}

	queue := make([]*Item, 0)
	for item, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, item)
		}
	}

	sorted := make([]*Item, 0, len(items))

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		sorted = append(sorted, current)

		for _, dependent := range graph[current] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(sorted) != len(items) {
		sortedSet := make(map[*Item]bool, len(sorted))
		for _, s := range sorted {
			sortedSet[s] = true
		}

		unsorted := make([]string, 0)
		for _, item := range items {
			if !sortedSet[item] {
				if item.FieldKey != nil && *item.FieldKey != "" {
					unsorted = append(unsorted, *item.FieldKey)
				} else if item.BASCode != nil {
					unsorted = append(unsorted, string(*item.BASCode))
				} else {
					unsorted = append(unsorted, item.Name)
				}
			}
		}

		if len(unsorted) > 0 {
			return nil, fmt.Errorf("circular dependency detected in fields: %v", unsorted)
		}
		return nil, fmt.Errorf("circular dependency detected")
	}

	return sorted, nil
}

func extractDependencies(expr interface{}) []string {
	deps := make([]string, 0)
	extractDepsRecursive(expr, &deps)
	return deps
}

func extractDepsRecursive(expr interface{}, deps *[]string) {
	exprMap, ok := expr.(map[string]interface{})
	if !ok {
		return
	}

	exprType, _ := exprMap["type"].(string)
	switch exprType {
	case "field":
		if key, ok := exprMap["key"].(string); ok && key != "" {
			*deps = append(*deps, key)
		}
	case "bas_code":
		if key, ok := exprMap["key"].(string); ok && key != "" {
			*deps = append(*deps, key)
		}
	case "operator":
		if left, ok := exprMap["left"]; ok {
			extractDepsRecursive(left, deps)
		}
		if right, ok := exprMap["right"]; ok {
			extractDepsRecursive(right, deps)
		}
	}
}
