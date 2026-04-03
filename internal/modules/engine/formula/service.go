package formula

import (
	"context"
	"fmt"
	"maps"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type IService interface {
	SyncTx(ctx context.Context, tx *sqlx.Tx, formVersionID uuid.UUID, formulas []RqFormula, keyToFieldID map[string]uuid.UUID) error
	ListByFormVersionID(ctx context.Context, formVersionID uuid.UUID) ([]RsFormula, error)
	EvalFormulas(ctx context.Context, formVersionID uuid.UUID, keyValues map[string]float64, taxTypeByKey map[string]string) (map[uuid.UUID]float64, error)
}

type service struct {
	repo IRepository
}

func NewService(repo IRepository) IService {
	return &service{repo: repo}
}

func (s *service) SyncTx(ctx context.Context, tx *sqlx.Tx, formVersionID uuid.UUID, formulas []RqFormula, keyToFieldID map[string]uuid.UUID) error {
	if err := s.repo.DeleteByFormVersionIDTx(ctx, tx, formVersionID); err != nil {
		return err
	}

	for _, rq := range formulas {
		if err := rq.Validate(); err != nil {
			return fmt.Errorf("formula %q: %w", rq.FieldKey, err)
		}

		fieldID, ok := keyToFieldID[rq.FieldKey]
		if !ok {
			return fmt.Errorf("formula references unknown field key %q", rq.FieldKey)
		}

		f := &Formula{
			ID:            uuid.New(),
			FormVersionID: formVersionID,
			FieldID:       fieldID,
			Name:          rq.Name,
		}
		if err := s.repo.CreateTx(ctx, tx, f); err != nil {
			return err
		}

		if err := insertNodes(ctx, tx, s.repo, f.ID, rq.Expression, nil, nil, keyToFieldID); err != nil {
			return fmt.Errorf("formula %q nodes: %w", rq.FieldKey, err)
		}
	}
	return nil
}

func (s *service) ListByFormVersionID(ctx context.Context, formVersionID uuid.UUID) ([]RsFormula, error) {
	formulas, err := s.repo.ListByFormVersionID(ctx, formVersionID)
	if err != nil {
		return nil, err
	}

	type formulaWithNodes struct {
		formula *Formula
		nodes   []*FormulaNodeWithKey
	}
	all := make([]formulaWithNodes, 0, len(formulas))
	for _, f := range formulas {
		nodes, err := s.repo.ListNodesWithKeyByFormulaID(ctx, f.ID)
		if err != nil {
			return nil, err
		}
		all = append(all, formulaWithNodes{f, nodes})
	}

	// Build index: fieldID -> slice index for topo sort
	fieldIDToIdx := make(map[uuid.UUID]int, len(all))
	for i, fw := range all {
		fieldIDToIdx[fw.formula.FieldID] = i
	}

	// Build adjacency list: deps[i] = indices that i depends on
	n := len(all)
	deps := make([][]int, n)
	for i, fw := range all {
		for _, node := range fw.nodes {
			if node.NodeType == "FIELD" && node.FieldID != nil {
				if j, ok := fieldIDToIdx[*node.FieldID]; ok && j != i {
					deps[i] = append(deps[i], j)
				}
			}
		}
	}

	sorted := topoSort(n, deps)
	out := make([]RsFormula, 0, n)
	for _, i := range sorted {
		fw := all[i]
		rs := RsFormula{
			ID:            fw.formula.ID,
			FormVersionID: fw.formula.FormVersionID,
			FieldID:       fw.formula.FieldID,
			FieldKey:      fw.formula.FieldKey,
			Name:          fw.formula.Name,
			CreatedAt:     fw.formula.CreatedAt,
			Expression:    buildExpressionTree(fw.nodes),
		}
		out = append(out, rs)
	}
	return out, nil
}

// topoSort returns indices in dependency-first order using Kahn's algorithm.
func topoSort(n int, deps [][]int) []int {
	// Build reverse adjacency: revAdj[j] = list of nodes that depend on j
	revAdj := make([][]int, n)
	inDegree := make([]int, n)
	for i, d := range deps {
		inDegree[i] = len(d)
		for _, j := range d {
			revAdj[j] = append(revAdj[j], i)
		}
	}

	queue := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if inDegree[i] == 0 {
			queue = append(queue, i)
		}
	}

	order := make([]int, 0, n)
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		order = append(order, curr)
		for _, dependent := range revAdj[curr] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}
	return order
}

