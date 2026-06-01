package accountant

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
	"github.com/stretchr/testify/suite"
)

type AccountantRepositoryTestSuite struct {
	suite.Suite
	db   *sqlx.DB
	repo Repository
}

func (s *AccountantRepositoryTestSuite) SetupSuite() {
	s.db = testutil.OpenTestDB(s.T())
	s.repo = NewRepository(s.db)
}

func (s *AccountantRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
}

func (s *AccountantRepositoryTestSuite) createTestAccountant(ctx context.Context, tx *sqlx.Tx) *RsAccountant {
	userID := uuid.New()
	_, err := tx.ExecContext(ctx,
		`INSERT INTO tbl_user (id, email, password, first_name, last_name, phone, role) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		userID,
		"accountant-test-"+uuid.NewString()+"@example.com",
		"password",
		"Accountant",
		"Tester",
		"1112223333",
		"ACCOUNTANT",
	)
	s.Require().NoError(err)

	entityName := lo.ToPtr("Test Accountant")
	account, err := s.repo.CreateAccountant(ctx, &RqCreateAccountant{
		UserID:         userID.String(),
		EntityType:     "SOLE_TRADER",
		EntityName:     entityName,
		ABN:            nil,
		ACN:            nil,
		Address:        nil,
		TaxAgentNumber: nil,
		Profession:     nil,
	}, tx)
	s.Require().NoError(err)
	s.Require().NotEqual(uuid.Nil, account.ID)
	return account
}

func (s *AccountantRepositoryTestSuite) TestCreateAccountant() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	account := s.createTestAccountant(ctx, tx)
	testutil.CommitTx(s.T(), tx)

	s.Require().NotEqual(uuid.Nil, account.ID)
}

func (s *AccountantRepositoryTestSuite) TestGetAccountantByUserID() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	account := s.createTestAccountant(ctx, tx)
	testutil.CommitTx(s.T(), tx)

	found, err := s.repo.GetAccountantByUserID(ctx, account.UserID)
	s.Require().NoError(err)
	s.Require().Equal(account.ID, found.ID)
}

func TestAccountantRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(AccountantRepositoryTestSuite))
}
