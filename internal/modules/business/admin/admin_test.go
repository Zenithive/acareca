package admin

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
	"github.com/stretchr/testify/suite"
)

type AdminRepositoryTestSuite struct {
	suite.Suite
	db   *sqlx.DB
	repo Repository
}

func (s *AdminRepositoryTestSuite) SetupSuite() {
	s.db = testutil.OpenTestDB(s.T())
	s.repo = NewRepository(s.db)
}

func (s *AdminRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
}

func (s *AdminRepositoryTestSuite) createTestAdmin(ctx context.Context, tx *sqlx.Tx) (*Admin, *User) {
	user, err := s.repo.CreateUser(ctx, &User{
		Email:     "admin-test-" + uuid.NewString() + "@example.com",
		Password:  lo.ToPtr("adminpass"),
		FirstName: "Admin",
		LastName:  "Tester",
		Phone:     lo.ToPtr("0000000000"),
		Role:      "ADMIN",
	}, tx)
	s.Require().NoError(err)
	s.Require().NotEqual(uuid.Nil, user.ID)

	admin, err := s.repo.CreateAdmin(ctx, &Admin{UserID: user.ID}, tx)
	s.Require().NoError(err)
	s.Require().NotEqual(uuid.Nil, admin.ID)
	return admin, user
}

func (s *AdminRepositoryTestSuite) TestCreateAdmin() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	admin, _ := s.createTestAdmin(ctx, tx)
	testutil.CommitTx(s.T(), tx)

	s.Require().NotEqual(uuid.Nil, admin.ID)
}

func (s *AdminRepositoryTestSuite) TestFindAdminByUserID() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	admin, _ := s.createTestAdmin(ctx, tx)
	testutil.CommitTx(s.T(), tx)

	found, err := s.repo.FindByUserID(ctx, admin.UserID)
	s.Require().NoError(err)
	s.Require().Equal(admin.ID, found.ID)
}

func (s *AdminRepositoryTestSuite) TestFindAdminByID() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	admin, user := s.createTestAdmin(ctx, tx)
	testutil.CommitTx(s.T(), tx)

	found, err := s.repo.FindByID(ctx, admin.ID)
	s.Require().NoError(err)
	s.Require().Equal(user.Email, found.User.Email)
}

func TestAdminRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(AdminRepositoryTestSuite))
}
