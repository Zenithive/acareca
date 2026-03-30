package events

import (
	"context"

	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"

	"github.com/iamarpitzala/acareca/internal/modules/notification"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
)

type Service interface {
	Record(ctx context.Context, e SharedEvent) error
}

type service struct {
	repo         Repository
	notification notification.Service
	auditSvc     audit.Service
}

func NewService(repo Repository, n notification.Service, a audit.Service) Service {
	return &service{repo: repo, notification: n, auditSvc: a}
}

func (s *service) Record(ctx context.Context, e SharedEvent) error {

	if err := s.repo.Save(ctx, e); err != nil {
		return err
	}

	// 2. TRIGGER NOTIFICATION (User Alert)
	if s.notification != nil {
		// TODO: Implement notification publishing using e.Description, e.Metadata
	}

	if s.auditSvc != nil {
		meta := auditctx.GetMetadata(ctx)
		auditEntry := &audit.LogEntry{
			PracticeID: meta.PracticeID,
			UserID:     meta.UserID,
			Action:     "shared_event.recorded",
			Module:     "shared_events",
			EntityType: strPtr("shared_event"),
			EntityID:   strPtr(e.ID.String()),
			//BeforeState: nil,
			//AfterState: e,
			IPAddress: meta.IPAddress,
			UserAgent: meta.UserAgent,
		}
		s.auditSvc.LogAsync(auditEntry)
	}

	return nil
}

func strPtr(s string) *string {
	return &s
}
