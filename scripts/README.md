# Seed Scripts

This directory contains scripts for seeding the database with fake data for development and testing purposes.

## seed_clinics_forms.go

Generates fake clinics and forms using the `gofakeit` library.

### Features

- Creates fake practitioners (if none exist)
- Generates realistic clinic data with:
  - Company names
  - ABN (Australian Business Number)
  - Addresses with city, state, and postcode
  - Contact information (email and phone)
- Creates forms for each clinic with:
  - Random form names based on job titles
  - Random methods (INDEPENDENT_CONTRACTOR or SERVICE_FEE)
  - Random status (DRAFT or PUBLISHED)
  - Owner/clinic share splits
  - Form versions
  - Form fields with different types (COLLECTION, COST, OTHER_COST)
  - Chart of Accounts associations

### Configuration

Edit the `SeedConfig` in `main()` to adjust:

```go
config := SeedConfig{
    NumClinics: 10,  // Number of clinics to create
    NumForms:   5,   // Number of forms per clinic
}
```

### Usage

1. Ensure your `.env` file is configured with database credentials:
   ```
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=your_user
   DB_PASSWORD=your_password
   DB_NAME=your_database
   ```

2. Run the script:
   ```bash
   go run scripts/seed_clinics_forms.go
   ```

   Or from the backend directory:
   ```bash
   cd backend
   go run scripts/seed_clinics_forms.go
   ```

### Output

The script will log progress as it creates data:
```
Starting seed with practitioner ID: xxx-xxx-xxx
Created clinic 1/10: xxx-xxx-xxx
  Created form 1/5: xxx-xxx-xxx
  Created form 2/5: xxx-xxx-xxx
  ...
Seeding completed successfully!
```

### Dependencies

- `github.com/brianvoe/gofakeit/v6` - Fake data generation
- `github.com/google/uuid` - UUID generation
- `github.com/jmoiron/sqlx` - Database operations
- `github.com/joho/godotenv` - Environment variable loading
- `github.com/lib/pq` - PostgreSQL driver

### Notes

- The script uses transactions to ensure data consistency
- If a practitioner already exists, it will use the first one found
- COA (Chart of Accounts) entries are created if they don't exist for a clinic
- All generated data is random but follows the database schema constraints

---

## seed_advanced.go

An enhanced version with command-line flags for more control over the seeding process.

### Features

All features from `seed_clinics_forms.go` plus:

- Command-line arguments for configuration
- Option to create new practitioners
- Verbose logging mode
- Configurable number of fields per form
- Multiple addresses per clinic (1-3)
- Multiple contacts per clinic (2-4)
- More realistic data generation
- Performance timing

### Usage

```bash
# Basic usage with defaults
go run scripts/seed_advanced.go

# Custom configuration
go run scripts/seed_advanced.go -clinics 20 -forms 10 -fields 8

# Create a new practitioner
go run scripts/seed_advanced.go -create-practitioner -practitioner-email "test@example.com"

# Verbose mode
go run scripts/seed_advanced.go -verbose

# All options combined
go run scripts/seed_advanced.go -clinics 5 -forms 3 -fields 6 -verbose -create-practitioner
```

### Command-Line Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-clinics` | int | 10 | Number of clinics to create |
| `-forms` | int | 5 | Number of forms per clinic |
| `-fields` | int | 6 | Number of fields per form (max 10) |
| `-create-practitioner` | bool | false | Create a new practitioner |
| `-practitioner-email` | string | "" | Email for new practitioner |
| `-practitioner-id` | string | "" | Specific practitioner UUID to seed for |
| `-verbose` | bool | false | Enable verbose logging |

### Examples

Create a small test dataset:
```bash
go run scripts/seed_advanced.go -clinics 2 -forms 2 -fields 4
```

Create a large dataset with verbose output:
```bash
go run scripts/seed_advanced.go -clinics 50 -forms 10 -fields 8 -verbose
```

