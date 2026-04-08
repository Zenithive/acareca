package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/iamarpitzala/acareca/internal/shared/db"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	// Command line flags
	confirm := flag.Bool("confirm", false, "Confirm deletion of seeded data")
	dryRun := flag.Bool("dry-run", false, "Show what would be deleted without actually deleting")
	
	flag.Parse()

	if !*confirm && !*dryRun {
		log.Println("⚠️  WARNING: This will delete seeded data from the database!")
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

	if *dryRun {
		log.Println("🔍 DRY RUN MODE - No data will be deleted")
		showStats(db)
		return
	}

	if *confirm {
		log.Println("🗑️  Starting cleanup...")
		if err := cleanupData(db); err != nil {
			log.Fatalf("Failed to cleanup data: %v", err)
		}
		log.Println("✓ Cleanup completed successfully!")
		showStats(db)
	}
}


func showStats(db *sqlx.DB) {
	type Stats struct {
		Table string
		Count int
	}

	tables := []string{
		"tbl_clinic",
		"tbl_clinic_address",
		"tbl_clinic_contact",
		"tbl_form",
		"tbl_custom_form_version",
		"tbl_form_field",
		"tbl_chart_of_accounts",
		"tbl_practitioner",
		"tbl_user",
	}

	log.Println("\n📊 Current Database Statistics:")
	log.Println("================================")
	
	for _, table := range tables {
		var count int
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
		err := db.Get(&count, query)
		if err != nil {
			log.Printf("%-35s: Error - %v", table, err)
			continue
		}
		log.Printf("%-35s: %d records", table, count)
	}
	log.Println("================================\n")
}

func cleanupData(db *sqlx.DB) error {
	// Order matters due to foreign key constraints
	queries := []string{
		"DELETE FROM tbl_form_field",
		"DELETE FROM tbl_custom_form_version",
		"DELETE FROM tbl_form",
		// Don't delete chart of accounts - keep them for reuse
		// "DELETE FROM tbl_chart_of_accounts WHERE is_system = FALSE",
		"DELETE FROM tbl_clinic_contact",
		"DELETE FROM tbl_clinic_address",
		"DELETE FROM tbl_clinic",
		// Optionally delete practitioners and users
		// "DELETE FROM tbl_practitioner",
		// "DELETE FROM tbl_user WHERE role = 'PRACTITIONER'",
	}

	for _, query := range queries {
		result, err := db.Exec(query)
		if err != nil {
			return fmt.Errorf("failed to execute %s: %w", query, err)
		}
		
		rowsAffected, _ := result.RowsAffected()
		log.Printf("Deleted %d records from %s", rowsAffected, extractTableName(query))
	}

	return nil
}

func extractTableName(query string) string {
	// Simple extraction of table name from DELETE query
	var tableName string
	fmt.Sscanf(query, "DELETE FROM %s", &tableName)
	return tableName
}
