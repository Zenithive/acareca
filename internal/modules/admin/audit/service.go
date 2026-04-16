package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/notification"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type Service interface {
	Log(ctx context.Context, entry *LogEntry) error
	LogAsync(entry *LogEntry)
	Query(ctx context.Context, f *Filter) (*util.RsList, error)
	GetByID(ctx context.Context, id string) (*RsAuditLog, error)
	Shutdown()
}

type service struct {
	repo                Repository
	logChan             chan *LogEntry
	done                chan struct{}
	notificationService notification.Service
}

func NewService(repo Repository, notificationService notification.Service) Service {
	s := &service{
		repo:                repo,
		logChan:             make(chan *LogEntry, 1000),
		done:                make(chan struct{}),
		notificationService: notificationService,
	}

	// Start async worker
	go s.asyncWorker()

	return s
}

// Log synchronously writes an audit log entry
func (s *service) Log(ctx context.Context, entry *LogEntry) error {
	return s.repo.Insert(ctx, entry)
}

// LogAsync queues an audit log entry for async processing
// This prevents audit logging from blocking main operations
func (s *service) LogAsync(entry *LogEntry) {
	select {
	case s.logChan <- entry:
		// Successfully queued
	default:
		// Channel full, log error but don't block
		log.Printf("WARN: audit log channel full, dropping entry: %s.%s", entry.Module, entry.Action)
	}
}

// asyncWorker processes audit log entries from the queue
func (s *service) asyncWorker() {
	for entry := range s.logChan {
		ctx := context.Background()
		if err := s.repo.Insert(ctx, entry); err != nil {
			// Log error but continue processing
			log.Printf("ERROR: failed to insert audit log: %v (action: %s.%s)", err, entry.Module, entry.Action)
			continue
		}

		// Trigger notification to admins asynchronously (in a separate goroutine)
		// This ensures the audit log is saved even if notification sending fails
		if s.notificationService != nil {
			go s.publishAuditLogNotification(entry)
		}
	}
	close(s.done)
}

