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

type AdvancedSeedConfig struct {
	NumClinics       int
	NumFormsPerClinic int
	NumFieldsPerForm int
	CreatePractitioner bool
	PractitionerEmail string
	Verbose          bool
}

func main() {
	// Command line flags
	numClinics := flag.Int("clinics", 10, "Number of clinics to create")
	numForms := flag.Int("forms", 5, "Number of forms per clinic")
	numFields := flag.Int("fields", 6, "Number of fields per form")
	createPractitioner := flag.Bool("create-practitioner", false, "Create a new practitioner instead of using existing")
	practitionerEmail := flag.String("practitioner-email", "", "Email for new practitioner (if creating)")
	practitionerIDStr := flag.String("practitioner-id", "", "Specific practitioner UUID (optional)")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	
	flag.Parse()

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

	// Seed configuration
	config := AdvancedSeedConfig{
		NumClinics:        *numClinics,
		NumFormsPerClinic: *numForms,
		NumFieldsPerForm:  *numFields,
		CreatePractitioner: *createPractitioner,
		PractitionerEmail: *practitionerEmail,
		Verbose:           *verbose,
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
		err = db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM tbl_practitioner WHERE id = $1)", practitionerID)
		if err != nil {
			log.Fatalf("Failed to check practitioner: %v", err)
		}
		if !exists {
			log.Fatalf("Practitioner with ID %s does not exist", practitionerID)
		}
		log.Printf("Using specified practitioner ID: %s", practitionerID)
	} else {
		practitionerID, err = getOrCreatePractitionerAdvanced(db, config)
		if err != nil {
			log.Fatalf("Failed to get/create practitioner: %v", err)
		}
	}

	log.Printf("Starting seed with practitioner ID: %s", practitionerID)
	log.Printf("Configuration: %d clinics, %d forms per clinic, %d fields per form", 
		config.NumClinics, config.NumFormsPerClinic, config.NumFieldsPerForm)

	// Seed clinics and forms
	if err := seedClinicsAndFormsAdvanced(db, practitionerID, config); err != nil {
		log.Fatalf("Failed to seed data: %v", err)
	}

	log.Println("✓ Seeding completed successfully!")
	log.Printf("Created %d clinics with %d forms each", config.NumClinics, config.NumFormsPerClinic)
}

func getOrCreatePractitionerAdvanced(db *sqlx.DB, config AdvancedSeedConfig) (uuid.UUID, error) {
	if config.CreatePractitioner {
		return createNewPractitioner(db, config.PractitionerEmail)
	}

	var practitionerID uuid.UUID
	err := db.Get(&practitionerID, "SELECT id FROM tbl_practitioner LIMIT 1")
	if err == nil {
		return practitionerID, nil
	}

	if err == sql.ErrNoRows {
		log.Println("No existing practitioner found, creating one...")
		return createNewPractitioner(db, config.PractitionerEmail)
	}

	return uuid.Nil, err
}

func createNewPractitioner(db *sqlx.DB, email string) (uuid.UUID, error) {
	userID := uuid.New()
	
	if email == "" {
		email = gofakeit.Email()
	}
	
	firstName := gofakeit.FirstName()
	lastName := gofakeit.LastName()
	
	_, err := db.Exec(`
		INSERT INTO tbl_user (id, email, first_name, last_name, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, userID, email, firstName, lastName, "PRACTITIONER", true, time.Now(), time.Now())
	
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create user: %w", err)
	}

	practitionerID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO tbl_practitioner (id, user_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4)
	`, practitionerID, userID, time.Now(), time.Now())
	
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create practitioner: %w", err)
	}

	log.Printf("Created new practitioner: %s (%s %s)", email, firstName, lastName)
	return practitionerID, nil
}

func seedClinicsAndFormsAdvanced(db *sqlx.DB, practitionerID uuid.UUID, config AdvancedSeedConfig) error {
	ctx := context.Background()
	startTime := time.Now()

	for i := 0; i < config.NumClinics; i++ {
		clinicID, clinicName, err := createClinicAdvanced(ctx, db, practitionerID, config)
		if err != nil {
			return fmt.Errorf("failed to create clinic %d: %w", i+1, err)
		}

		if config.Verbose {
			log.Printf("[%d/%d] Created clinic: %s (ID: %s)", i+1, config.NumClinics, clinicName, clinicID)
		} else {
			log.Printf("Created clinic %d/%d", i+1, config.NumClinics)
		}

		// Create forms for this clinic
		for j := 0; j < config.NumFormsPerClinic; j++ {
			formID, formName, err := createFormAdvanced(ctx, db, clinicID, config)
			if err != nil {
				return fmt.Errorf("failed to create form %d for clinic %s: %w", j+1, clinicID, err)
			}
			
			if config.Verbose {
				log.Printf("  [%d/%d] Created form: %s (ID: %s)", j+1, config.NumFormsPerClinic, formName, formID)
			}
		}
	}

	elapsed := time.Since(startTime)
	log.Printf("Seeding completed in %s", elapsed)
	
	return nil
}

