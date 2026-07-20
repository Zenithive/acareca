package fy

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"
)

type FYRepositoryTestSuite struct {
	suite.Suite
	db   *sqlx.DB
	repo Repository
}

func (s *FYRepositoryTestSuite) SetupSuite() {
	s.db = testutil.OpenTestDB(s.T())
	s.repo = NewRepository(s.db)
}

func (s *FYRepositoryTestSuite) SetupTest() {
	if s.db != nil {
		_, err := s.db.Exec("TRUNCATE TABLE tbl_financial_quarter, tbl_financial_year CASCADE")
		s.Require().NoError(err)
	}
}

func (s *FYRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
}

func (s *FYRepositoryTestSuite) createTestFinancialYearWithDates(ctx context.Context, tx *sqlx.Tx, label string, startYear int) *FinancialYear {
	uniqueLabel := label + " (" + uuid.NewString()[:8] + ")"

	fy := &FinancialYear{
		ID:        uuid.New(),
		Label:     uniqueLabel,
		IsActive:  false,
		StartDate: time.Date(startYear, 7, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(startYear+1, 6, 30, 23, 59, 59, 999999999, time.UTC),
	}

	created, err := s.repo.CreateFinancialYear(ctx, fy, tx)
	s.Require().NoError(err)
	s.Require().NotEqual(uuid.Nil, created.ID)
	return created
}

func (s *FYRepositoryTestSuite) createTestFinancialQuarter(ctx context.Context, tx *sqlx.Tx, fyID uuid.UUID, label string, startYear int) *FinancialQuarter {
	quarter := &FinancialQuarter{
		ID:              uuid.New(),
		FinancialYearID: fyID,
		Label:           label,
		StartDate:       time.Date(startYear, 7, 1, 0, 0, 0, 0, time.UTC),
		EndDate:         time.Date(startYear, 9, 30, 23, 59, 59, 999999999, time.UTC),
	}

	created, err := s.repo.CreateFinancialQuarter(ctx, quarter, tx)
	s.Require().NoError(err)
	s.Require().NotEqual(uuid.Nil, created.ID)
	return created
}

func (s *FYRepositoryTestSuite) TestCreateFinancialYear() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	fy := s.createTestFinancialYearWithDates(ctx, tx, "FY 2080-2081", 2080)

	s.Require().Contains(fy.Label, "FY 2080-2081")
	s.Require().False(fy.IsActive)
	s.Require().Equal(2080, fy.StartDate.Year())
}

func (s *FYRepositoryTestSuite) TestGetFinancialYearByID() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	created := s.createTestFinancialYearWithDates(ctx, tx, "FY 2081-2082", 2081)
	testutil.CommitTx(s.T(), tx)

	fetched, err := s.repo.GetFinancialYearByID(ctx, created.ID)
	s.Require().NoError(err)
	s.Require().Equal(created.ID, fetched.ID)
	s.Require().Equal(created.Label, fetched.Label)
}

func (s *FYRepositoryTestSuite) TestUpdateFinancialYear() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	created := s.createTestFinancialYearWithDates(ctx, tx, "FY 2082-2083", 2082)
	testutil.CommitTx(s.T(), tx)

	tx2 := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx2)

	created.Label = "Updated FY 2082-2083 (" + uuid.NewString()[:8] + ")"
	created.IsActive = true
	updated, err := s.repo.UpdateFinancialYear(ctx, created, tx2)
	s.Require().NoError(err)
	s.Require().Equal(created.Label, updated.Label)
	testutil.CommitTx(s.T(), tx2)
}

func (s *FYRepositoryTestSuite) TestListFinancialYears() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	s.createTestFinancialYearWithDates(ctx, tx, "FY 2083-2084", 2083)
	s.createTestFinancialYearWithDates(ctx, tx, "FY 2084-2085", 2084)
	testutil.CommitTx(s.T(), tx)

	list, err := s.repo.GetFinancialYears(ctx)
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(len(list), 2)
}

func (s *FYRepositoryTestSuite) TestCreateFinancialQuarter() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	fy := s.createTestFinancialYearWithDates(ctx, tx, "FY 2086-2087", 2086)
	quarter := s.createTestFinancialQuarter(ctx, tx, fy.ID, "Q1", 2086)

	s.Require().Contains(quarter.Label, "Q1")
	s.Require().Equal(fy.ID, quarter.FinancialYearID)
}

func (s *FYRepositoryTestSuite) TestGetFinancialQuartersByYear() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	fy := s.createTestFinancialYearWithDates(ctx, tx, "FY 2085-2086", 2085)
	s.createTestFinancialQuarter(ctx, tx, fy.ID, "Q1", 2085)
	s.createTestFinancialQuarter(ctx, tx, fy.ID, "Q2", 2085)
	testutil.CommitTx(s.T(), tx)

	quarters, err := s.repo.GetFinancialQuarters(ctx, fy.ID)
	s.Require().NoError(err)
	s.Require().Len(quarters, 2)
}

func (s *FYRepositoryTestSuite) TestDeactivateAllExcept() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	fy1 := s.createTestFinancialYearWithDates(ctx, tx, "FY 2088-2089", 2088)
	fy1.IsActive = true
	_, err := s.repo.UpdateFinancialYear(ctx, fy1, tx)
	s.Require().NoError(err)

	fy2 := s.createTestFinancialYearWithDates(ctx, tx, "FY 2089-2090", 2089)
	testutil.CommitTx(s.T(), tx)

	tx2 := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx2)

	err = s.repo.DeactivateAllFinancialYearsExcept(ctx, tx2, fy2.ID)
	s.Require().NoError(err)
	testutil.CommitTx(s.T(), tx2)
}

func (s *FYRepositoryTestSuite) TestDeleteQuartersByFYID() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	fy := s.createTestFinancialYearWithDates(ctx, tx, "FY 2090-2091", 2090)
	s.createTestFinancialQuarter(ctx, tx, fy.ID, "Q1", 2090)
	s.createTestFinancialQuarter(ctx, tx, fy.ID, "Q2", 2090)
	testutil.CommitTx(s.T(), tx)

	tx2 := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx2)

	err := s.repo.DeleteQuartersByFYID(ctx, fy.ID, tx2)
	s.Require().NoError(err)
	testutil.CommitTx(s.T(), tx2)

	quarters, err := s.repo.GetFinancialQuarters(ctx, fy.ID)
	s.Require().NoError(err)
	s.Require().Len(quarters, 0)
}

func TestFYRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(FYRepositoryTestSuite))
}