func buildExpressionTree(nodes []*FormulaNodeWithKey) *ExprNode {
	if len(nodes) == 0 {
		return nil
	}

	nodeMap := make(map[uuid.UUID]*FormulaNodeWithKey, len(nodes))
	var root *FormulaNodeWithKey
	for _, n := range nodes {
		nodeMap[n.ID] = n
		if n.ParentID == nil {
			root = n
		}
	}
	if root == nil {
		return nil
	}
	return buildExprNode(root, nodeMap)
}

func buildExprNode(node *FormulaNodeWithKey, nodeMap map[uuid.UUID]*FormulaNodeWithKey) *ExprNode {
	expr := &ExprNode{}
	switch node.NodeType {
	case "OPERATOR":
		expr.Type = "operator"
		if node.Operator != nil {
			expr.Op = *node.Operator
		}
		for _, n := range nodeMap {
			if n.ParentID == nil || *n.ParentID != node.ID || n.Position == nil {
				continue
			}
			switch *n.Position {
			case 0:
				expr.Left = buildExprNode(n, nodeMap)
			case 1:
				expr.Right = buildExprNode(n, nodeMap)
			}
		}
	case "FIELD":
		expr.Type = "field"
		if node.FieldKey != nil {
			expr.Key = *node.FieldKey
		}
	case "CONSTANT":
		expr.Type = "constant"
		expr.Value = node.ConstantValue
	case "SECTION":
		expr.Type = "section"
		if node.FieldKey != nil {
			expr.Key = *node.FieldKey
		}
	case "TEXT":
		expr.Type = "text"
	}
	return expr
}

