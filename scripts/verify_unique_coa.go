package main

import (
	"log"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/db"
	"github.com/iamarpitzala/acareca/pkg/config"
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

	// Get all forms and check for duplicate COA IDs within each form
	type FormField struct {
		FormID      uuid.UUID  `db:"form_id"`
		FormName    string     `db:"form_name"`
		FieldKey    string     `db:"field_key"`
		FieldLabel  string     `db:"field_label"`
		CoaID       *uuid.UUID `db:"coa_id"`
		IsComputed  bool       `db:"is_computed"`
	}

	var fields []FormField
	query := `
		SELECT 
			f.id as form_id,
			f.name as form_name,
			ff.field_key,
			ff.label as field_label,
			ff.coa_id,
			ff.is_computed
		FROM tbl_form f
		JOIN tbl_custom_form_version fv ON f.id = fv.form_id AND fv.is_active = true
		JOIN tbl_form_field ff ON fv.id = ff.form_version_id
		ORDER BY f.id, ff.sort_order
	`

	err = db.Select(&fields, query)
	if err != nil {
		log.Fatalf("Failed to query forms: %v", err)
	}

	if len(fields) == 0 {
		log.Println("No forms found in database")
		return
	}

	// Group by form and check for duplicates
	formMap := make(map[uuid.UUID][]FormField)
	for _, field := range fields {
		formMap[field.FormID] = append(formMap[field.FormID], field)
	}

	log.Printf("📊 Checking %d forms for unique COA IDs per form field...\n", len(formMap))
	log.Println("================================================================")

	totalForms := 0
	formsWithDuplicates := 0
	formsChecked := 0

	for formID, formFields := range formMap {
		totalForms++
		
		// Only check first 5 forms for brevity
		if formsChecked >= 5 {
			continue
		}
		formsChecked++

		log.Printf("\n📋 Form: %s", formFields[0].FormName)
		log.Printf("   Form ID: %s", formID)
		log.Println("   Fields:")

		// Track COA IDs used in this form
		coaIDMap := make(map[uuid.UUID][]string)
		hasDuplicate := false

		for _, field := range formFields {
			if field.IsComputed {
				log.Printf("      [%s] %s (Computed - No COA)", field.FieldKey, field.FieldLabel)
			} else if field.CoaID != nil {
				coaIDMap[*field.CoaID] = append(coaIDMap[*field.CoaID], field.FieldKey)
				log.Printf("      [%s] %s → COA: %s", field.FieldKey, field.FieldLabel, (*field.CoaID).String()[:8]+"...")
			} else {
				log.Printf("      [%s] %s → COA: NULL (ERROR!)", field.FieldKey, field.FieldLabel)
			}
		}

		// Check for duplicates
		for coaID, fieldKeys := range coaIDMap {
			if len(fieldKeys) > 1 {
				hasDuplicate = true
				log.Printf("   ⚠️  DUPLICATE COA ID %s used by fields: %v", coaID.String()[:8]+"...", fieldKeys)
			}
		}

		if hasDuplicate {
			formsWithDuplicates++
			log.Println("   ❌ This form has duplicate COA IDs!")
		} else {
			log.Println("   ✅ All COA IDs are unique in this form")
		}
	}

	log.Println("\n================================================================")
	log.Printf("📈 Summary:")
	log.Printf("   Total forms in database: %d", totalForms)
	log.Printf("   Forms checked (sample): %d", formsChecked)
	log.Printf("   Forms with duplicate COA IDs: %d", formsWithDuplicates)
	
	if formsWithDuplicates > 0 {
		log.Println("   ❌ ISSUE FOUND: Some forms have duplicate COA IDs")
	} else {
		log.Println("   ✅ SUCCESS: All checked forms have unique COA IDs per field")
	}
	log.Println("================================================================")
}
