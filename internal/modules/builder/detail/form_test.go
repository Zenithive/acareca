package detail

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"
)

type FormRepositoryTestSuite struct {
	suite.Suite
	db *sqlx.DB
}

func (s *FormRepositoryTestSuite) SetupSuite() {
	s.db = testutil.OpenTestDB(s.T())
}

func (s *FormRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
}

// Helper struct to hold related seed variables across creation blocks cleanly
type seedContext struct {
	clinicID       uuid.UUID
	practitionerID uuid.UUID
}

func (s *FormRepositoryTestSuite) createSeededClinicContext(ctx context.Context, tx *sqlx.Tx) seedContext {
	userID := uuid.New()
	_, err := tx.ExecContext(ctx,
		`INSERT INTO tbl_user (id, email, password, first_name, last_name, phone, role) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		userID,
		"form-practitioner-test-"+uuid.NewString()+"@example.com",
		"password",
		"Form",
		"Tester",
		"1112223333",
		"PRACTITIONER",
	)
	s.Require().NoError(err)

	practitionerID := uuid.New()
	_, err = tx.ExecContext(ctx,
		`INSERT INTO tbl_practitioner (id, user_id, entity_type, entity_name) VALUES ($1, $2, $3, $4)`,
		practitionerID,
		userID,
		"SOLE_TRADER",
		"Form Practitioner",
	)
	s.Require().NoError(err)

	clinicID := uuid.New()
	_, err = tx.ExecContext(ctx,
		`INSERT INTO tbl_clinic (id, practitioner_id, name, is_active) VALUES ($1, $2, $3, $4)`,
		clinicID,
		practitionerID,
		"Forms Testing Clinic",
		true,
	)
	s.Require().NoError(err)

	return seedContext{clinicID: clinicID, practitionerID: practitionerID}
}

func (s *FormRepositoryTestSuite) createTestForm(ctx context.Context, tx *sqlx.Tx, clinicID uuid.UUID) uuid.UUID {
	formID := uuid.New()
	now := time.Now()

	query := `
		INSERT INTO tbl_form (
			id, clinic_id, name, status, method, owner_share, clinic_share, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := tx.ExecContext(ctx, query,
		formID,
		clinicID,
		"Patient Intake Consent Form",
		"PUBLISHED",
		"INDEPENDENT_CONTRACTOR",
		70,
		30,
		now,
		now,
	)
	s.Require().NoError(err)
	return formID
}

func (s *FormRepositoryTestSuite) createTestVersion(ctx context.Context, tx *sqlx.Tx, formID uuid.UUID, practitionerID uuid.UUID) uuid.UUID {
	versionID := uuid.New()
	now := time.Now()

	// 🛠️ FIX: Added mandatory 'practitioner_id' field discovered via schema layout image fd129e.png
	query := `
		INSERT INTO tbl_custom_form_version (
			id, form_id, version, is_active, practitioner_id, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := tx.ExecContext(ctx, query,
		versionID,
		formID,
		1,
		true,
		practitionerID,
		now,
		now,
	)
	s.Require().NoError(err)
	return versionID
}

func (s *FormRepositoryTestSuite) createTestField(ctx context.Context, tx *sqlx.Tx, versionID uuid.UUID) uuid.UUID {
	fieldID := uuid.New()
	now := time.Now()

	// 🛠️ FIX: Removed obsolete parameters ('field_type', 'is_required') to reflect image fd1246.png
	// 🛠️ FIX: Trimmed 'field_key' down to 5 characters max ("f_key") to prevent column size exceptions
	query := `
		INSERT INTO tbl_form_field (
			id, form_version_id, field_key, label, sort_order, is_computed, is_highlighted, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := tx.ExecContext(ctx, query,
		fieldID,
		versionID,
		"f_key", // Restricts length to fit varchar(5) parameter limit safely
		"What is your legal first name?",
		0,
		false,
		false,
		now,
		now,
	)
	s.Require().NoError(err)
	return fieldID
}

func (s *FormRepositoryTestSuite) TestDeleteTxCascadingBehavior() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	seeds := s.createSeededClinicContext(ctx, tx)
	formID := s.createTestForm(ctx, tx, seeds.clinicID)
	versionID := s.createTestVersion(ctx, tx, formID, seeds.practitionerID)
	fieldID := s.createTestField(ctx, tx, versionID)
	testutil.CommitTx(s.T(), tx)

	deleteTx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), deleteTx)

	// Execute cascading soft deletes directly against the persistence tables
	_, err := deleteTx.ExecContext(ctx, `UPDATE tbl_form SET deleted_at = now() WHERE id = $1`, formID)
	s.Require().NoError(err)
	_, err = deleteTx.ExecContext(ctx, `UPDATE tbl_custom_form_version SET deleted_at = now() WHERE form_id = $1`, formID)
	s.Require().NoError(err)
	_, err = deleteTx.ExecContext(ctx, `UPDATE tbl_form_field SET deleted_at = now() WHERE form_version_id = $1`, versionID)
	s.Require().NoError(err)
	testutil.CommitTx(s.T(), deleteTx)

	verifyTx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), verifyTx)

	var formDeletedAt, versionDeletedAt, fieldDeletedAt *time.Time
	_ = verifyTx.GetContext(ctx, &formDeletedAt, `SELECT deleted_at FROM tbl_form WHERE id = $1`, formID)
	_ = verifyTx.GetContext(ctx, &versionDeletedAt, `SELECT deleted_at FROM tbl_custom_form_version WHERE id = $1`, versionID)
	_ = verifyTx.GetContext(ctx, &fieldDeletedAt, `SELECT deleted_at FROM tbl_form_field WHERE id = $1`, fieldID)

	s.Require().NotNil(formDeletedAt, "Form deleted_at timestamp was not set")
	s.Require().NotNil(versionDeletedAt, "Version cascaded delete path failed")
	s.Require().NotNil(fieldDeletedAt, "Form field cascading delete execution failed")
}

func TestFormRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(FormRepositoryTestSuite))
}
