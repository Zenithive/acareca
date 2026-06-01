package notification

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"
)

type NotificationRepositoryTestSuite struct {
	suite.Suite
	db   *sqlx.DB
	repo Repository
}

func (s *NotificationRepositoryTestSuite) SetupSuite() {
	s.db = testutil.OpenTestDB(s.T())
	s.repo = NewRepository(s.db)
}

func (s *NotificationRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
}

func (s *NotificationRepositoryTestSuite) createTestNotification(ctx context.Context, tx *sqlx.Tx, recipientID, entityID uuid.UUID) uuid.UUID {
	payload := json.RawMessage(`{"message":"test notification"}`)

	notificationID := uuid.New()
	notification := Notification{
		ID:            notificationID,
		RecipientID:   recipientID,
		RecipientType: ActorPractitioner,
		EventType:     EventSystemError,
		EntityType:    EntitySystem,
		EntityID:      entityID,
		Status:        StatusUnread,
		Payload:       payload,
		CreatedAt:     time.Now().UTC(),
	}

	id, err := s.repo.CreateNotificationWithDeliveries(ctx, tx, notification, []Channel{ChannelInApp, ChannelEmail})
	s.Require().NoError(err)
	s.Require().Equal(notificationID, id)
	return notificationID
}

func (s *NotificationRepositoryTestSuite) TestCreateNotificationWithDeliveries() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	recipientID := uuid.New()
	entityID := uuid.New()
	notificationID := s.createTestNotification(ctx, tx, recipientID, entityID)
	testutil.CommitTx(s.T(), tx)

	s.Require().NotEqual(uuid.Nil, notificationID)
}

func (s *NotificationRepositoryTestSuite) TestListNotificationByRecipient() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	recipientID := uuid.New()
	entityID := uuid.New()
	s.createTestNotification(ctx, tx, recipientID, entityID)
	testutil.CommitTx(s.T(), tx)

	list, total, err := s.repo.ListByRecipient(ctx, recipientID, FilterNotification{})
	s.Require().NoError(err)
	s.Require().Equal(1, total)
	s.Require().Len(list, 1)
}

func (s *NotificationRepositoryTestSuite) TestMarkNotificationRead() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	recipientID := uuid.New()
	entityID := uuid.New()
	notificationID := s.createTestNotification(ctx, tx, recipientID, entityID)
	testutil.CommitTx(s.T(), tx)

	err := s.repo.MarkRead(ctx, notificationID, recipientID)
	s.Require().NoError(err)

	active, err := s.repo.HasActiveSystemNotification(ctx, entityID, EventSystemError)
	s.Require().NoError(err)
	s.Require().False(active)
}

func (s *NotificationRepositoryTestSuite) TestCreateAndGetNotificationPreference() {
	ctx := context.Background()

	recipientID := uuid.New()
	entityID := uuid.New()
	preference := NotificationPreference{
		ID:         uuid.New(),
		UserID:     recipientID,
		EntityID:   entityID,
		EntityType: string(EntitySystem),
		EventType:  EventSystemActivityAlert,
		Channels: NotificationChannels{
			string(ChannelInApp): true,
		},
	}

	err := s.repo.CreatePreference(ctx, preference)
	s.Require().NoError(err)

	prefs, err := s.repo.GetAllPreferences(ctx, recipientID)
	s.Require().NoError(err)
	s.Require().NotEmpty(prefs)
	s.Require().Equal(recipientID, prefs[0].UserID)
}

func TestNotificationRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(NotificationRepositoryTestSuite))
}