// This runs in its own goroutine to avoid blocking the audit worker
// publishAuditLogNotification sends notifications to all admin users about a new audit log
func (s *service) publishAuditLogNotification(entry *LogEntry) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Fetch Admins
	adminIDs, err := s.repo.GetAdminIDs(ctx)
	if err != nil || len(adminIDs) == 0 {
		return
	}

	// Get User Name
	userName := "User"
	if entry.UserID != nil {
		userName, err = s.repo.GetUserName(ctx, *entry.UserID)
		if err != nil {
			log.Printf("ERROR: [Audit-Notification] Failed to get user name for ID %s: %v", *entry.UserID, err)
		}
	} else {
		uID, err := s.repo.GetUserIDByPractitionerID(ctx, *entry.PracticeID)
		if err != nil {
			log.Printf("ERROR: [Audit-Notification] Failed to get user ID for practitioner ID %s: %v", *entry.PracticeID, err)
		} else {
			userName, err = s.repo.GetUserName(ctx, uID)
			if err != nil {
				log.Printf("ERROR: [Audit-Notification] Failed to get user name for ID %s: %v", uID, err)
			}
		}

	}

	// Format the Action into a generic sentence
	var actionVerbs = map[string]string{
		// Auth
		"user.password_reset":   "reset their password",
		"user.password_changed": "changed their password",
		"session.created":       "started a new session",
		"user.oauth_linked":     "linked their OAuth account",

		//Admin
		"permission.granted": "granted permissions",
		"permission.revoked": "revoked permissions",

		// Business
		"clinic.created":  "created clinic",
		"clinic.updated":  "updated clinic",
		"clinic.deleted":  "deleted clinic",
		"form.created":    "created form",
		"form.updated":    "updated form",
		"form.deleted":    "deleted form",
		"entry.created":   "created entry",
		"entry.updated":   "updated entry",
		"entry.deleted":   "deleted entry",
		"entry.confirmed": "confirmed entry",
		"coa.created":     "created Chart of Accounts",
		"coa.updated":     "updated Chart of Accounts",
		"coa.deleted":     "deleted Chart of Accounts",
		"fy.updated":      "updated Financial Year",
		"fy.closed":       "closed Financial Year",

		// Permissions / Invites
		"accountant.invite_sent":      "sent an invitation to accountant",
		"accountant.invite_accepted":  "accepted an invitation from practitioner",
		"accountant.invite_rejected":  "rejected an invitation from practitioner",
		"accountant.invite_completed": "completed registration after accepting invitation from practitioner",
		"accountant.invite_expired":   "invitation expired",
		"accountant.invite_revoked":   "revoked invitation for accountant",
		"invite.permission_assigned":  "assigned permissions to accountant",
		"invite.permission_updated":   "updated permissions for accountant",
	}
	formattedAction, exists := actionVerbs[entry.Action]
	if !exists {
		// Fallback: Replace dots/underscores and title case it
		formattedAction = strings.NewReplacer(".", " ", "_", " ").Replace(entry.Action)
	}

	// Get Entity Name
	entityName := ""
	if entry.EntityType != nil && entry.EntityID != nil {
		// entityContext = *entry.EntityType
		entityName, err = s.repo.GetEntityName(ctx, *entry.EntityType, *entry.EntityID)
		if err != nil {
			log.Printf("ERROR: [Audit-Notification] Failed to get entity name for ID %s: %v", *entry.EntityID, err)
		}
	}

	title := "System Activity Alert"
	message := fmt.Sprintf("%s %s %s", userName, formattedAction, entityName)

	// Construct Payload
	extraData := map[string]interface{}{
		"module":    entry.Module,
		"action":    entry.Action,
		"entity_id": entry.EntityID,
	}

	// Simplification: Direct JSON message for the body
	msgJson, _ := json.Marshal(message)

	notifPayload := notification.BuildNotificationPayload(
		title,
		msgJson,
		nil,
		nil,
		&extraData,
	)

	payloadBytes, err := json.Marshal(notifPayload)
	if err != nil {
		log.Printf("ERROR: [Audit-Notification] Marshal failed: %v", err)
		return
	}

	// Send to each admin
	senderType := notification.ActorSystem
	var entityID uuid.UUID
	if entry.EntityID != nil {
		parsed, err := uuid.Parse(*entry.EntityID)
		if err == nil {
			entityID = parsed
		}
	}
	for _, adminID := range adminIDs {
		req := notification.RqNotification{
			ID:            uuid.New(),
			RecipientID:   adminID,
			RecipientType: notification.ActorAdmin,
			SenderType:    &senderType,
			EventType:     notification.EventAuditLogCreated,
			EntityType:    notification.EntityAuditLog,
			EntityID:      entityID,
			Status:        notification.StatusUnread,
			Payload:       payloadBytes,
			Channels:      []notification.Channel{notification.ChannelInApp},
			CreatedAt:     time.Now(),
		}

		// Using a closure to capture 'req' correctly in goroutine
		go func(r notification.RqNotification) {
			pCtx, pCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer pCancel()
			if err := s.notificationService.Publish(pCtx, r); err != nil {
				log.Printf("ERROR: [Audit-Notification] Publish failed for admin %s: %v", r.RecipientID, err)
			}
		}(req)
	}
}

// Shutdown drains the log channel and waits for the worker to finish.
func (s *service) Shutdown() {
	close(s.logChan)
	<-s.done
}

// Query retrieves audit logs based on filter parameters
func (s *service) Query(ctx context.Context, f *Filter) (*util.RsList, error) {
	ft := f.MapToFilter()

	// Fetch data
	list, err := s.repo.List(ctx, ft)
	if err != nil {
		return nil, err
	}

	// Fetch total count for pagination
	total, err := s.repo.Count(ctx, ft)
	if err != nil {
		return nil, err
	}

	data := make([]*RsAuditLog, 0, len(list))
	for _, item := range list {
		data = append(data, item.ToRs())
	}

	var rsList util.RsList
	rsList.MapToList(data, total, *ft.Offset, *ft.Limit)

	return &rsList, nil
}

// GetByID retrieves a specific audit log entry
func (s *service) GetByID(ctx context.Context, id string) (*RsAuditLog, error) {
	l, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toRsAuditLog(l), nil
}