func createClinicAdvanced(ctx context.Context, db *sqlx.DB, practitionerID uuid.UUID, config AdvancedSeedConfig) (uuid.UUID, string, error) {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return uuid.Nil, "", err
	}
	defer tx.Rollback()

	clinicID := uuid.New()
	entityID := uuid.New()
	
	// Generate realistic clinic data
	name := fmt.Sprintf("%s %s", gofakeit.Company(), gofakeit.RandomString([]string{"Clinic", "Medical Center", "Health Services", "Practice"}))
	abn := gofakeit.Numerify("###########")
	description := gofakeit.Sentence(gofakeit.Number(8, 15))
	
	// Insert clinic
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tbl_clinic (id, practitioner_id, entity_id, name, abn, description, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, clinicID, practitionerID, entityID, name, abn, description, true, time.Now(), time.Now())
	
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("failed to insert clinic: %w", err)
	}

	// Insert multiple addresses (1-3 per clinic)
	numAddresses := gofakeit.Number(1, 3)
	for i := 0; i < numAddresses; i++ {
		addressID := uuid.New()
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tbl_clinic_address (id, clinic_id, address, city, state, postcode, is_primary, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, addressID, clinicID, gofakeit.Street(), gofakeit.City(), gofakeit.StateAbr(), 
			gofakeit.Numerify("####"), i == 0, time.Now(), time.Now())
		
		if err != nil {
			return uuid.Nil, "", fmt.Errorf("failed to insert clinic address: %w", err)
		}
	}

	// Insert multiple contacts (2-4 per clinic)
	contactTypes := []string{"EMAIL", "PHONE", "WEBSITE", "FAX"}
	numContacts := gofakeit.Number(2, 4)
	
	for i := 0; i < numContacts; i++ {
		contactID := uuid.New()
		contactType := contactTypes[i%len(contactTypes)]
		
		var value, label string
		switch contactType {
		case "EMAIL":
			value = gofakeit.Email()
			label = gofakeit.RandomString([]string{"Primary Email", "Admin Email", "Billing Email"})
		case "PHONE":
			value = gofakeit.Phone()
			label = gofakeit.RandomString([]string{"Main Phone", "Reception", "Emergency"})
		case "WEBSITE":
			value = fmt.Sprintf("https://%s.com", gofakeit.Username())
			label = "Website"
		case "FAX":
			value = gofakeit.Phone()
			label = "Fax"
		}
		
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tbl_clinic_contact (id, clinic_id, contact_type, value, label, is_primary, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, contactID, clinicID, contactType, value, label, i == 0, time.Now(), time.Now())
		
		if err != nil {
			return uuid.Nil, "", fmt.Errorf("failed to insert clinic contact: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return uuid.Nil, "", err
	}

	return clinicID, name, nil
}

func createFormAdvanced(ctx context.Context, db *sqlx.DB, clinicID uuid.UUID, config AdvancedSeedConfig) (uuid.UUID, string, error) {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return uuid.Nil, "", err
	}
	defer tx.Rollback()

	formID := uuid.New()
	
	// Generate realistic form names
	formTypes := []string{"Consultation", "Treatment", "Assessment", "Billing", "Service", "Procedure"}
	formName := fmt.Sprintf("%s %s Form", gofakeit.JobTitle(), gofakeit.RandomString(formTypes))
	description := gofakeit.Sentence(gofakeit.Number(6, 12))
	
	// Random method and status with weighted distribution
	methods := []string{"INDEPENDENT_CONTRACTOR", "SERVICE_FEE"}
	statuses := []string{"DRAFT", "PUBLISHED"}
	method := methods[gofakeit.Number(0, 1)]
	status := statuses[gofakeit.Number(0, 1)]
	
	// More realistic share distribution
	ownerShare := gofakeit.Number(40, 80)
	clinicShare := 100 - ownerShare
	superComponent := float64(gofakeit.Number(9, 12)) + gofakeit.Float64Range(0, 0.5)
	
	// Insert form
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tbl_form (id, clinic_id, name, description, status, method, owner_share, clinic_share, super_component, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, formID, clinicID, formName, description, status, method, ownerShare, clinicShare, superComponent, time.Now(), time.Now())
	
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("failed to insert form: %w", err)
	}

	// Get practitioner ID from clinic
	var practitionerID uuid.UUID
	err = tx.GetContext(ctx, &practitionerID, `
		SELECT practitioner_id FROM tbl_clinic WHERE id = $1
	`, clinicID)
	
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("failed to get practitioner ID: %w", err)
	}

	// Create a form version
	versionID := uuid.New()
	versionNumber := 1
	
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tbl_custom_form_version (id, form_id, version, is_active, practitioner_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, versionID, formID, versionNumber, true, practitionerID, time.Now(), time.Now())
	
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("failed to insert form version: %w", err)
	}

	// Create form fields based on config
	fieldTemplates := []struct {
		key         string
		label       string
		isComputed  bool
		sectionType string
		taxType     string
	}{
		{"A", "Total Revenue", false, "COLLECTION", "INCLUSIVE"},
		{"B", "Service Fees", false, "COLLECTION", "EXCLUSIVE"},
		{"C", "Direct Costs", false, "COST", "EXCLUSIVE"},
		{"D", "Labor Costs", false, "COST", "INCLUSIVE"},
		{"E", "Operating Expenses", false, "OTHER_COST", "EXCLUSIVE"},
		{"F", "Administrative Costs", false, "OTHER_COST", "EXCLUSIVE"},
		{"G", "Gross Profit", true, "", ""},
		{"H", "Net Income", true, "", ""},
		{"I", "Tax Amount", true, "", ""},
		{"J", "Final Total", true, "", ""},
	}

	numFields := config.NumFieldsPerForm
	if numFields > len(fieldTemplates) {
		numFields = len(fieldTemplates)
	}

	// Get or create COA IDs
	var coaIDs []uuid.UUID
	err = tx.SelectContext(ctx, &coaIDs, `
		SELECT id FROM tbl_chart_of_accounts WHERE practitioner_id = $1 ORDER BY code
	`, practitionerID)
	
	// Calculate how many non-computed fields we'll have
	nonComputedCount := 0
	for i := 0; i < numFields; i++ {
		if !fieldTemplates[i].isComputed {
			nonComputedCount++
		}
	}
	
	// If no COA exists or not enough, create sufficient COAs
	if err == sql.ErrNoRows || len(coaIDs) == 0 || len(coaIDs) < nonComputedCount {
		coaIDs, err = createBasicCOAAdvanced(ctx, tx, practitionerID)
		if err != nil {
			return uuid.Nil, "", fmt.Errorf("failed to create COA: %w", err)
		}
	} else if err != nil {
		return uuid.Nil, "", fmt.Errorf("failed to get COA: %w", err)
	}
	
	// Verify we have enough COAs
	if len(coaIDs) < nonComputedCount {
		return uuid.Nil, "", fmt.Errorf("insufficient COA entries: have %d, need %d", len(coaIDs), nonComputedCount)
	}

	coaIndex := 0
	fieldIDMap := make(map[string]uuid.UUID) // Map field keys to their IDs
	
	for i := 0; i < numFields; i++ {
		field := fieldTemplates[i]
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
				return uuid.Nil, "", fmt.Errorf("not enough COA entries for all fields (need at least %d)", coaIndex+1)
			}
			pr := gofakeit.RandomString([]string{"OWNER", "CLINIC"})
			paymentResp = &pr
		}
		
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tbl_form_field (id, form_version_id, field_key, label, is_computed, is_formula, 
				section_type, payment_responsibility, tax_type, coa_id, sort_order, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`, fieldID, versionID, field.key, field.label, field.isComputed, field.isComputed,
			sectionType, paymentResp, taxType, coaIDPtr, i+1, time.Now(), time.Now())
		
		if err != nil {
			return uuid.Nil, "", fmt.Errorf("failed to insert form field: %w", err)
		}
	}

	// Create formulas for computed fields
	// Formula for field G (Gross Profit) = A + B - C - D
	if numFields >= 7 {
		if fieldG, ok := fieldIDMap["G"]; ok {
			err = createFormulaGrossProfit(ctx, tx, versionID, fieldG, fieldIDMap)
			if err != nil {
				return uuid.Nil, "", fmt.Errorf("failed to create formula for Gross Profit: %w", err)
			}
		}
	}

	// Formula for field H (Net Income) = G - E - F
	if numFields >= 8 {
		if fieldH, ok := fieldIDMap["H"]; ok {
			err = createFormulaNetIncome(ctx, tx, versionID, fieldH, fieldIDMap)
			if err != nil {
				return uuid.Nil, "", fmt.Errorf("failed to create formula for Net Income: %w", err)
			}
		}
	}

	// Formula for field I (Tax Amount) = H * 0.1 (10% tax)
	if numFields >= 9 {
		if fieldI, ok := fieldIDMap["I"]; ok {
			err = createFormulaTaxAmount(ctx, tx, versionID, fieldI, fieldIDMap)
			if err != nil {
				return uuid.Nil, "", fmt.Errorf("failed to create formula for Tax Amount: %w", err)
			}
		}
	}

	// Formula for field J (Final Total) = H - I
	if numFields >= 10 {
		if fieldJ, ok := fieldIDMap["J"]; ok {
			err = createFormulaFinalTotal(ctx, tx, versionID, fieldJ, fieldIDMap)
			if err != nil {
				return uuid.Nil, "", fmt.Errorf("failed to create formula for Final Total: %w", err)
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return uuid.Nil, "", err
	}

	return formID, formName, nil
}

func createBasicCOAAdvanced(ctx context.Context, tx *sqlx.Tx, practitionerID uuid.UUID) ([]uuid.UUID, error) {
	// Create enough COA entries to ensure uniqueness per field (at least 12 unique COAs)
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
		{4300, "product_sales", "Product Sales", 4, 1},
		{5000, "cogs", "Cost of Goods Sold", 5, 1},
		{5100, "direct_labor", "Direct Labor", 5, 1},
		{5200, "materials", "Materials", 5, 1},
		{5300, "subcontractor_costs", "Subcontractor Costs", 5, 1},
		{6000, "operating_expenses", "Operating Expenses", 5, 1},
		{6100, "admin_expenses", "Administrative Expenses", 5, 1},
		{6200, "marketing_expenses", "Marketing Expenses", 5, 1},
		{6300, "utilities", "Utilities", 5, 1},
		{7000, "other_income", "Other Income", 4, 1},
		{7100, "misc_income", "Miscellaneous Income", 4, 1},
		{7200, "interest_income", "Interest Income", 4, 1},
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

// Formula helper functions

// createFormulaGrossProfit creates formula: G = A + B - C - D
func createFormulaGrossProfit(ctx context.Context, tx *sqlx.Tx, versionID, fieldG uuid.UUID, fieldIDMap map[string]uuid.UUID) error {
	formulaID := uuid.New()
	
	_, err := tx.ExecContext(ctx, `
		INSERT INTO tbl_formula (id, form_version_id, field_id, name, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, formulaID, versionID, fieldG, "Gross Profit Calculation", time.Now())
	
	if err != nil {
		return err
	}

	// Tree: ((A + B) - C) - D
	// Root: -
	rootID := uuid.New()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, operator, position, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, rootID, formulaID, nil, "OPERATOR", "-", nil, time.Now())
	if err != nil {
		return err
	}

	// Left: (A + B) - C
	leftSubID := uuid.New()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, operator, position, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, leftSubID, formulaID, rootID, "OPERATOR", "-", 0, time.Now())
	if err != nil {
		return err
	}

	// Left-Left: A + B
	addID := uuid.New()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, operator, position, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, addID, formulaID, leftSubID, "OPERATOR", "+", 0, time.Now())
	if err != nil {
		return err
	}

	// A
	if fieldA, ok := fieldIDMap["A"]; ok {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, field_id, position, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, uuid.New(), formulaID, addID, "FIELD", fieldA, 0, time.Now())
		if err != nil {
			return err
		}
	}

	// B
	if fieldB, ok := fieldIDMap["B"]; ok {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, field_id, position, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, uuid.New(), formulaID, addID, "FIELD", fieldB, 1, time.Now())
		if err != nil {
			return err
		}
	}

	// C
	if fieldC, ok := fieldIDMap["C"]; ok {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, field_id, position, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, uuid.New(), formulaID, leftSubID, "FIELD", fieldC, 1, time.Now())
		if err != nil {
			return err
		}
	}

	// D
	if fieldD, ok := fieldIDMap["D"]; ok {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, field_id, position, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, uuid.New(), formulaID, rootID, "FIELD", fieldD, 1, time.Now())
		if err != nil {
			return err
		}
	}

	return nil
}

