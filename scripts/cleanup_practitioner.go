package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/db"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	// Command line flags
	practitionerID := flag.String("practitioner-id", "", "Practitioner UUID to delete data for (required)")
	confirm := flag.Bool("confirm", false, "Confirm deletion of practitioner's data")
	dryRun := flag.Bool("dry-run", false, "Show what would be deleted without actually deleting")
	
	flag.Parse()

	if *practitionerID == "" {
		log.Println("❌ Error: -practitioner-id is required")
		log.Println("\nUsage:")
		log.Println("  go run scripts/cleanup_practitioner.go -practitioner-id <UUID> -dry-run")
		log.Println("  go run scripts/cleanup_practitioner.go -practitioner-id <UUID> -confirm")
		return
	}

	// Validate UUID
	practID, err := uuid.Parse(*practitionerID)
	if err != nil {
		log.Fatalf("❌ Invalid practitioner ID: %v", err)
	}

	if !*confirm && !*dryRun {
		log.Println("⚠️  WARNING: This will delete all clinics and forms for the specified practitioner!")
		log.Println("Use -dry-run to see what would be deleted")
		log.Println("Use -confirm to actually delete the data")
		return
	}

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

	// Verify practitioner exists
	var exists bool
	err = db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM tbl_practitioner WHERE id = $1)", practID)
	if err != nil {
		log.Fatalf("Failed to check practitioner: %v", err)
	}
	if !exists {
		log.Fatalf("❌ Practitioner with ID %s does not exist", practID)
	}

	if *dryRun {
		log.Printf("🔍 DRY RUN MODE - No data will be deleted for practitioner: %s\n", practID)
		showPractitionerStats(db, practID)
		return
	}

	if *confirm {
		log.Printf("🗑️  Starting cleanup for practitioner: %s\n", practID)
		if err := cleanupPractitionerData(db, practID); err != nil {
			log.Fatalf("Failed to cleanup data: %v", err)
		}
		log.Println("✓ Cleanup completed successfully!")
		showPractitionerStats(db, practID)
	}
}


func showPractitionerStats(db *sqlx.DB, practitionerID uuid.UUID) {
	type Stats struct {
		Description string
		Count       int
	}

	stats := []Stats{}

	// Get clinic count
	var clinicCount int
	err := db.Get(&clinicCount, `
		SELECT COUNT(*) FROM tbl_clinic WHERE practitioner_id = $1
	`, practitionerID)
	if err == nil {
		stats = append(stats, Stats{"Clinics", clinicCount})
	}

	// Get clinic address count
	var addressCount int
	err = db.Get(&addressCount, `
		SELECT COUNT(*) FROM tbl_clinic_address 
		WHERE clinic_id IN (SELECT id FROM tbl_clinic WHERE practitioner_id = $1)
	`, practitionerID)
	if err == nil {
		stats = append(stats, Stats{"Clinic Addresses", addressCount})
	}

	// Get clinic contact count
	var contactCount int
	err = db.Get(&contactCount, `
		SELECT COUNT(*) FROM tbl_clinic_contact 
		WHERE clinic_id IN (SELECT id FROM tbl_clinic WHERE practitioner_id = $1)
	`, practitionerID)
	if err == nil {
		stats = append(stats, Stats{"Clinic Contacts", contactCount})
	}

	// Get form count
	var formCount int
	err = db.Get(&formCount, `
		SELECT COUNT(*) FROM tbl_form 
		WHERE clinic_id IN (SELECT id FROM tbl_clinic WHERE practitioner_id = $1)
	`, practitionerID)
	if err == nil {
		stats = append(stats, Stats{"Forms", formCount})
	}

	// Get form version count
	var versionCount int
	err = db.Get(&versionCount, `
		SELECT COUNT(*) FROM tbl_custom_form_version WHERE practitioner_id = $1
	`, practitionerID)
	if err == nil {
		stats = append(stats, Stats{"Form Versions", versionCount})
	}

	// Get form field count
	var fieldCount int
	err = db.Get(&fieldCount, `
		SELECT COUNT(*) FROM tbl_form_field 
		WHERE form_version_id IN (
			SELECT id FROM tbl_custom_form_version WHERE practitioner_id = $1
		)
	`, practitionerID)
	if err == nil {
		stats = append(stats, Stats{"Form Fields", fieldCount})
	}

	// Get COA count (non-system)
	var coaCount int
	err = db.Get(&coaCount, `
		SELECT COUNT(*) FROM tbl_chart_of_accounts 
		WHERE practitioner_id = $1 AND is_system = FALSE
	`, practitionerID)
	if err == nil {
		stats = append(stats, Stats{"Chart of Accounts (non-system)", coaCount})
	}

	log.Printf("\n📊 Statistics for Practitioner %s:\n", practitionerID)
	log.Println("================================")
	for _, stat := range stats {
		log.Printf("%-35s: %d records", stat.Description, stat.Count)
	}
	log.Println("================================\n")
}

