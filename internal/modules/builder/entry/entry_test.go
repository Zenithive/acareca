package entry

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
	"github.com/stretchr/testify/suite"
)

type EntryRepositoryTestSuite struct {
	suite.Suite
	db   *sqlx.DB
	repo IRepository
}

func (s *EntryRepositoryTestSuite) SetupSuite() {
	s.db = testutil.OpenTestDB(s.T())
	s.repo = NewRepository(s.db)
}

func (s *EntryRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
}

// Helper to handle dependency injection setup for a valid practitioner
func (s *EntryRepositoryTestSuite) seedPractitioner(ctx context.Context, tx *sqlx.Tx) uuid.UUID {
	userID := uuid.New()
	_, err := tx.ExecContext(ctx,
		`INSERT INTO tbl_user (id, email, password, first_name, last_name, phone, role) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		userID, "entry-practitioner-"+uuid.NewString()+"@example.com", "password", "Entry", "Tester", "1115559999", "PRACTITIONER",
	)
	s.Require().NoError(err)

	practitionerID := uuid.New()
	_, err = tx.ExecContext(ctx,
		`INSERT INTO tbl_practitioner (id, user_id, entity_type, entity_name) VALUES ($1, $2, $3, $4)`,
		practitionerID, userID, "SOLE_TRADER", "Entry Test Practitioner",
	)
	s.Require().NoError(err)
	return practitionerID
}

// 🛠️ FIX: Rewritten to reflect the exact structure shown in image_fcb508.png and your sample values
func (s *EntryRepositoryTestSuite) seedChartOfAccounts(ctx context.Context, tx *sqlx.Tx, practitionerID uuid.UUID) uuid.UUID {
	coaID := uuid.New()
	now := time.Now()

	query := `
		INSERT INTO tbl_chart_of_accounts (
			id, practitioner_id, account_type_id, account_tax_id, code, name, is_system, created_at, updated_at, key, classification
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	_, err := tx.ExecContext(ctx, query,
		coaID,
		practitionerID,                  // practitioner_id (uuid)
		5,                               // account_type_id (int2)
		2,                               // account_tax_id (int2)
		405,                             // code (int2)
		"Subscription/Membership (GST)", // name (varchar)
		false,                           // is_system (bool)
		now,
		now,
		"subscriptionmembership_gst", // key (varchar)
		"Operating Expense",          // classification (account_classification enum value)
	)
	s.Require().NoError(err)
	return coaID
}

func (s *EntryRepositoryTestSuite) createSeededClinicAndVersion(ctx context.Context, tx *sqlx.Tx, clinicID uuid.UUID, method string) (uuid.UUID, uuid.UUID, uuid.UUID) {
	formID := uuid.New()
	versionID := uuid.New()
	now := time.Now()

	practitionerID := s.seedPractitioner(ctx, tx)

	queryForm := `
		INSERT INTO tbl_form (
			id, clinic_id, name, status, method, owner_share, clinic_share, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := tx.ExecContext(ctx, queryForm,
		formID, clinicID, "Intake Structure", "PUBLISHED", method, 70, 30, now, now,
	)
	s.Require().NoError(err)

	queryVersion := `
		INSERT INTO tbl_custom_form_version (
			id, form_id, version, is_active, practitioner_id, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err = tx.ExecContext(ctx, queryVersion,
		versionID, formID, 1, true, practitionerID, now, now,
	)
	s.Require().NoError(err)

	return formID, versionID, practitionerID
}

func (s *EntryRepositoryTestSuite) TestCreateAndGetStandardFormEntry() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	clinicID := uuid.New()
	_, versionID, _ := s.createSeededClinicAndVersion(ctx, tx, clinicID, "INDEPENDENT_CONTRACTOR")

	entryID := uuid.New()
	entry := &FormEntry{
		ID:            entryID,
		FormVersionID: versionID,
		ClinicID:      clinicID,
		Status:        "DRAFT",
	}

	valueID := uuid.New()
	fieldID := uuid.New()

	now := time.Now()
	_, err := tx.ExecContext(ctx,
		`INSERT INTO tbl_form_field (id, form_version_id, field_key, label, sort_order, is_computed, is_highlighted, created_at, updated_at) 
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		fieldID, versionID, "f_key", "Label Summary", 0, false, false, now, now,
	)
	s.Require().NoError(err)

	values := []*FormEntryValue{
		{
			ID:          valueID,
			EntryID:     entryID,
			FormFieldID: &fieldID,
			Description: lo.ToPtr("Standard medical entry text"),
		},
	}

	err = s.repo.Create(ctx, tx, entry, values)
	s.Require().NoError(err)
	testutil.CommitTx(s.T(), tx)

	readTx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), readTx)

	fetchedEntry, fetchedValues, err := s.repo.GetByID(ctx, readTx, entryID)
	s.Require().NoError(err)
	s.Require().Equal(entryID, fetchedEntry.ID)
	s.Require().Len(fetchedValues, 1)
}

func (s *EntryRepositoryTestSuite) TestCreateExpenseEntrySpecification() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	globalClinicID := uuid.Nil
	_, versionID, practitionerID := s.createSeededClinicAndVersion(ctx, tx, globalClinicID, "EXPENSE_ENTRY")

	entryID := uuid.New()
	entry := &FormEntry{
		ID:            entryID,
		FormVersionID: versionID,
		ClinicID:      globalClinicID,
		Status:        "SUBMITTED",
	}

	// 🛠️ FIX: Now passing the correct practitionerID to bind the Chart of Accounts record correctly
	coaID := s.seedChartOfAccounts(ctx, tx, practitionerID)
	valueID := uuid.New()
	values := []*FormEntryValue{
		{
			ID:          valueID,
			EntryID:     entryID,
			FormFieldID: nil,
			CoaID:       &coaID,
			Description: lo.ToPtr("Clinic operational hardware expenses"),
		},
	}

	err := s.repo.Create(ctx, tx, entry, values)
	s.Require().NoError(err)
	testutil.CommitTx(s.T(), tx)
}

func (s *EntryRepositoryTestSuite) TestListByFormVersionIDInterface() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	clinicID := uuid.New()
	_, versionID, _ := s.createSeededClinicAndVersion(ctx, tx, clinicID, "INDEPENDENT_CONTRACTOR")
	testutil.CommitTx(s.T(), tx)

	actorID := uuid.New()
	_, err := s.repo.ListByFormVersionID(ctx, versionID, common.Filter{}, actorID, "PRACTITIONER")
	s.Require().NoError(err)
}

func (s *EntryRepositoryTestSuite) TestDeleteEntryPipeline() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	clinicID := uuid.New()
	_, versionID, _ := s.createSeededClinicAndVersion(ctx, tx, clinicID, "INDEPENDENT_CONTRACTOR")

	entryID := uuid.New()
	entry := &FormEntry{
		ID:            entryID,
		FormVersionID: versionID,
		ClinicID:      clinicID,
		Status:        "DRAFT",
	}

	err := s.repo.Create(ctx, tx, entry, []*FormEntryValue{})
	s.Require().NoError(err)
	testutil.CommitTx(s.T(), tx)

	deleteTx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), deleteTx)

	err = s.repo.Delete(ctx, deleteTx, entryID)
	s.Require().NoError(err)
	testutil.CommitTx(s.T(), deleteTx)
}

func TestEntryRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(EntryRepositoryTestSuite))
}