Create data for a specific practitioner:
```bash
go run scripts/seed_advanced.go \
  -practitioner-id c9b2ecb5-ced4-43ea-9507-ac9ba56482f0 \
  -clinics 5 \
  -forms 3
```

Create a new practitioner and seed data:
```bash
go run scripts/seed_advanced.go -create-practitioner -practitioner-email "john.doe@clinic.com" -clinics 5
```

### Makefile Targets

Add to your makefile for convenience:

```makefile
seed:
	go run scripts/seed_clinics_forms.go

seed-advanced:
	go run scripts/seed_advanced.go

seed-large:
	go run scripts/seed_advanced.go -clinics 50 -forms 10 -verbose

seed-cleanup:
	go run scripts/cleanup_seed.go -confirm

seed-cleanup-dry:
	go run scripts/cleanup_seed.go -dry-run
```

---

## cleanup_seed.go

Utility script to clean up seeded data from the database.

### Features

- Dry-run mode to preview what will be deleted
- Shows database statistics before and after cleanup
- Respects foreign key constraints
- Safe deletion order
- Confirmation required for actual deletion

### Usage

```bash
# Preview what will be deleted (safe)
go run scripts/cleanup_seed.go -dry-run
# or
make seed-cleanup-dry

# Actually delete the data (requires confirmation)
go run scripts/cleanup_seed.go -confirm
# or
make seed-cleanup
```

### What Gets Deleted

The script deletes in this order (respecting foreign keys):
1. Form fields
2. Form versions
3. Forms
4. Clinic chart of accounts
5. Clinic contacts
6. Clinic addresses
7. Clinics

**Note:** Practitioners and users are NOT deleted by default. Uncomment the relevant lines in the script if you want to delete them too.

### Safety Features

- Requires explicit `-confirm` flag to delete
- Provides `-dry-run` mode to preview
- Shows statistics before and after
- Warns user if no flags provided

---

## cleanup_practitioner.go

Utility script to delete all clinics and forms for a specific practitioner.

### Features

- Target specific practitioner by UUID
- Dry-run mode to preview what will be deleted
- Shows detailed statistics for the practitioner
- Transactional deletion (all or nothing)
- Validates practitioner exists before deletion
- Safe deletion order respecting foreign keys

### Usage

```bash
# Preview what will be deleted for a practitioner (safe)
go run scripts/cleanup_practitioner.go -practitioner-id <UUID> -dry-run

# Actually delete the data (requires confirmation)
go run scripts/cleanup_practitioner.go -practitioner-id <UUID> -confirm

# Using makefile
make cleanup-practitioner-dry PRACTITIONER_ID=<UUID>
make cleanup-practitioner PRACTITIONER_ID=<UUID>
```

### Example

```bash
# Dry run first to see what will be deleted
go run scripts/cleanup_practitioner.go \
  -practitioner-id c9b2ecb5-ced4-43ea-9507-ac9ba56482f0 \
  -dry-run

# Then confirm deletion
go run scripts/cleanup_practitioner.go \
  -practitioner-id c9b2ecb5-ced4-43ea-9507-ac9ba56482f0 \
  -confirm
```

### What Gets Deleted

The script deletes in this order (respecting foreign keys):
1. Form fields (for all forms owned by practitioner's clinics)
2. Form versions (owned by practitioner)
3. Forms (for all clinics owned by practitioner)
4. Chart of accounts (non-system entries for practitioner)
5. Clinic contacts (for all clinics owned by practitioner)
6. Clinic addresses (for all clinics owned by practitioner)
7. Clinics (owned by practitioner)

**Note:** The practitioner record itself is NOT deleted. Only their clinics, forms, and related data.

### Statistics Shown

The script displays counts for:
- Clinics
- Clinic Addresses
- Clinic Contacts
- Forms
- Form Versions
- Form Fields
- Chart of Accounts (non-system)

### Safety Features

- Requires practitioner UUID (no wildcards)
- Validates practitioner exists before proceeding
- Requires explicit `-confirm` flag to delete
- Provides `-dry-run` mode to preview
- Uses transactions (rollback on any error)
- Shows statistics before and after deletion