func insertNodes(ctx context.Context, tx *sqlx.Tx, repo IRepository, formulaID uuid.UUID, node *ExprNode, parentID *uuid.UUID, position *int16, keyToFieldID map[string]uuid.UUID) error {
	n := &FormulaNode{
		ID:        uuid.New(),
		FormulaID: formulaID,
		ParentID:  parentID,
		Position:  position,
	}

	switch node.Type {
	case "operator":
		n.NodeType = "OPERATOR"
		n.Operator = &node.Op
	case "field":
		n.NodeType = "FIELD"
		fid, ok := keyToFieldID[node.Key]
		if !ok {
			return fmt.Errorf("node references unknown field key %q", node.Key)
		}
		n.FieldID = &fid
	case "constant":
		n.NodeType = "CONSTANT"
		n.ConstantValue = node.Value
	case "section":
		n.NodeType = "SECTION"
		// Section nodes don't need FieldID, key is stored for lookup
	case "text":
		n.NodeType = "TEXT"
	}

	if err := repo.CreateNodeTx(ctx, tx, n); err != nil {
		return err
	}

	if node.Left != nil {
		pos := int16(0)
		if err := insertNodes(ctx, tx, repo, formulaID, node.Left, &n.ID, &pos, keyToFieldID); err != nil {
			return err
		}
	}
	if node.Right != nil {
		pos := int16(1)
		if err := insertNodes(ctx, tx, repo, formulaID, node.Right, &n.ID, &pos, keyToFieldID); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) EvalFormulas(ctx context.Context, formVersionID uuid.UUID, keyValues map[string]float64, taxTypeByKey map[string]string) (map[uuid.UUID]float64, error) {
	formulas, err := s.repo.ListByFormVersionID(ctx, formVersionID)
	if err != nil {
		return nil, err
	}

	// Build topo order directly from nodes without building expression trees
	type formulaWithNodes struct {
		formula *Formula
		nodes   []*FormulaNodeWithKey
	}
	all := make([]formulaWithNodes, 0, len(formulas))
	for _, f := range formulas {
		nodes, err := s.repo.ListNodesWithKeyByFormulaID(ctx, f.ID)
		if err != nil {
			return nil, err
		}
		all = append(all, formulaWithNodes{f, nodes})
	}

	fieldIDToIdx := make(map[uuid.UUID]int, len(all))
	for i, fw := range all {
		fieldIDToIdx[fw.formula.FieldID] = i
	}

	n := len(all)
	deps := make([][]int, n)
	for i, fw := range all {
		for _, node := range fw.nodes {
			if node.NodeType == "FIELD" && node.FieldID != nil {
				if j, ok := fieldIDToIdx[*node.FieldID]; ok && j != i {
					deps[i] = append(deps[i], j)
				}
			}
		}
	}

	sorted := topoSort(n, deps)

	vals := make(map[string]float64, len(keyValues))
	maps.Copy(vals, keyValues)
	result := make(map[uuid.UUID]float64, n)

	for _, i := range sorted {
		fw := all[i]
		val, err := evalNodes(fw.nodes, vals)
		if err != nil {
			return nil, fmt.Errorf("formula %q: %w", fw.formula.Name, err)
		}
		result[fw.formula.FieldID] = val

		// Feedback value: computed fields with tax type feed GROSS amount back for dependent formulas
		// Non-tax computed fields feed NET amount
		feedbackVal := val
		if taxType, hasTax := taxTypeByKey[fw.formula.FieldKey]; hasTax {
			// Calculate gross amount: for exclusive tax, gross = net * 1.1
			// For inclusive, the formula already returns gross, so use as-is
			switch taxType {
			case "EXCLUSIVE":
				feedbackVal = val * 1.1 // Add 10% GST
			case "INCLUSIVE":
				feedbackVal = val // Already gross
			case "ZERO":
				feedbackVal = val // No GST
			}
		}
		vals[fw.formula.FieldKey] = feedbackVal
	}

	return result, nil
}

func evalNodes(nodes []*FormulaNodeWithKey, vals map[string]float64) (float64, error) {
	byID := make(map[uuid.UUID]*FormulaNodeWithKey, len(nodes))
	var root *FormulaNodeWithKey
	for _, n := range nodes {
		byID[n.ID] = n
		if n.ParentID == nil {
			root = n
		}
	}
	if root == nil {
		return 0, fmt.Errorf("formula has no root node")
	}
	return evalNode(root, byID, vals)
}

func evalNode(n *FormulaNodeWithKey, byID map[uuid.UUID]*FormulaNodeWithKey, vals map[string]float64) (float64, error) {
	switch n.NodeType {
	case "CONSTANT":
		if n.ConstantValue == nil {
			return 0, fmt.Errorf("constant node has nil value")
		}
		return *n.ConstantValue, nil

	case "FIELD":
		if n.FieldKey == nil {
			return 0, fmt.Errorf("field node has nil key")
		}
		v, ok := vals[*n.FieldKey]
		if !ok {
			return 0, fmt.Errorf("field key %q not found in values", *n.FieldKey)
		}
		return v, nil

	case "SECTION":
		// SECTION aggregates all fields with matching section_type
		// Section key format: "SECTION:COLLECTION", "SECTION:COST", "SECTION:OTHER_COST"
		if n.FieldKey == nil {
			return 0, fmt.Errorf("section node has nil key")
		}
		v, ok := vals[*n.FieldKey]
		if !ok {
			return 0, fmt.Errorf("section key %q not found in values", *n.FieldKey)
		}
		return v, nil

	case "TEXT":
		// TEXT fields are non-numeric, return 0
		return 0, nil

	case "OPERATOR":
		if n.Operator == nil {
			return 0, fmt.Errorf("operator node has nil operator")
		}
		var left, right *FormulaNodeWithKey
		for _, node := range byID {
			if node.ParentID == nil || *node.ParentID != n.ID || node.Position == nil {
				continue
			}
			switch *node.Position {
			case 0:
				left = node
			case 1:
				right = node
			}
		}
		if left == nil || right == nil {
			return 0, fmt.Errorf("operator %q missing children", *n.Operator)
		}
		l, err := evalNode(left, byID, vals)
		if err != nil {
			return 0, err
		}
		r, err := evalNode(right, byID, vals)
		if err != nil {
			return 0, err
		}
		switch *n.Operator {
		case "+":
			return l + r, nil
		case "-":
			return l - r, nil
		case "*":
			return l * r, nil
		case "/":
			if r == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			return l / r, nil
		default:
			return 0, fmt.Errorf("unknown operator %q", *n.Operator)
		}
	}
	return 0, fmt.Errorf("unknown node type %q", n.NodeType)
}
