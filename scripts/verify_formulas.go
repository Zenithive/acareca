package main

import (
	"log"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/db"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	cfg := config.NewConfig()

	// Connect to database
	db, err := db.DBConn(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Get all formulas with their fields
	type FormulaInfo struct {
		FormulaID      uuid.UUID `db:"formula_id"`
		FormulaName    string    `db:"formula_name"`
		FormName       string    `db:"form_name"`
		FieldKey       string    `db:"field_key"`
		FieldLabel     string    `db:"field_label"`
		NodeCount      int       `db:"node_count"`
	}

	var formulas []FormulaInfo
	query := `
		SELECT 
			fo.id as formula_id,
			fo.name as formula_name,
			f.name as form_name,
			ff.field_key,
			ff.label as field_label,
			(SELECT COUNT(*) FROM tbl_formula_node WHERE formula_id = fo.id) as node_count
		FROM tbl_formula fo
		JOIN tbl_form_field ff ON fo.field_id = ff.id
		JOIN tbl_custom_form_version fv ON fo.form_version_id = fv.id
		JOIN tbl_form f ON fv.form_id = f.id
		ORDER BY f.name, ff.field_key
	`

	err = db.Select(&formulas, query)
	if err != nil {
		log.Fatalf("Failed to query formulas: %v", err)
	}

	if len(formulas) == 0 {
		log.Println("No formulas found in database")
		return
	}

	log.Printf("📊 Found %d formulas in the database\n", len(formulas))
	log.Println("================================================================")

	for _, formula := range formulas {
		log.Printf("\n📐 Formula: %s", formula.FormulaName)
		log.Printf("   Form: %s", formula.FormName)
		log.Printf("   Field: [%s] %s", formula.FieldKey, formula.FieldLabel)
		log.Printf("   Formula ID: %s", formula.FormulaID)
		log.Printf("   Nodes: %d", formula.NodeCount)
		
		// Get formula tree
		showFormulaTree(db, formula.FormulaID)
	}

	log.Println("\n================================================================")
	log.Printf("✅ Total formulas verified: %d", len(formulas))
}

func showFormulaTree(db *sqlx.DB, formulaID uuid.UUID) {
	type Node struct {
		ID            uuid.UUID  `db:"id"`
		ParentID      *uuid.UUID `db:"parent_id"`
		NodeType      string     `db:"node_type"`
		Operator      *string    `db:"operator"`
		FieldID       *uuid.UUID `db:"field_id"`
		FieldKey      *string    `db:"field_key"`
		ConstantValue *float64   `db:"constant_value"`
		Position      *int       `db:"position"`
	}

	var nodes []Node
	query := `
		SELECT 
			fn.id,
			fn.parent_id,
			fn.node_type,
			fn.operator,
			fn.field_id,
			ff.field_key,
			fn.constant_value,
			fn.position
		FROM tbl_formula_node fn
		LEFT JOIN tbl_form_field ff ON fn.field_id = ff.id
		WHERE fn.formula_id = $1
		ORDER BY fn.parent_id NULLS FIRST, fn.position
	`

	err := db.Select(&nodes, query, formulaID)
	if err != nil {
		log.Printf("   Error getting formula tree: %v", err)
		return
	}

	log.Println("   Formula Tree:")
	for _, node := range nodes {
		indent := "      "
		if node.ParentID != nil {
			indent = "         "
		}
		
		switch node.NodeType {
		case "OPERATOR":
			log.Printf("%s%s (operator)", indent, *node.Operator)
		case "FIELD":
			log.Printf("%sField %s", indent, *node.FieldKey)
		case "CONSTANT":
			log.Printf("%sConstant %.4f", indent, *node.ConstantValue)
		}
	}
}

