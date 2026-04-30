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
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type Service interface {
	Log(ctx context.Context, entry *LogEntry) error
	LogAsync(entry *LogEntry)
	LogSystemIssue(ctx context.Context, level, action string, issueErr error, actorID, entityID, entityType, module string)
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

// LogSystemIssue records a system-level error or warning to the audit log and notifies all admins.
func (s *service) LogSystemIssue(ctx context.Context, level, action string, issueErr error, actorID, entityID, entityType, module string) {
	if issueErr == nil {
		return
	}
	// Set Defaults
	if ctx == nil {
		ctx = context.Background()
	}
	if module == "" {
		module = auditctx.ModuleSystem
	}

	// Resolve Names (Single place for lookups)
	resolveCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	actorName := s.repo.ResolveActorName(resolveCtx, actorID)
	entityLabel := s.repo.ResolveEntityLabel(resolveCtx, entityType, entityID)

	// Build the HUMAN-READABLE message
	detail := buildSystemIssueMessage(level, action, actorName, entityLabel, issueErr)

	// Determine notification event type
	var eventType notification.EventType
	if level == auditctx.ActionSystemError {
		eventType = notification.EventSystemError
	} else {
		eventType = notification.EventSystemWarning
	}

	// Deduplication Check (Prevent spamming admins with the same error)
	parsedEntityID, _ := uuid.Parse(entityID)
	if s.notificationService != nil && parsedEntityID != uuid.Nil {
		if exists, _ := s.repo.HasActiveSystemNotification(resolveCtx, parsedEntityID, eventType); exists {
			return
		}
	}

	// Audit Log Entry
	entry := &LogEntry{
		Action:     level,
		Module:     module,
		EntityType: &entityType,
		EntityID:   &entityID,
		AfterState: map[string]interface{}{
			"summary":   detail,
			"raw_error": issueErr.Error(),
		},
	}

	_ = s.repo.Insert(ctx, entry)

	// Notify Admins
	if s.notificationService != nil {
		eventType := notification.EventSystemWarning
		if level == auditctx.ActionSystemError {
			eventType = notification.EventSystemError
		}
		// Send the clean 'detail' string as the notification body
		go s.publishSystemIssueNotification(level, action, detail, parsedEntityID, eventType)
	}
}

// buildSystemIssueMessage produces the single human-readable string shown on the admin panel.
func buildSystemIssueMessage(level, action, actorName, entityName string, err error) string {
	reason := err.Error()

	if strings.Contains(reason, "attempted to access") {
		return fmt.Sprintf("'%s' attempted to access %s '%s' they do not own",
			actorName,
			strings.ToLower(strings.Split(action, ".")[0]),
			entityName,
		)
	}

	return fmt.Sprintf("'%s' '%s' '%s': %v",
		actorName, action, entityName, err)
}

// publishSystemIssueNotification fans out system error/warning notifications to all admins.
func (s *service) publishSystemIssueNotification(level, action, detail string, entityID uuid.UUID, eventType notification.EventType) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	adminIDs, err := s.repo.GetAdminIDs(ctx)
	if err != nil || len(adminIDs) == 0 {
		return
	}

	title := "System Warning"
	if level == auditctx.ActionSystemError {
		title = "System Error"
	}

	body, _ := json.Marshal(detail)
	extraData := map[string]interface{}{"action": action}
	notifPayload := notification.BuildNotificationPayload(title, body, nil, nil, &extraData)
	payloadBytes, err := json.Marshal(notifPayload)
	if err != nil {
		log.Printf("ERROR: [SystemIssue] marshal payload: %v", err)
		return
	}

	senderType := notification.ActorSystem
	for _, adminID := range adminIDs {
		req := notification.RqNotification{
			ID:            uuid.New(),
			RecipientID:   adminID,
			RecipientType: notification.ActorAdmin,
			SenderType:    &senderType,
			EventType:     eventType,
			EntityType:    notification.EntitySystem,
			EntityID:      entityID,
			Status:        notification.StatusUnread,
			Payload:       payloadBytes,
			Channels:      []notification.Channel{notification.ChannelInApp},
			CreatedAt:     time.Now(),
		}
		go func(r notification.RqNotification) {
			pCtx, pCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer pCancel()
			if err := s.notificationService.Publish(pCtx, r); err != nil {
				log.Printf("ERROR: [SystemIssue] publish to admin %s: %v", r.RecipientID, err)
			}
		}(req)
	}
}

