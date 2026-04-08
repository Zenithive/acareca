package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/db"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type SeedConfig struct {
	NumClinics int
	NumForms   int
}

func main() {
	// Command line flags
	numClinics := flag.Int("clinics", 10, "Number of clinics to create")
	numForms := flag.Int("forms", 5, "Number of forms per clinic")
	practitionerIDStr := flag.String("practitioner-id", "", "Specific practitioner UUID (optional)")
	
	flag.Parse()

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}
	cfg := config.NewConfig()
	// Connect to database
	database, err := db.DBConn(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Seed configuration
	seedConfig := SeedConfig{
		NumClinics: *numClinics,
		NumForms:   *numForms,
	}

	// Get or create a practitioner for seeding
	var practitionerID uuid.UUID
	if *practitionerIDStr != "" {
		practitionerID, err = uuid.Parse(*practitionerIDStr)
		if err != nil {
			log.Fatalf("Invalid practitioner ID: %v", err)
		}
		
		// Verify practitioner exists
		var exists bool
		err = database.Get(&exists, "SELECT EXISTS(SELECT 1 FROM tbl_practitioner WHERE id = $1)", practitionerID)
		if err != nil {
			log.Fatalf("Failed to check practitioner: %v", err)
		}
		if !exists {
			log.Fatalf("Practitioner with ID %s does not exist", practitionerID)
		}
		log.Printf("Using specified practitioner ID: %s", practitionerID)
	} else {
		practitionerID, err = getOrCreatePractitioner(database)
		if err != nil {
			log.Fatalf("Failed to get/create practitioner: %v", err)
		}
		log.Printf("Using default practitioner ID: %s", practitionerID)
	}

	log.Printf("Starting seed with %d clinics and %d forms per clinic", seedConfig.NumClinics, seedConfig.NumForms)

	// Seed clinics and forms
	if err := seedClinicsAndForms(database, practitionerID, seedConfig); err != nil {
		log.Fatalf("Failed to seed data: %v", err)
	}

	log.Println("Seeding completed successfully!")
}


