package audit

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"
)

type AuditRepositoryTestSuite struct {
	suite.Suite
	db   *sqlx.DB
	repo Repository
}

func (s *AuditRepositoryTestSuite) SetupSuite() {
	s.db = testutil.OpenTestDB(s.T())
	s.repo = NewRepository(s.db)
}

func (s *AuditRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
}

func (s *AuditRepositoryTestSuite) createTestAuditEntry(ctx context.Context) *LogEntry {
	message := map[string]string{"status": "created"}
	afterState, err := json.Marshal(message)
	s.Require().NoError(err)

	entry := &LogEntry{
		PracticeID: nil,
		UserID:     nil,
		Action:     "CREATE_RESOURCE",
		Module:     "TESTING",
		EntityType: nil,
		EntityID:   nil,
		BeforeState: map[string]string{
			"state": "before",
		},
		AfterState: afterState,
		IPAddress:  nil,
		UserAgent:  nil,
	}

	err = s.repo.Insert(ctx, entry)
	s.Require().NoError(err)
	return entry
}

func (s *AuditRepositoryTestSuite) TestInsertAuditLog() {
	ctx := context.Background()
	entry := s.createTestAuditEntry(ctx)
	s.Require().NotNil(entry)
}

func (s *AuditRepositoryTestSuite) TestListAuditLogs() {
	ctx := context.Background()
	s.createTestAuditEntry(ctx)

	logs, err := s.repo.List(ctx, common.Filter{Search: nil})
	s.Require().NoError(err)
	s.Require().NotEmpty(logs)
}

func (s *AuditRepositoryTestSuite) TestGetAuditLogByID() {
	ctx := context.Background()
	s.createTestAuditEntry(ctx)

	logs, err := s.repo.List(ctx, common.Filter{Search: nil})
	s.Require().NoError(err)
	s.Require().NotEmpty(logs)

	found, err := s.repo.GetByID(ctx, logs[0].ID)
	s.Require().NoError(err)
	s.Require().Equal(logs[0].ID, found.ID)
}

func (s *AuditRepositoryTestSuite) TestCountAuditLogs() {
	ctx := context.Background()
	s.createTestAuditEntry(ctx)

	count, err := s.repo.Count(ctx, common.Filter{})
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(count, 1)
}

func TestAuditRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(AuditRepositoryTestSuite))
}
