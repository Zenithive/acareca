package billing

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"
)

type BillingRepositoryTestSuite struct {
	suite.Suite
	db   *sqlx.DB
	repo Repository
}

func (s *BillingRepositoryTestSuite) SetupSuite() {
	s.db = testutil.OpenTestDB(s.T())
	s.repo = NewRepository(s.db)
}

func (s *BillingRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
}

func (s *BillingRepositoryTestSuite) createTestPractitioner(ctx context.Context) uuid.UUID {
	userID := uuid.New()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO tbl_user (id, email, password, first_name, last_name, phone, role) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		userID,
		"billing-test-"+uuid.NewString()+"@example.com",
		"password",
		"Billing",
		"Tester",
		"9998887777",
		"PRACTITIONER",
	)
	s.Require().NoError(err)

	practitionerID := uuid.New()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO tbl_practitioner (id, user_id, entity_type, entity_name, abn, acn, address, profession) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		practitionerID,
		userID,
		"SOLE_TRADER",
		"Billing Practitioner",
		nil,
		nil,
		nil,
		nil,
	)
	s.Require().NoError(err)

	return practitionerID
}

func (s *BillingRepositoryTestSuite) TestGetPractitionerWithStripe() {
	ctx := context.Background()
	practitionerID := s.createTestPractitioner(ctx)

	result, err := s.repo.GetPractitionerWithStripe(ctx, practitionerID)
	s.Require().NoError(err)
	s.Require().Equal(practitionerID, result.ID)
	s.Require().Equal("Billing Tester", result.FirstName+" "+result.LastName)
}

func (s *BillingRepositoryTestSuite) TestListActiveSubscriptionsAndGetSubscriptionWithStripe() {
	ctx := context.Background()

	subscriptions, err := s.repo.ListActiveSubscriptions(ctx)
	s.Require().NoError(err)
	if len(subscriptions) == 0 {
		s.T().Skip("no active subscriptions available")
	}

	for _, subscription := range subscriptions {
		fetched, err := s.repo.GetSubscriptionWithStripe(ctx, subscription.ID)
		s.Require().NoError(err)
		s.Require().Equal(subscription.ID, fetched.ID)
		s.Require().Equal(subscription.Name, fetched.Name)
	}
}

func TestBillingRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(BillingRepositoryTestSuite))
}