// createFormulaNetIncome creates formula: H = G - E - F
func createFormulaNetIncome(ctx context.Context, tx *sqlx.Tx, versionID, fieldH uuid.UUID, fieldIDMap map[string]uuid.UUID) error {
	formulaID := uuid.New()
	
	_, err := tx.ExecContext(ctx, `
		INSERT INTO tbl_formula (id, form_version_id, field_id, name, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, formulaID, versionID, fieldH, "Net Income Calculation", time.Now())
	
	if err != nil {
		return err
	}

	// Tree: (G - E) - F
	// Root: -
	rootID := uuid.New()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, operator, position, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, rootID, formulaID, nil, "OPERATOR", "-", nil, time.Now())
	if err != nil {
		return err
	}

	// Left: G - E
	leftSubID := uuid.New()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, operator, position, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, leftSubID, formulaID, rootID, "OPERATOR", "-", 0, time.Now())
	if err != nil {
		return err
	}

	// G
	if fieldG, ok := fieldIDMap["G"]; ok {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, field_id, position, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, uuid.New(), formulaID, leftSubID, "FIELD", fieldG, 0, time.Now())
		if err != nil {
			return err
		}
	}

	// E
	if fieldE, ok := fieldIDMap["E"]; ok {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, field_id, position, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, uuid.New(), formulaID, leftSubID, "FIELD", fieldE, 1, time.Now())
		if err != nil {
			return err
		}
	}

	// F
	if fieldF, ok := fieldIDMap["F"]; ok {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, field_id, position, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, uuid.New(), formulaID, rootID, "FIELD", fieldF, 1, time.Now())
		if err != nil {
			return err
		}
	}

	return nil
}

// createFormulaTaxAmount creates formula: I = H * 0.1
func createFormulaTaxAmount(ctx context.Context, tx *sqlx.Tx, versionID, fieldI uuid.UUID, fieldIDMap map[string]uuid.UUID) error {
	formulaID := uuid.New()
	
	_, err := tx.ExecContext(ctx, `
		INSERT INTO tbl_formula (id, form_version_id, field_id, name, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, formulaID, versionID, fieldI, "Tax Amount Calculation", time.Now())
	
	if err != nil {
		return err
	}

	// Tree: H * 0.1
	// Root: *
	rootID := uuid.New()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, operator, position, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, rootID, formulaID, nil, "OPERATOR", "*", nil, time.Now())
	if err != nil {
		return err
	}

	// H
	if fieldH, ok := fieldIDMap["H"]; ok {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, field_id, position, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, uuid.New(), formulaID, rootID, "FIELD", fieldH, 0, time.Now())
		if err != nil {
			return err
		}
	}

	// 0.1 constant
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, constant_value, position, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, uuid.New(), formulaID, rootID, "CONSTANT", 0.1, 1, time.Now())
	if err != nil {
		return err
	}

	return nil
}

// createFormulaFinalTotal creates formula: J = H - I
func createFormulaFinalTotal(ctx context.Context, tx *sqlx.Tx, versionID, fieldJ uuid.UUID, fieldIDMap map[string]uuid.UUID) error {
	formulaID := uuid.New()
	
	_, err := tx.ExecContext(ctx, `
		INSERT INTO tbl_formula (id, form_version_id, field_id, name, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, formulaID, versionID, fieldJ, "Final Total Calculation", time.Now())
	
	if err != nil {
		return err
	}

	// Tree: H - I
	// Root: -
	rootID := uuid.New()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, operator, position, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, rootID, formulaID, nil, "OPERATOR", "-", nil, time.Now())
	if err != nil {
		return err
	}

	// H
	if fieldH, ok := fieldIDMap["H"]; ok {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, field_id, position, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, uuid.New(), formulaID, rootID, "FIELD", fieldH, 0, time.Now())
		if err != nil {
			return err
		}
	}

	// I
	if fieldI, ok := fieldIDMap["I"]; ok {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tbl_formula_node (id, formula_id, parent_id, node_type, field_id, position, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, uuid.New(), formulaID, rootID, "FIELD", fieldI, 1, time.Now())
		if err != nil {
			return err
		}
	}

	return nil
}
