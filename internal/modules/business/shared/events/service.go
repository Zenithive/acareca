package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
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
		// SYSTEM ERROR: Failed to log Accountant activity
		if s.auditSvc != nil {
			s.auditSvc.LogSystemIssue(
				ctx,
				auditctx.ActionSystemError,
				"events.shared_event_record_failed",
				fmt.Errorf("failed to save shared event: %w", err),
				e.AccountantID.String(),
				e.EntityID.String(),
				string(e.EntityType),
				auditctx.ModuleBusiness,
			)
		}
		return err
	}

	if s.notification != nil {

		notificationReq := s.mapToNotificationRequest(e)

		// Use a goroutine to publish so it doesn't slow down the main request
		go func() {
			// Create a background context for the async task
			asyncCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := s.notification.Publish(asyncCtx, notificationReq); err != nil {

				fmt.Printf(">>> ERROR: Failed to publish notification: %v\n", err)
			}
		}()
	}

	if s.auditSvc != nil {
		meta := auditctx.GetMetadata(ctx)
		userIDStr := e.ActorID.String()
		auditEntry := &audit.LogEntry{
			PracticeID: meta.PracticeID,
			UserID:     &userIDStr,
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

func (s *service) mapToNotificationRequest(e SharedEvent) notification.RqNotification {

	payloadObj := notification.BuildNotificationPayload(
		"Accountant Activity Alert",
		json.RawMessage(fmt.Sprintf(`"%s"`, e.Description)),
		e.ActorName, // senderName
		nil,
		nil,
	)

	payloadBytes, _ := json.Marshal(payloadObj)

	// 2. Map SharedEvent to RqNotification
	senderType := notification.ActorAccountant

	return notification.RqNotification{
		ID:            uuid.New(),
		RecipientID:   e.PractitionerID,
		RecipientType: notification.ActorPractitioner,
		SenderID:      &e.AccountantID,
		SenderType:    &senderType,
		EventType:     notification.EventType(e.EventType),
		EntityType:    notification.EntityType(e.EntityType),
		EntityID:      e.EntityID,
		Payload:       payloadBytes,
		Channels:      []notification.Channel{notification.ChannelInApp},
		CreatedAt:     time.Now(),
	}
}