// This runs in its own goroutine to avoid blocking the audit worker
// publishAuditLogNotification sends notifications to all admin users about a new audit log
func (s *service) publishAuditLogNotification(entry *LogEntry) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	// Fetch Admins
	adminIDs, err := s.repo.GetAdminIDs(ctx)
	if err != nil || len(adminIDs) == 0 {
		return
	}

	// Get User Name
	userName := "User"

	// Define invitation-specific actions
	isInvitationAction := entry.Action == "accountant.invite_accepted" ||
		entry.Action == "accountant.invite_rejected" ||
		entry.Action == "accountant.invite_expired" ||
		entry.Action == "accountant.invite_completed"

	if isInvitationAction && entry.EntityID != nil {
		// Try to fetch email from tbl_invitation to identify the accountant
		email, err := s.repo.GetInvitationEmail(ctx, *entry.EntityID)
		if err == nil && email != "" {
			userName = email
		} else if entry.UserID != nil {
			// Fallback to UserID if invitation lookup fails
			userName, _ = s.repo.GetUserName(ctx, *entry.UserID)
		}
	} else if entry.UserID != nil {
		if entry.Action == "shared_event.recorded" {
			userName, err = s.repo.GetAccountantNameForSharedEvents(ctx, *entry.UserID)
			fmt.Printf("Accountant Name for shared events: %s", userName)
		} else {
			userName, err = s.repo.GetUserName(ctx, *entry.UserID)
		}
		if err != nil {
			log.Printf("ERROR: [Audit-Notification] Failed to get user name for ID %s: %v", *entry.UserID, err)
		}
	} else if entry.PracticeID != nil {
		// Fallback for system/practice level actions
		uID, err := s.repo.GetUserIDByPractitionerID(ctx, *entry.PracticeID)
		if err == nil {
			userName, _ = s.repo.GetUserName(ctx, uID)
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

		// Billing & Subscriptions (Success)
		"subscription.created":          "created a new subscription plan",
		"subscription.updated":          "updated subscription plan",
		"subscription.deleted":          "deleted subscription plan",
		"billing.payment_success":       "successfully processed payment for",
		"billing.activation_successful": "successfully activated subscription for",

		// Report Generate and Export
		"bas_report.exported":          "exported BAS Report",
		"pl_report.exported":           "exported Profit and Loss Report",
		"activity_statement.exported":  "exported Activity Statement",
		"transactions.exported":        "exported Transactions",
		"bas_report.generated":         "generated BAS Report",
		"pl_report.generated":          "generated Profit and Loss Report",
		"activity_statement.generated": "generated Activity Statement",
		"transactions.generated":       "generated Transactions",
	}
	formattedAction, exists := actionVerbs[entry.Action]
	if !exists {
		// Fallback: Replace dots/underscores and title case it
		formattedAction = strings.NewReplacer(".", " ", "_", " ").Replace(entry.Action)
	}

	// Get Entity Name
	var entityNamePtr *string
	entityName := ""
	if entry.EntityType != nil && entry.EntityID != nil {
		entityNamePtr, err = s.repo.GetEntityName(ctx, *entry.EntityType, *entry.EntityID)

		// If the pointer is not nil, use the value
		if entityNamePtr != nil {
			entityName = *entityNamePtr
		}
	}

	title := "System Activity Alert"
	message := fmt.Sprintf("%s %s %s", userName, formattedAction, entityName)
	message = strings.TrimSpace(message) // Clean up trailing spaces if name was nil

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
