package practitioner

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

type PractitionerRepositoryTestSuite struct {
	suite.Suite
	db   *sqlx.DB
	repo Repository
}

func (s *PractitionerRepositoryTestSuite) SetupSuite() {
	s.db = testutil.OpenTestDB(s.T())
	s.repo = NewRepository(s.db)
}

func (s *PractitionerRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
}

func (s *PractitionerRepositoryTestSuite) createTestPractitioner(ctx context.Context, tx *sqlx.Tx) *RsPractitioner {
	userID := uuid.New()
	_, err := tx.ExecContext(ctx,
		`INSERT INTO tbl_user (id, email, password, first_name, last_name, phone, role) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		userID,
		"practitioner-test-"+uuid.NewString()+"@example.com",
		"secret",
		"Practitioner",
		"Tester",
		"1234567890",
		"PRACTITIONER",
	)
	s.Require().NoError(err)

	practitionerName := lo.ToPtr("Test Practitioner")
	practitioner, err := s.repo.CreatePractitioner(ctx, &RqCreatePractitioner{
		UserID:     userID.String(),
		EntityType: "SOLE_TRADER",
		EntityName: practitionerName,
		ABN:        nil,
		ACN:        nil,
		Address:    nil,
		Profession: nil,
	}, tx)
	s.Require().NoError(err)
	s.Require().NotEqual(uuid.Nil, practitioner.ID)
	return practitioner
}

func (s *PractitionerRepositoryTestSuite) TestCreatePractitioner() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	practitioner := s.createTestPractitioner(ctx, tx)
	testutil.CommitTx(s.T(), tx)

	s.Require().NotEqual(uuid.Nil, practitioner.ID)
}

func (s *PractitionerRepositoryTestSuite) TestGetPractitioner() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	practitioner := s.createTestPractitioner(ctx, tx)
	testutil.CommitTx(s.T(), tx)

	fetched, err := s.repo.GetPractitioner(ctx, practitioner.ID)
	s.Require().NoError(err)
	s.Require().Equal(practitioner.ID, fetched.ID)
}

func (s *PractitionerRepositoryTestSuite) TestCountPractitioners() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	s.createTestPractitioner(ctx, tx)
	testutil.CommitTx(s.T(), tx)

	total, err := s.repo.CountPractitioners(ctx, common.Filter{})
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(total, 1)
}

func (s *PractitionerRepositoryTestSuite) TestSoftDeletePractitioner() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	practitioner := s.createTestPractitioner(ctx, tx)
	testutil.CommitTx(s.T(), tx)

	err := s.repo.DeletePractitioner(ctx, practitioner.ID)
	s.Require().NoError(err)

	_, err = s.repo.GetPractitioner(ctx, practitioner.ID)
	s.Require().Error(err)
}

func TestPractitionerRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(PractitionerRepositoryTestSuite))
}
