package clinic

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
	"github.com/stretchr/testify/suite"
)

type ClinicRepositoryTestSuite struct {
	suite.Suite
	db   *sqlx.DB
	repo Repository
}

func (s *ClinicRepositoryTestSuite) SetupSuite() {
	s.db = testutil.OpenTestDB(s.T())
	s.repo = NewRepository(s.db)
}

func (s *ClinicRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
}

func (s *ClinicRepositoryTestSuite) createTestClinic(ctx context.Context, tx *sqlx.Tx) *Clinic {
	userID := uuid.New()
	_, err := tx.ExecContext(ctx,
		`INSERT INTO tbl_user (id, email, password, first_name, last_name, phone, role) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		userID,
		"clinic-practitioner-test-"+uuid.NewString()+"@example.com",
		"password",
		"Clinic",
		"Practitioner",
		"4445556666",
		"PRACTITIONER",
	)
	s.Require().NoError(err)

	practitionerID := uuid.New()
	_, err = tx.ExecContext(ctx,
		`INSERT INTO tbl_practitioner (id, user_id, entity_type, entity_name, abn, acn, address, profession) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		practitionerID,
		userID,
		"SOLE_TRADER",
		"Clinic Practitioner",
		nil,
		nil,
		nil,
		nil,
	)
	s.Require().NoError(err)

	clinic, err := s.repo.CreateClinic(ctx, tx, &Clinic{
		PractitionerID: practitionerID,
		Name:           "Test Clinic",
		ABN:            nil,
		Description:    nil,
		IsActive:       true,
	})
	s.Require().NoError(err)
	s.Require().NotEqual(uuid.Nil, clinic.ID)
	return clinic
}

func (s *ClinicRepositoryTestSuite) createTestClinicAddress(ctx context.Context, tx *sqlx.Tx, clinicID uuid.UUID) *ClinicAddress {
	address, err := s.repo.CreateClinicAddress(ctx, tx, &ClinicAddress{
		ClinicID:  clinicID,
		Address:   lo.ToPtr("123 Test Street"),
		City:      lo.ToPtr("Testville"),
		State:     lo.ToPtr("Test State"),
		Postcode:  lo.ToPtr("9999"),
		IsPrimary: true,
	})
	s.Require().NoError(err)
	s.Require().NotEqual(uuid.Nil, address.ID)
	return address
}

func (s *ClinicRepositoryTestSuite) createTestClinicContact(ctx context.Context, tx *sqlx.Tx, clinicID uuid.UUID) *ClinicContact {
	contact, err := s.repo.CreateClinicContact(ctx, tx, &ClinicContact{
		ClinicID:    clinicID,
		ContactType: "PHONE",
		Value:       "555-0000",
		Label:       lo.ToPtr("Main"),
		IsPrimary:   true,
	})
	s.Require().NoError(err)
	s.Require().NotEqual(uuid.Nil, contact.ID)
	return contact
}

func (s *ClinicRepositoryTestSuite) TestCreateClinic() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	clinic := s.createTestClinic(ctx, tx)
	testutil.CommitTx(s.T(), tx)

	s.Require().NotEqual(uuid.Nil, clinic.ID)
}

func (s *ClinicRepositoryTestSuite) TestCreateClinicAddress() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	clinic := s.createTestClinic(ctx, tx)
	address := s.createTestClinicAddress(ctx, tx, clinic.ID)
	testutil.CommitTx(s.T(), tx)

	s.Require().NotEqual(uuid.Nil, address.ID)
}

func (s *ClinicRepositoryTestSuite) TestCreateClinicContact() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	clinic := s.createTestClinic(ctx, tx)
	contact := s.createTestClinicContact(ctx, tx, clinic.ID)
	testutil.CommitTx(s.T(), tx)

	s.Require().NotEqual(uuid.Nil, contact.ID)
}

func (s *ClinicRepositoryTestSuite) TestGetClinicByID() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	clinic := s.createTestClinic(ctx, tx)
	testutil.CommitTx(s.T(), tx)

	readTx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), readTx)
	fetched, err := s.repo.GetClinicByID(ctx, readTx, clinic.ID)
	s.Require().NoError(err)
	s.Require().Equal(clinic.ID, fetched.ID)
}

func (s *ClinicRepositoryTestSuite) TestListAndCountClinicByPractitioner() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	clinic := s.createTestClinic(ctx, tx)
	testutil.CommitTx(s.T(), tx)

	list, err := s.repo.ListClinicByPractitioner(ctx, clinic.PractitionerID, common.Filter{})
	s.Require().NoError(err)
	s.Require().NotEmpty(list)

	count, err := s.repo.CountClinicByPractitioner(ctx, clinic.PractitionerID, common.Filter{})
	s.Require().NoError(err)
	s.Require().Equal(len(list), count)
}

func (s *ClinicRepositoryTestSuite) TestSoftDeleteClinic() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	clinic := s.createTestClinic(ctx, tx)
	testutil.CommitTx(s.T(), tx)

	deleteTx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), deleteTx)
	err := s.repo.DeleteClinic(ctx, deleteTx, clinic.ID)
	s.Require().NoError(err)
	testutil.CommitTx(s.T(), deleteTx)

	readTx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), readTx)
	_, err = s.repo.GetClinicByID(ctx, readTx, clinic.ID)
	s.Require().Error(err)
}

func TestClinicRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(ClinicRepositoryTestSuite))
}
