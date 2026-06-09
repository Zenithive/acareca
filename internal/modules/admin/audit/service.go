package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/business/admin"
	"github.com/iamarpitzala/acareca/internal/modules/notification"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type Service interface {
	Log(ctx context.Context, entry *LogEntry) error
	LogAsync(ctx context.Context, entry *LogEntry)
	LogSystemIssue(ctx context.Context, level, action string, issueErr error, actorID, entityID, entityType, module string)
	LogWithNotification(ctx context.Context, entry *LogEntry, notifyAdmins bool) error
	Query(ctx context.Context, f *Filter) (*util.RsList, error)
	GetByID(ctx context.Context, id string) (*RsAuditLog, error)
	Shutdown()
}

type service struct {
	repo                Repository
	logChan             chan *logJob
	done                chan struct{}
	notificationService notification.Service
	notificationPub     *sharednotification.Publisher
	adminRepo           admin.Repository
}

type logJob struct {
	ctx   context.Context
	entry *LogEntry
}

func NewService(repo Repository, notificationService notification.Service, adminRepo admin.Repository) Service {
	s := &service{
		repo:                repo,
		logChan:             make(chan *logJob, 1000),
		done:                make(chan struct{}),
		notificationService: notificationService,
		notificationPub:     sharednotification.NewPublisher(notification.NewServiceAdapter(notificationService), nil),
		adminRepo:           adminRepo,
	}

	// Start async worker
	go s.asyncWorker()

	return s
}

func (s *service) Log(ctx context.Context, entry *LogEntry) error {
	s.enrichEntry(ctx, entry)
	return s.repo.Insert(ctx, entry)
}

func (s *service) LogAsync(ctx context.Context, entry *LogEntry) {
	s.enrichEntry(ctx, entry)
	select {
	case s.logChan <- &logJob{ctx: context.Background(), entry: entry}:
	default:
		log.Printf("WARN: audit log channel full, dropping entry: %s.%s", entry.Module, entry.Action)
	}
}

func (s *service) LogWithNotification(ctx context.Context, entry *LogEntry, notifyAdmins bool) error {
	s.enrichEntry(ctx, entry)
	if err := s.repo.Insert(ctx, entry); err != nil {
		return err
	}
	if notifyAdmins && s.notificationPub != nil {
		go s.publishAuditLogNotification(entry)
	}
	return nil
}

func (s *service) enrichEntry(ctx context.Context, entry *LogEntry) {
	meta := auditctx.GetMetadata(ctx)
	
	// DEBUG: Log context metadata
	log.Printf("[DEBUG-ENRICH] Action: %s | Context UserID: %v | Context PracticeID: %v",
		entry.Action,
		meta.UserID,
		meta.PracticeID,
	)
	
	if entry.PracticeID == nil {
		entry.PracticeID = meta.PracticeID
	}
	if entry.UserID == nil {
		entry.UserID = meta.UserID
	}
	if entry.IPAddress == nil {
		entry.IPAddress = meta.IPAddress
	}
	if entry.UserAgent == nil {
		entry.UserAgent = meta.UserAgent
	}
	
	// DEBUG: Log enriched entry
	log.Printf("[DEBUG-ENRICHED] Action: %s | Entry UserID: %v | Entry PracticeID: %v",
		entry.Action,
		entry.UserID,
		entry.PracticeID,
	)
}

