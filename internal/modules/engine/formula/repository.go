package formula

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type IRepository interface {
	CreateTx(ctx context.Context, tx *sqlx.Tx, f *Formula) error
	CreateNodeTx(ctx context.Context, tx *sqlx.Tx, n *FormulaNode) error
	DeleteByFormVersionIDTx(ctx context.Context, tx *sqlx.Tx, formVersionID uuid.UUID) error
	ListByFormVersionID(ctx context.Context, formVersionID uuid.UUID) ([]*Formula, error)
	ListNodesWithKeyByFormulaID(ctx context.Context, formulaID uuid.UUID) ([]*FormulaNodeWithKey, error)
	GetFieldKeyByFieldID(ctx context.Context, fieldID uuid.UUID) (string, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) IRepository {
	return &repository{db: db}
}

func (r *repository) CreateTx(ctx context.Context, tx *sqlx.Tx, f *Formula) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO tbl_formula (id, form_version_id, field_id, name)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at`,
		f.ID, f.FormVersionID, f.FieldID, f.Name,
	)
	if err != nil {
		return fmt.Errorf("create formula: %w", err)
	}
	return nil
}

func (r *repository) CreateNodeTx(ctx context.Context, tx *sqlx.Tx, n *FormulaNode) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, operator, field_id, constant_value, position)
		VALUES ($1, $2, $3, $4::formula_node_type, $5, $6, $7, $8)`,
		n.ID, n.FormulaID, n.ParentID, n.NodeType, n.Operator, n.FieldID, n.ConstantValue, n.Position,
	)
	if err != nil {
		return fmt.Errorf("create formula node: %w", err)
	}
	return nil
}

func (r *repository) DeleteByFormVersionIDTx(ctx context.Context, tx *sqlx.Tx, formVersionID uuid.UUID) error {
	_, err := tx.ExecContext(ctx,
		`DELETE FROM tbl_formula WHERE form_version_id = $1`, formVersionID,
	)
	if err != nil {
		return fmt.Errorf("delete formulas by version: %w", err)
	}
	return nil
}

func (r *repository) ListByFormVersionID(ctx context.Context, formVersionID uuid.UUID) ([]*Formula, error) {
	var rows []*Formula
	err := r.db.SelectContext(ctx, &rows, `
		SELECT f.id, f.form_version_id, f.field_id, COALESCE(ff.field_key, '') AS field_key, f.name, f.created_at
		FROM tbl_formula f
		LEFT JOIN tbl_form_field ff ON ff.id = f.field_id AND ff.deleted_at IS NULL
		WHERE f.form_version_id = $1
		ORDER BY f.created_at ASC`, formVersionID,
	)
	if err != nil {
		return nil, fmt.Errorf("list formulas: %w", err)
	}
	return rows, nil
}

func (r *repository) ListNodesWithKeyByFormulaID(ctx context.Context, formulaID uuid.UUID) ([]*FormulaNodeWithKey, error) {
	query := `
		SELECT
			n.id, n.formula_id, n.parent_id, n.node_type, n.operator,
			n.field_id, n.constant_value, n.position, n.created_at,
			ff.field_key
		FROM tbl_formula_node n
		LEFT JOIN tbl_form_field ff ON ff.id = n.field_id AND ff.deleted_at IS NULL
		WHERE n.formula_id = $1
	`
	var rows []*FormulaNodeWithKey
	if err := r.db.SelectContext(ctx, &rows, query, formulaID); err != nil {
		return nil, fmt.Errorf("list formula nodes with key: %w", err)
	}
	return rows, nil
}

func (r *repository) GetFieldKeyByFieldID(ctx context.Context, fieldID uuid.UUID) (string, error) {
	var key string
	err := r.db.QueryRowContext(ctx,
		`SELECT field_key FROM tbl_form_field WHERE id = $1 AND deleted_at IS NULL`, fieldID,
	).Scan(&key)
	if err != nil {
		return "", fmt.Errorf("get field key: %w", err)
	}
	return key, nil
}