func cleanupPractitionerData(db *sqlx.DB, practitionerID uuid.UUID) error {
	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete form fields (via form versions)
	result, err := tx.Exec(`
		DELETE FROM tbl_form_field 
		WHERE form_version_id IN (
			SELECT id FROM tbl_custom_form_version WHERE practitioner_id = $1
		)
	`, practitionerID)
	if err != nil {
		return fmt.Errorf("failed to delete form fields: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	log.Printf("Deleted %d form fields", rowsAffected)

	// Delete form versions
	result, err = tx.Exec(`
		DELETE FROM tbl_custom_form_version WHERE practitioner_id = $1
	`, practitionerID)
	if err != nil {
		return fmt.Errorf("failed to delete form versions: %w", err)
	}
	rowsAffected, _ = result.RowsAffected()
	log.Printf("Deleted %d form versions", rowsAffected)

	// Delete forms (via clinics)
	result, err = tx.Exec(`
		DELETE FROM tbl_form 
		WHERE clinic_id IN (
			SELECT id FROM tbl_clinic WHERE practitioner_id = $1
		)
	`, practitionerID)
	if err != nil {
		return fmt.Errorf("failed to delete forms: %w", err)
	}
	rowsAffected, _ = result.RowsAffected()
	log.Printf("Deleted %d forms", rowsAffected)

	// Don't delete chart of accounts - keep them for reuse
	log.Printf("Skipped chart of accounts deletion (preserving for reuse)")

	// Delete clinic contacts
	result, err = tx.Exec(`
		DELETE FROM tbl_clinic_contact 
		WHERE clinic_id IN (
			SELECT id FROM tbl_clinic WHERE practitioner_id = $1
		)
	`, practitionerID)
	if err != nil {
		return fmt.Errorf("failed to delete clinic contacts: %w", err)
	}
	rowsAffected, _ = result.RowsAffected()
	log.Printf("Deleted %d clinic contacts", rowsAffected)

	// Delete clinic addresses
	result, err = tx.Exec(`
		DELETE FROM tbl_clinic_address 
		WHERE clinic_id IN (
			SELECT id FROM tbl_clinic WHERE practitioner_id = $1
		)
	`, practitionerID)
	if err != nil {
		return fmt.Errorf("failed to delete clinic addresses: %w", err)
	}
	rowsAffected, _ = result.RowsAffected()
	log.Printf("Deleted %d clinic addresses", rowsAffected)

	// Delete clinics
	result, err = tx.Exec(`
		DELETE FROM tbl_clinic WHERE practitioner_id = $1
	`, practitionerID)
	if err != nil {
		return fmt.Errorf("failed to delete clinics: %w", err)
	}
	rowsAffected, _ = result.RowsAffected()
	log.Printf("Deleted %d clinics", rowsAffected)

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