func getOrCreatePractitioner(db *sqlx.DB) (uuid.UUID, error) {
	var practitionerID uuid.UUID
	
	// Try to get existing practitioner
	err := db.Get(&practitionerID, "SELECT id FROM tbl_practitioner LIMIT 1")
	if err == nil {
		return practitionerID, nil
	}

	// If no practitioner exists, create one
	if err == sql.ErrNoRows {
		// First create a user
		userID := uuid.New()
		email := gofakeit.Email()
		
		_, err = db.Exec(`
			INSERT INTO tbl_user (id, email, first_name, last_name, role, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, userID, email, gofakeit.FirstName(), gofakeit.LastName(), "PRACTITIONER", true, time.Now(), time.Now())
		
		if err != nil {
			return uuid.Nil, fmt.Errorf("failed to create user: %w", err)
		}

		// Create practitioner
		practitionerID = uuid.New()
		_, err = db.Exec(`
			INSERT INTO tbl_practitioner (id, user_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4)
		`, practitionerID, userID, time.Now(), time.Now())
		
		if err != nil {
			return uuid.Nil, fmt.Errorf("failed to create practitioner: %w", err)
		}

		return practitionerID, nil
	}

	return uuid.Nil, err
}

func seedClinicsAndForms(db *sqlx.DB, practitionerID uuid.UUID, config SeedConfig) error {
	ctx := context.Background()

	for i := 0; i < config.NumClinics; i++ {
		clinicID, err := createClinic(ctx, db, practitionerID)
		if err != nil {
			return fmt.Errorf("failed to create clinic %d: %w", i+1, err)
		}

		log.Printf("Created clinic %d/%d: %s", i+1, config.NumClinics, clinicID)

		// Create forms for this clinic
		for j := 0; j < config.NumForms; j++ {
			formID, err := createForm(ctx, db, clinicID)
			if err != nil {
				return fmt.Errorf("failed to create form %d for clinic %s: %w", j+1, clinicID, err)
			}
			log.Printf("  Created form %d/%d: %s", j+1, config.NumForms, formID)
		}
	}

	return nil
}

func createClinic(ctx context.Context, db *sqlx.DB, practitionerID uuid.UUID) (uuid.UUID, error) {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback()

	clinicID := uuid.New()
	entityID := uuid.New()
	name := gofakeit.Company()
	abn := gofakeit.Numerify("###########") // 11 digit ABN
	description := gofakeit.Sentence(10)
	
	// Insert clinic
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tbl_clinic (id, practitioner_id, entity_id, name, abn, description, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, clinicID, practitionerID, entityID, name, abn, description, true, time.Now(), time.Now())
	
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to insert clinic: %w", err)
	}

	// Insert clinic address
	addressID := uuid.New()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tbl_clinic_address (id, clinic_id, address, city, state, postcode, is_primary, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, addressID, clinicID, gofakeit.Street(), gofakeit.City(), gofakeit.StateAbr(), 
		gofakeit.Numerify("####"), true, time.Now(), time.Now())
	
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to insert clinic address: %w", err)
	}

	// Insert clinic contacts
	contacts := []struct {
		contactType string
		value       string
		label       string
	}{
		{"EMAIL", gofakeit.Email(), "Primary Email"},
		{"PHONE", gofakeit.Phone(), "Main Phone"},
	}

	for idx, contact := range contacts {
		contactID := uuid.New()
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tbl_clinic_contact (id, clinic_id, contact_type, value, label, is_primary, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, contactID, clinicID, contact.contactType, contact.value, contact.label, idx == 0, time.Now(), time.Now())
		
		if err != nil {
			return uuid.Nil, fmt.Errorf("failed to insert clinic contact: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return uuid.Nil, err
	}

	return clinicID, nil
}

func createForm(ctx context.Context, db *sqlx.DB, clinicID uuid.UUID) (uuid.UUID, error) {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback()

	formID := uuid.New()
	formName := fmt.Sprintf("%s Form", gofakeit.JobTitle())
	description := gofakeit.Sentence(8)
	
	// Random method and status
	methods := []string{"INDEPENDENT_CONTRACTOR", "SERVICE_FEE"}
	statuses := []string{"DRAFT", "PUBLISHED"}
	method := methods[gofakeit.Number(0, 1)]
	status := statuses[gofakeit.Number(0, 1)]
	
	ownerShare := gofakeit.Number(30, 70)
	clinicShare := 100 - ownerShare
	superComponent := float64(gofakeit.Number(9, 12))
	
	// Insert form
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tbl_form (id, clinic_id, name, description, status, method, owner_share, clinic_share, super_component, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, formID, clinicID, formName, description, status, method, ownerShare, clinicShare, superComponent, time.Now(), time.Now())
	
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to insert form: %w", err)
	}

	// Get practitioner ID from clinic
	var practitionerID uuid.UUID
	err = tx.GetContext(ctx, &practitionerID, `
		SELECT practitioner_id FROM tbl_clinic WHERE id = $1
	`, clinicID)
	
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to get practitioner ID: %w", err)
	}

	// Create a form version
	versionID := uuid.New()
	versionNumber := 1
	
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tbl_custom_form_version (id, form_id, version, is_active, practitioner_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, versionID, formID, versionNumber, true, practitionerID, time.Now(), time.Now())
	
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to insert form version: %w", err)
	}

	// Create form fields
	fields := []struct {
		key         string
		label       string
		isComputed  bool
		sectionType string
		taxType     string
		sortOrder   int
	}{
		{"A", "Revenue", false, "COLLECTION", "INCLUSIVE", 1},
		{"B", "Cost of Services", false, "COST", "EXCLUSIVE", 2},
		{"C", "Operating Expenses", false, "OTHER_COST", "EXCLUSIVE", 3},
		{"D", "Net Income", true, "", "", 4},
	}

	// Get available COA IDs for this practitioner
	var coaIDs []uuid.UUID
	err = tx.SelectContext(ctx, &coaIDs, `
		SELECT id FROM tbl_chart_of_accounts WHERE practitioner_id = $1 ORDER BY code
	`, practitionerID)
	
	// Calculate how many non-computed fields we have
	nonComputedCount := 0
	for _, field := range fields {
		if !field.isComputed {
			nonComputedCount++
		}
	}
	
	// If no COA exists or not enough, create sufficient COAs
	if err == sql.ErrNoRows || len(coaIDs) == 0 || len(coaIDs) < nonComputedCount {
		coaIDs, err = createBasicCOAs(ctx, tx, practitionerID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("failed to create COA: %w", err)
		}
	} else if err != nil {
		return uuid.Nil, fmt.Errorf("failed to get COA: %w", err)
	}
	
	// Verify we have enough COAs
	if len(coaIDs) < nonComputedCount {
		return uuid.Nil, fmt.Errorf("insufficient COA entries: have %d, need %d", len(coaIDs), nonComputedCount)
	}

	coaIndex := 0
	fieldIDMap := make(map[string]uuid.UUID) // Map field keys to their IDs
	
	for _, field := range fields {
		fieldID := uuid.New()
		fieldIDMap[field.key] = fieldID
		
		var sectionType, taxType, paymentResp *string
		var coaIDPtr *uuid.UUID
		
		if !field.isComputed {
			sectionType = &field.sectionType
			taxType = &field.taxType
			// Use different COA for each field (unique per form)
			if coaIndex < len(coaIDs) {
				coaIDPtr = &coaIDs[coaIndex]
				coaIndex++
			} else {
				// If we run out of COAs, this is an error - each field must have unique COA
				return uuid.Nil, fmt.Errorf("not enough COA entries for all fields (need at least %d)", coaIndex+1)
			}
			pr := "OWNER"
			paymentResp = &pr
		}
		
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tbl_form_field (id, form_version_id, field_key, label, is_computed, is_formula, 
				section_type, payment_responsibility, tax_type, coa_id, sort_order, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`, fieldID, versionID, field.key, field.label, field.isComputed, field.isComputed,
			sectionType, paymentResp, taxType, coaIDPtr, field.sortOrder, time.Now(), time.Now())
		
		if err != nil {
			return uuid.Nil, fmt.Errorf("failed to insert form field: %w", err)
		}
	}

	// Create formulas for computed fields
	// Formula for field D (Net Income) = A + B - C
	if fieldD, ok := fieldIDMap["D"]; ok {
		formulaID := uuid.New()
		
		// Create formula
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tbl_formula (id, form_version_id, field_id, name, created_at)
			VALUES ($1, $2, $3, $4, $5)
		`, formulaID, versionID, fieldD, "Net Income Calculation", time.Now())
		
		if err != nil {
			return uuid.Nil, fmt.Errorf("failed to insert formula: %w", err)
		}

		// Create formula tree: (A + B) - C
		// Root node: - (subtraction)
		rootNodeID := uuid.New()
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, operator, position, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, rootNodeID, formulaID, nil, "OPERATOR", "-", nil, time.Now())
		
		if err != nil {
			return uuid.Nil, fmt.Errorf("failed to insert root formula node: %w", err)
		}

		// Left child: + (addition)
		addNodeID := uuid.New()
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, operator, position, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, addNodeID, formulaID, rootNodeID, "OPERATOR", "+", 0, time.Now())
		
		if err != nil {
			return uuid.Nil, fmt.Errorf("failed to insert add formula node: %w", err)
		}

		// Left-left child: Field A
		if fieldA, ok := fieldIDMap["A"]; ok {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, field_id, position, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, uuid.New(), formulaID, addNodeID, "FIELD", fieldA, 0, time.Now())
			
			if err != nil {
				return uuid.Nil, fmt.Errorf("failed to insert field A node: %w", err)
			}
		}

		// Left-right child: Field B
		if fieldB, ok := fieldIDMap["B"]; ok {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, field_id, position, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, uuid.New(), formulaID, addNodeID, "FIELD", fieldB, 1, time.Now())
			
			if err != nil {
				return uuid.Nil, fmt.Errorf("failed to insert field B node: %w", err)
			}
		}

		// Right child: Field C
		if fieldC, ok := fieldIDMap["C"]; ok {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, field_id, position, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, uuid.New(), formulaID, rootNodeID, "FIELD", fieldC, 1, time.Now())
			
			if err != nil {
				return uuid.Nil, fmt.Errorf("failed to insert field C node: %w", err)
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return uuid.Nil, err
	}

	return formID, nil
}

func createBasicCOAs(ctx context.Context, tx *sqlx.Tx, practitionerID uuid.UUID) ([]uuid.UUID, error) {
	// Create enough COA entries to ensure uniqueness per field (at least 10 unique COAs)
	coaEntries := []struct {
		code          int
		key           string
		name          string
		accountTypeID int
		accountTaxID  int
	}{
		{4000, "revenue", "Revenue", 4, 1},
		{4100, "service_income", "Service Income", 4, 1},
		{4200, "consultation_fees", "Consultation Fees", 4, 1},
		{5000, "cogs", "Cost of Goods Sold", 5, 1},
		{5100, "direct_labor", "Direct Labor", 5, 1},
		{5200, "materials", "Materials", 5, 1},
		{6000, "operating_expenses", "Operating Expenses", 5, 1},
		{6100, "admin_expenses", "Administrative Expenses", 5, 1},
		{6200, "marketing_expenses", "Marketing Expenses", 5, 1},
		{7000, "other_income", "Other Income", 4, 1},
	}

	var coaIDs []uuid.UUID
	
	for _, entry := range coaEntries {
		coaID := uuid.New()
		coaIDs = append(coaIDs, coaID)
		
		_, err := tx.ExecContext(ctx, `
			INSERT INTO tbl_chart_of_accounts (id, practitioner_id, code, key, name, account_type_id, account_tax_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, coaID, practitionerID, entry.code, entry.key, entry.name, entry.accountTypeID, entry.accountTaxID, time.Now(), time.Now())
		
		if err != nil {
			return nil, err
		}
	}
	
	return coaIDs, nil
}
