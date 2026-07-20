package invitation

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"
)

type InvitationRepositoryTestSuite struct {
	suite.Suite
	db   *sqlx.DB
	repo Repository
}

func (s *InvitationRepositoryTestSuite) SetupSuite() {
	s.db = testutil.OpenTestDB(s.T())
	s.repo = NewRepository(s.db)
}

func (s *InvitationRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
}

func (s *InvitationRepositoryTestSuite) createTestInvitation(ctx context.Context, tx *sqlx.Tx) (uuid.UUID, uuid.UUID, string, uuid.UUID) {
	userID := uuid.New()
	_, err := tx.ExecContext(ctx,
		`INSERT INTO tbl_user (id, email, password, first_name, last_name, phone, role) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		userID,
		"inviter-test-"+uuid.NewString()+"@example.com",
		"password",
		"Invitation",
		"Creator",
		"4443332222",
		"PRACTITIONER",
	)
	s.Require().NoError(err)

	practitionerID := uuid.New()
	_, err = tx.ExecContext(ctx,
		`INSERT INTO tbl_practitioner (id, user_id, entity_type, entity_name, abn, acn, address, profession) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		practitionerID,
		userID,
		"SOLE_TRADER",
		"Invitation Practitioner",
		nil,
		nil,
		nil,
		nil,
	)
	s.Require().NoError(err)

	accountantUserID := uuid.New()
	accountantEmail := "accountant-test-" + uuid.NewString() + "@example.com"
	_, err = tx.ExecContext(ctx,
		`INSERT INTO tbl_user (id, email, password, first_name, last_name, phone, role) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		accountantUserID,
		accountantEmail,
		"password",
		"Accountant",
		"Invite",
		"5556667777",
		"ACCOUNTANT",
	)
	s.Require().NoError(err)

	accountantID := uuid.New()
	_, err = tx.ExecContext(ctx,
		`INSERT INTO tbl_accountant (id, user_id, entity_type, entity_name, abn, acn, address, tax_agent_number, profession) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		accountantID,
		accountantUserID,
		"SOLE_TRADER",
		"Invitation Accountant",
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	s.Require().NoError(err)

	invitationID := uuid.New()
	invitation := &Invitation{
		ID:             invitationID,
		PractitionerID: practitionerID,
		AccountantID:   &accountantID,
		Email:          accountantEmail,
		Status:         StatusSent,
		ExpiresAt:      time.Now().Add(72 * time.Hour),
	}
	err = s.repo.Create(ctx, tx, invitation)
	s.Require().NoError(err)
	return invitationID, accountantID, accountantEmail, practitionerID
}

func (s *InvitationRepositoryTestSuite) TestCreateInvitation() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	invitationID, _, _, _ := s.createTestInvitation(ctx, tx)
	testutil.CommitTx(s.T(), tx)

	fetched, err := s.repo.GetByID(ctx, invitationID)
	s.Require().NoError(err)
	s.Require().NotNil(fetched)
}

func (s *InvitationRepositoryTestSuite) TestGetInvitationByEmail() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	_, _, email, _ := s.createTestInvitation(ctx, tx)
	testutil.CommitTx(s.T(), tx)

	byEmail, err := s.repo.GetByEmail(ctx, email)
	s.Require().NoError(err)
	s.Require().NotNil(byEmail)
}

func (s *InvitationRepositoryTestSuite) TestUpdateInvitationStatus() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	invitationID, accountantID, email, practitionerID := s.createTestInvitation(ctx, tx)
	testutil.CommitTx(s.T(), tx)

	tx2 := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx2)
	err := s.repo.UpdateStatus(ctx, tx2, invitationID, StatusAccepted, &accountantID, time.Now().Add(48*time.Hour))
	s.Require().NoError(err)
	testutil.CommitTx(s.T(), tx2)

	isLinked, err := s.repo.IsAccountantLinkedToPractitioner(ctx, practitionerID, accountantID)
	s.Require().NoError(err)
	s.Require().True(isLinked)

	foundAccountantID, err := s.repo.GetAccountantIDByEmail(ctx, email)
	s.Require().NoError(err)
	s.Require().NotNil(foundAccountantID)
	s.Require().Equal(accountantID, *foundAccountantID)
}

func TestInvitationRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(InvitationRepositoryTestSuite))
}
