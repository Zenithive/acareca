package coa

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"
)

type COARepositoryTestSuite struct {
	suite.Suite
	db   *sqlx.DB
	repo Repository
}

func (s *COARepositoryTestSuite) SetupSuite() {
	s.db = testutil.OpenTestDB(s.T())
	s.repo = NewRepository(s.db)
}

func (s *COARepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
}

func (s *COARepositoryTestSuite) createTestChartOfAccount(ctx context.Context, tx *sqlx.Tx, practitionerID uuid.UUID) *ChartOfAccount {
	return s.createTestChartOfAccountWithCode(ctx, tx, practitionerID, int16(500+len(uuid.New().String())))
}

func (s *COARepositoryTestSuite) createTestChartOfAccountWithCode(ctx context.Context, tx *sqlx.Tx, practitionerID uuid.UUID, code int16) *ChartOfAccount {
	chart := &ChartOfAccount{
		ID:             uuid.New(),
		PractitionerID: practitionerID,
		AccountTypeID:  1,
		AccountTaxID:   1,
		Code:           code,
		Name:           "Test COA " + uuid.New().String()[:8],
		Key:            GenerateKeyFromName("Test COA " + uuid.New().String()[:8]),
		IsSystem:       false,
		Classification: ClassificationOperatingExpense,
	}

	created, err := s.repo.CreateChartOfAccount(ctx, chart, tx)
	s.Require().NoError(err)
	s.Require().NotEqual(uuid.Nil, created.ID)
	return created
}

func (s *COARepositoryTestSuite) TestCreateChartOfAccount() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	practitionerID := uuid.New()
	created := s.createTestChartOfAccount(ctx, tx, practitionerID)

	s.Require().Contains(created.Name, "Test COA")
	s.Require().NotEqual(int16(0), created.Code)
	s.Require().Equal(practitionerID, created.PractitionerID)
}

func (s *COARepositoryTestSuite) TestGetChartOfAccount() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	practitionerID := uuid.New()
	created := s.createTestChartOfAccount(ctx, tx, practitionerID)
	testutil.CommitTx(s.T(), tx)

	fetched, err := s.repo.GetChartOfAccount(ctx, created.ID, practitionerID)
	s.Require().NoError(err)
	s.Require().Equal(created.ID, fetched.ID)
	s.Require().Equal(created.Name, fetched.Name)
}

func (s *COARepositoryTestSuite) TestUpdateChartOfAccount() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	practitionerID := uuid.New()
	created := s.createTestChartOfAccount(ctx, tx, practitionerID)
	testutil.CommitTx(s.T(), tx)

	tx2 := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx2)

	// Update name
	created.Name = "Updated COA"
	updated, err := s.repo.UpdateCharOfAccount(ctx, created)
	s.Require().NoError(err)
	s.Require().Equal("Updated COA", updated.Name)
}

func (s *COARepositoryTestSuite) TestDeleteChartOfAccount() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	practitionerID := uuid.New()
	created := s.createTestChartOfAccount(ctx, tx, practitionerID)
	testutil.CommitTx(s.T(), tx)

	// Delete
	err := s.repo.DeleteChartOfAccount(ctx, created.ID, practitionerID)
	s.Require().NoError(err)

	// Verify deleted (should not be found)
	_, err = s.repo.GetChartOfAccount(ctx, created.ID, practitionerID)
	s.Require().Error(err)
}

func (s *COARepositoryTestSuite) TestGetAccountType() {
	ctx := context.Background()

	// Account type ID 1 should exist as seed data
	accType, err := s.repo.GetAccountType(ctx, int16(1))
	s.Require().NoError(err)
	s.Require().NotNil(accType)
}

func (s *COARepositoryTestSuite) TestGetAccountTax() {
	ctx := context.Background()

	// Account tax ID 1 should exist as seed data
	accTax, err := s.repo.GetAccountTax(ctx, int16(1))
	s.Require().NoError(err)
	s.Require().NotNil(accTax)
}

func (s *COARepositoryTestSuite) TestListChartOfAccount() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	practitionerID := uuid.New()

	s.createTestChartOfAccountWithCode(ctx, tx, practitionerID, 501)
	s.createTestChartOfAccountWithCode(ctx, tx, practitionerID, 502)
	testutil.CommitTx(s.T(), tx)

	filter := &Filter{}
	list, err := s.repo.ListChartOfAccount(ctx, &practitionerID, "PRACTITIONER", filter.MapToFilter())
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(len(list), 2)
}

func (s *COARepositoryTestSuite) TestCheckCodeUnique() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	practitionerID := uuid.New()
	created := s.createTestChartOfAccount(ctx, tx, practitionerID)
	testutil.CommitTx(s.T(), tx)

	// Same code should exist
	existing, err := s.repo.GetChartByCodeAndPractitionerID(ctx, created.Code, practitionerID, nil)
	s.Require().NoError(err)
	s.Require().NotNil(existing)

	// Different code should not exist
	other, err := s.repo.GetChartByCodeAndPractitionerID(ctx, 9999, practitionerID, nil)
	s.Require().Error(err)
	s.Require().Nil(other)
}

func TestCOARepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(COARepositoryTestSuite))
}
