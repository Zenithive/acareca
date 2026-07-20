package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
	"github.com/stretchr/testify/suite"
)

type AuthRepositoryTestSuite struct {
	suite.Suite
	db   *sqlx.DB
	repo Repository
}

func (s *AuthRepositoryTestSuite) SetupSuite() {
	s.db = testutil.OpenTestDB(s.T())
	s.repo = NewRepository(s.db)
}

func (s *AuthRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
}

func (s *AuthRepositoryTestSuite) createTestUser(ctx context.Context, tx *sqlx.Tx) *User {
	email := "auth-test-" + uuid.NewString() + "@example.com"
	user, err := s.repo.CreateUser(ctx, &User{
		Email:     email,
		Password:  lo.ToPtr("secret"),
		FirstName: "Auth",
		LastName:  "Tester",
		Phone:     lo.ToPtr("1234567890"),
		Role:      "ADMIN",
	}, tx)
	s.Require().NoError(err)
	s.Require().NotEqual(uuid.Nil, user.ID)
	return user
}

func (s *AuthRepositoryTestSuite) createTestSession(ctx context.Context, userID uuid.UUID) *Session {
	refreshToken := "refresh-token-" + uuid.NewString()
	sessionID := uuid.New()
	session, err := s.repo.CreateSession(ctx, &Session{
		ID:           sessionID,
		UserID:       userID,
		RefreshToken: refreshToken,
		UserAgent:    lo.ToPtr("go-test"),
		IPAddress:    lo.ToPtr("127.0.0.1"),
		ExpiresAt:    time.Now().Add(2 * time.Hour),
	})
	s.Require().NoError(err)
	s.Require().Equal(sessionID, session.ID)
	return session
}

func (s *AuthRepositoryTestSuite) TestCreateUser() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	user := s.createTestUser(ctx, tx)
	testutil.CommitTx(s.T(), tx)

	s.Require().NotEqual(uuid.Nil, user.ID)
}

func (s *AuthRepositoryTestSuite) TestFindUserByEmailAndID() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	user := s.createTestUser(ctx, tx)
	testutil.CommitTx(s.T(), tx)

	foundByEmail, err := s.repo.FindByEmail(ctx, user.Email)
	s.Require().NoError(err)
	s.Require().Equal(user.Email, foundByEmail.Email)
	s.Require().Equal(user.ID, foundByEmail.ID)

	foundByID, err := s.repo.FindByID(ctx, user.ID)
	s.Require().NoError(err)
	s.Require().Equal(user.Email, foundByID.Email)
}

func (s *AuthRepositoryTestSuite) TestCreateAndDeleteSession() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	user := s.createTestUser(ctx, tx)
	testutil.CommitTx(s.T(), tx)

	session := s.createTestSession(ctx, user.ID)

	fetched, err := s.repo.FindSessionByRefreshToken(ctx, session.RefreshToken)
	s.Require().NoError(err)
	s.Require().Equal(session.ID, fetched.ID)

	err = s.repo.DeleteSession(ctx, session.ID)
	s.Require().NoError(err)

	_, err = s.repo.FindSessionByRefreshToken(ctx, session.RefreshToken)
	s.Require().Error(err)
}

func TestAuthRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(AuthRepositoryTestSuite))
}