func (s *service) asyncWorker() {
	for job := range s.logChan {
		if err := s.repo.Insert(job.ctx, job.entry); err != nil {
			log.Printf("ERROR: failed to insert audit log: %v (action: %s.%s)", err, job.entry.Module, job.entry.Action)
			continue
		}
		if s.notificationService != nil {
			go s.publishAuditLogNotification(job.entry)
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
	detail := buildSystemIssueMessage(action, actorName, entityLabel, issueErr)

	// Determine notification event type
	var eventType util.EventType
	if level == auditctx.ActionSystemError {
		eventType = util.EventSystemError
	} else {
		eventType = util.EventSystemWarning
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
		eventType := util.EventSystemWarning
		if level == auditctx.ActionSystemError {
			eventType = util.EventSystemError
		}
		// Send the clean 'detail' string as the notification body
		go s.publishSystemIssueNotification(level, action, detail, parsedEntityID, eventType)
	}
}

// buildSystemIssueMessage produces the single human-readable string shown on the admin panel.
func buildSystemIssueMessage(action, actorName, entityName string, err error) string {
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
func (s *service) publishSystemIssueNotification(level, action, detail string, entityID uuid.UUID, eventType util.EventType) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	title := "System Warning"
	if level == auditctx.ActionSystemError {
		title = "System Error"
	}

	extraData := map[string]interface{}{"action": action}
	senderType := util.ActorSystem
	senderID := uuid.Nil

	// Build admin recipients list using adminRepo
	recipients := make([]sharednotification.RecipientWithPreferences, 0)
	
	if s.adminRepo != nil {
		admins, err := s.adminRepo.GetAllAdmins(ctx)
		if err != nil {
			log.Printf("[WARN] failed to get admin users for system notification: %v", err)
			return
		}
		
		for _, admin := range admins {
			recipients = append(recipients, sharednotification.RecipientWithPreferences{
				RecipientID:   admin.ID,
				RecipientType: util.ActorAdmin,
				UserID:        admin.User.ID,
			})
		}
	}

	if len(recipients) == 0 {
		return
	}

	// Use Publish method which checks preferences
	_ = s.notificationPub.Publish(ctx, sharednotification.PublishRequest{
		Recipients: recipients,
		SenderID:   senderID,
		SenderType: senderType,
		SenderName: "System",
		EventType:  eventType,
		EntityType: util.EntitySystem,
		EntityID:   entityID,
		EntityKey:  "system_id",
		Title:      title,
		Body:       detail,
		ExtraData:  extraData,
	})
}

// This runs in its own goroutine to avoid blocking the audit worker
// publishAuditLogNotification sends notifications to all admin users about a new audit log
func (s *service) publishAuditLogNotification(entry *LogEntry) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// DEBUG: Log entry details to check if UserID is present
	log.Printf("[DEBUG-AUDIT] Action: %s | UserID: %v | PracticeID: %v",
		entry.Action,
		entry.UserID,
		entry.PracticeID,
	)

	var err error
	// Get User Name
	userName := "System"

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
		"clinic.created":    "created clinic",
		"clinic.updated":    "updated clinic",
		"clinic.deleted":    "deleted clinic",
		"form.created":      "created form",
		"form.updated":      "updated form",
		"form.deleted":      "deleted form",
		"entry.created":     "created entry",
		"entry.updated":     "updated entry",
		"entry.deleted":     "deleted entry",
		"entry.confirmed":   "confirmed entry",
		"coa.created":       "created Chart of Accounts",
		"coa.updated":       "updated Chart of Accounts",
		"coa.deleted":       "deleted Chart of Accounts",
		"fy.updated":        "updated Financial Year",
		"fy.closed":         "closed Financial Year",
		"lock_date.updated": "updated lock date",

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
		"subscription.created":                   "created a new subscription plan",
		"subscription.updated":                   "updated subscription plan",
		"subscription.deleted":                   "deleted subscription plan",
		auditctx.ActionBillingPaymentSuccess:     "successfully processed payment for",
		auditctx.ActionBillingActivationSuccess:  "successfully activated subscription for",
		auditctx.ActionBillingPaymentFailed:      "payment failed for",
		auditctx.ActionBillingActivationFailed:   "failed to activate subscription for",
		auditctx.ActionBillingWebhookSigInvalid:  "received invalid billing webhook signature",
		auditctx.ActionBillingStatusUpdateFailed: "failed to update subscription status for",

		// Report Generate and Export
		"bas_report.exported":         "exported BAS Report",
		"pl_report.exported":          "exported Profit and Loss Report",
		"activity_statement.exported": "exported Activity Statement",
		"transactions.exported":       "exported Transactions",
		"balance_sheet.exported":      "exported Balance Sheet",

		"bas_report.generated":         "generated BAS Report",
		"pl_report.generated":          "generated Profit and Loss Report",
		"activity_statement.generated": "generated Activity Statement",
		"transactions.generated":       "generated Transactions",
		"balance_sheet.generated":      "generated Balance Sheet",
	}

	var message string

	title := "System Activity Alert"

	// Specialized logic for Lock Date messages
	if entry.Action == auditctx.ActionLockDateUpdated {
		getLockDate := func(state interface{}) *string {
			if state == nil {
				return nil
			}

			bytes, _ := json.Marshal(state)
			var data map[string]interface{}
			json.Unmarshal(bytes, &data)

			val, ok := data["LockDate"]
			if !ok {
				val, ok = data["lock_date"]
			}

			if ok && val != nil {
				str, isString := val.(string)
				if isString && str != "" {
					return &str
				}
			}
			return nil
		}

		afterDate := getLockDate(entry.AfterState)

		if afterDate == nil {
			message = fmt.Sprintf("%s unset the lock date", userName)
		} else {
			message = fmt.Sprintf("%s changed lock date to \"%s\"", userName, *afterDate)
		}
	} else {
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
		message = fmt.Sprintf("%s %s %s", userName, formattedAction, entityName)
	}
	message = strings.TrimSpace(message) // Clean up trailing spaces if name was nil

	// Construct Payload
	extraData := map[string]interface{}{
		"module":    entry.Module,
		"action":    entry.Action,
		"entity_id": entry.EntityID,
	}

	// Send to each admin (excluding the acting admin)
	senderType := util.ActorSystem
	senderID := uuid.Nil
	var entityID uuid.UUID
	if entry.EntityID != nil {
		parsed, err := uuid.Parse(*entry.EntityID)
		if err == nil {
			entityID = parsed
		}
	}
	eventType := resolveAdminEventType(entry.Action)

	// Build admin recipients list, excluding the acting admin
	recipients := make([]sharednotification.RecipientWithPreferences, 0)

	if s.adminRepo != nil {
		admins, adminErr := s.adminRepo.GetAllAdmins(ctx)
		if adminErr != nil {
			log.Printf("[WARN] failed to get admin users for audit notification: %v", adminErr)
			return
		}

		log.Printf("[DEBUG-FILTER] Total admins found: %d", len(admins))

		// Get the acting user's ID to filter them out
		var actingUserID *uuid.UUID
		if entry.UserID != nil {
			if parsed, parseErr := uuid.Parse(*entry.UserID); parseErr == nil {
				actingUserID = &parsed
				log.Printf("[DEBUG-FILTER] Acting UserID parsed: %s", parsed.String())
			} else {
				log.Printf("[ERROR-FILTER] Failed to parse UserID: %v", parseErr)
			}
		} else {
			log.Printf("[WARN-FILTER] entry.UserID is nil - cannot filter acting admin")
		}

		// Add all admins except the one who performed the action
		for _, admin := range admins {
			log.Printf("[DEBUG-FILTER] Checking admin: %s (UserID: %s)", admin.User.Email, admin.User.ID.String())

			// Skip if this admin is the one who performed the action
			if actingUserID != nil && admin.User.ID == *actingUserID {
				log.Printf("[INFO] ✓ Skipping notification to acting admin: %s (action: %s)", admin.User.Email, entry.Action)
				continue
			}

			log.Printf("[DEBUG-FILTER] ✓ Adding admin %s to recipients", admin.User.Email)
			recipients = append(recipients, sharednotification.RecipientWithPreferences{
				RecipientID:   admin.ID,
				RecipientType: util.ActorAdmin,
				UserID:        admin.User.ID,
			})
		}
	}

	log.Printf("[DEBUG-FILTER] Final recipient count: %d", len(recipients))

	// Skip if no recipients left after filtering
	if len(recipients) == 0 {
		log.Printf("[INFO] No admin recipients for audit notification: %s", entry.Action)
		return
	}

	// Use Publish method which checks preferences
	_ = s.notificationPub.Publish(ctx, sharednotification.PublishRequest{
		Recipients: recipients,
		SenderID:   senderID,
		SenderType: senderType,
		SenderName: "System",
		EventType:  eventType,
		EntityType: util.EntityAuditLog,
		EntityID:   entityID,
		EntityKey:  "audit_log_id",
		Title:      title,
		Body:       message,
		ExtraData:  extraData,
	})
}

// resolveAdminEventType maps an audit action string to the appropriate admin EventType
// so that admin notification preferences can be controlled at a granular level.
func resolveAdminEventType(action string) util.EventType {
	switch action {
	case auditctx.ActionBillingPaymentSuccess, auditctx.ActionBillingActivationSuccess:
		return util.EventBillingPaymentSuccess
	case auditctx.ActionBillingPaymentFailed:
		return util.EventBillingPaymentFailed
	case auditctx.ActionSubscriptionCreated:
		return util.EventSubscriptionCreated
	case auditctx.ActionSubscriptionUpdated:
		return util.EventSubscriptionUpdated
	case auditctx.ActionSubscriptionDeleted:
		return util.EventSubscriptionDeleted
	case auditctx.ActionUserRegistered, auditctx.ActionPractitionerCreated:
		return util.EventPractitionerCreated
	default:
		return util.EventAuditLogCreated
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
