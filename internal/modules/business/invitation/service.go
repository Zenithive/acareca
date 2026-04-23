package invitation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/notification"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type Service interface {
	SendInvite(ctx context.Context, practitionerID uuid.UUID, req *RqSendInvitation) (*RsInvitation, error)
	GetInvitation(ctx context.Context, inviteID uuid.UUID) (*RsInviteDetails, error)
	ProcessInvitation(ctx context.Context, req *RqProcessAction) (*RsInviteProcess, error)
	FinalizeRegistrationInternal(ctx context.Context, tx *sqlx.Tx, email string, entityID uuid.UUID) error
	ListInvitation(ctx context.Context, actorID *uuid.UUID, f *Filter) (*util.RsList, error)
	ResendInvite(ctx context.Context, practitionerID uuid.UUID, inviteID uuid.UUID) (*RsInvitation, error)
	RevokeInvite(ctx context.Context, practitionerID uuid.UUID, inviteID uuid.UUID) error

	UpdatePermissions(ctx context.Context, practitionerID uuid.UUID, req *RqUpdatePermissions) (*Permissions, error)
	ListPermissions(ctx context.Context, accountantID uuid.UUID, f *Filter) (*RsPermission, error)
}

const (
	ActionAccept = "ACCEPT"
	ActionReject = "REJECT"
)

type service struct {
	repo         Repository
	cfg          *config.Config
	inviteConfig util.InvitationConfig
	notification notification.Service
	auditSvc     audit.Service
	db           *sqlx.DB
}

func NewService(repo Repository, cfg *config.Config, notification notification.Service, auditSvc audit.Service, db *sqlx.DB) Service {
	return &service{
		repo:         repo,
		cfg:          cfg,
		inviteConfig: util.InviteDefaultConfig(),
		notification: notification,
		auditSvc:     auditSvc,
		db:           db,
	}
}

func (s *service) SendInvite(ctx context.Context, practitionerID uuid.UUID, req *RqSendInvitation) (*RsInvitation, error) {
	senderName, err := s.repo.GetPractitionerName(ctx, practitionerID)
	if err != nil {
		return nil, err
	}

	// Check if an accountant already exists for this email
	existingAccID, err := s.repo.GetAccountantIDByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing accountant: %w", err)
	}

	baseURL, err := s.cfg.GetBaseURL()
	if err != nil {
		return nil, err
	}

	inviteLink := ""
	invite := &Invitation{
		ID:             uuid.New(),
		PractitionerID: practitionerID,
		EntityID:       existingAccID,
		Email:          strings.ToLower(strings.TrimSpace(req.Email)),
		Status:         StatusSent,
		ExpiresAt:      time.Now().AddDate(0, 0, s.inviteConfig.ExpirationDays),
	}

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {

		if err := s.repo.Create(ctx, tx, invite); err != nil {
			return fmt.Errorf("failed to save invite: %w", err)
		}

		err = s.repo.GrantEntityPermission(ctx, tx, practitionerID, existingAccID, invite.Email, *req.Permissions)
		if err != nil {
			return fmt.Errorf("failed to save permissions: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	inviteLink = fmt.Sprintf("%s/accept-invite?token=%s", baseURL, invite.ID)

	go func() {
		if err := s.sendEmailViaResend(invite.Email, inviteLink, senderName); err != nil {
			fmt.Printf("[ERROR] Failed to send invitation email: %v\n", err)
			s.auditSvc.LogSystemIssue(context.Background(), auditctx.ActionSystemError, "invitation.send_email",
				err, practitionerID.String(), invite.ID.String(), auditctx.EntityInvitation, auditctx.ModuleBusiness,
			)
		}
	}()

	common.PublishNotification(ctx, s.notification, invite.EntityID, practitionerID, invite,
		func(inv *Invitation) common.NotificationMeta {
			return common.NotificationMeta{
				EntityID:      inv.ID,
				EntityKey:     "invite_id",
				Title:         "Invitation received",
				Body:          fmt.Sprintf(`"%s invited you to collaborate."`, senderName),
				EventType:     notification.EventInviteSent,
				EntityType:    notification.EntityInvite,
				RecipientType: notification.ActorAccountant,
			}
		},
	)

	// Audit log: invitation sent
	meta := auditctx.GetMetadata(ctx)
	entityID := invite.ID.String()
	pIDStr := practitionerID.String()
	entityType := auditctx.EntityInvitation
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID:  &pIDStr,
		UserID:      meta.UserID,
		Module:      auditctx.ModuleBusiness,
		Action:      auditctx.ActionInviteSent,
		EntityType:  &entityType,
		EntityID:    &entityID,
		BeforeState: nil,
		AfterState:  invite,
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
	})

	// Audit Log: Permissions Assigned
	permEntityType := auditctx.EntityPermission
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID:  &pIDStr,
		UserID:      meta.UserID,
		Module:      auditctx.ModuleBusiness,
		Action:      auditctx.ActionPermissionAssigned,
		EntityType:  &permEntityType,
		EntityID:    &entityID,
		BeforeState: nil,
		// AfterState:  processedPerms,
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})

	return &RsInvitation{
		ID:           invite.ID,
		Email:        invite.Email,
		AccountantID: existingAccID,
		InviteLink:   inviteLink,
		Status:       invite.Status,
		ExpiresAt:    invite.ExpiresAt,
		// Permissions:  processedPerms,
	}, nil
}

func (s *service) sendEmailViaResend(to string, link string, senderName string) error {
	url := "https://api.resend.com/emails"

	namePart := strings.Split(to, "@")[0]
	namePart = strings.ReplaceAll(namePart, ".", " ")
	namePart = strings.ReplaceAll(namePart, "_", " ")
	recipientName := cases.Title(language.English).String(namePart)

	payload := map[string]interface{}{
		"from":    "Acareca <hardik@zenithive.digital>",
		"to":      []string{to},
		"subject": fmt.Sprintf("Invitation: Manage %s's files on Acareca", senderName),
		"html": fmt.Sprintf(`
			<div style="font-family: sans-serif; color: #333; line-height: 1.6;">
				<p style="font-size: 14px;">Hello <strong>%s</strong>,</p>
				<p><strong>%s</strong> has invited you to collaborate on <strong>Acareca</strong> as their Accountant/Bookkeeper.</p>
				<p>Acareca is a secure platform designed to streamline financial management and document sharing between practitioners and financial professionals.</p>
				<div style="margin: 30px 0;">
					<a href="%s" style="background-color: #1a73e8; color: white; padding: 12px 24px; text-decoration: none; border-radius: 4px; font-weight: bold;">
						Access Client Files
					</a>
				</div>
				<p>By accepting this invitation, you will be able to view and manage financial records shared by %s.</p>
				<hr style="border: none; border-top: 1px solid #eee; margin: 20px 0;" />
				<small style="color: #888;">This invitation was intended for %s and will expire in 7 days.</small>
			</div>
		`, recipientName, senderName, link, senderName, to),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+s.cfg.ResendAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: s.inviteConfig.EmailTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend api returned status: %d, detail: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func (s *service) GetInvitation(ctx context.Context, inviteID uuid.UUID) (*RsInviteDetails, error) {
	inv, err := s.repo.GetInvitationByID(ctx, inviteID)
	if err != nil {
		return nil, err
	}

	if inv == nil {
		return &RsInviteDetails{InvitationID: inviteID, IsFound: false}, nil
	}

	if inv.Status == StatusSent && time.Now().After(inv.ExpiresAt) {
		return nil, ErrInvitationExpired
	}

	recipient := UserDetails{Email: inv.Email}

	queryUser, _ := s.repo.GetUserDetailsByEmail(ctx, inv.Email)
	var accountantID *uuid.UUID
	isFound := false
	if queryUser != nil {
		recipient.FirstName = queryUser.FirstName
		recipient.LastName = queryUser.LastName
		accID, _ := s.repo.GetAccountantIDByEmail(ctx, inv.Email)
		accountantID = accID
		isFound = true
	}

	permissions, err := s.repo.GetPermission(ctx, accountantID, inv.PractitionerID, &inv.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch permissions: %w", err)
	}

	return &RsInviteDetails{
		InvitationID: inv.ID,
		Status:       inv.Status,
		IsFound:      isFound,
		AccountantID: accountantID,
		Email:        inv.Email,
		SenderRole:   util.RolePractitioner,
		SentBy: UserDetails{
			FirstName: inv.SenderFirstName,
			LastName:  inv.SenderLastName,
			Email:     inv.SenderEmail,
		},
		SentTo:     recipient,
		Permission: permissions,
	}, nil
}

func (s *service) ProcessInvitation(ctx context.Context, req *RqProcessAction) (*RsInviteProcess, error) {
	inv, err := s.repo.GetByID(ctx, req.TokenID)
	if err != nil || inv == nil {
		return nil, ErrInvitationNotFound
	}

	beforeState := inv
	if time.Now().After(inv.ExpiresAt) {
		return nil, ErrInvitationExpired
	}

	if inv.Status == StatusResent {
		return nil, ErrInvitationInvalidated
	}

	res := &RsInviteProcess{InvitationID: inv.ID, PractitionerID: inv.PractitionerID, Email: inv.Email}

	if inv.Status == StatusRejected || inv.Status == StatusCompleted {
		return nil, ErrInvitationAlreadyUsed
	}
	var targetStatus InvitationStatus
	var accountantID *uuid.UUID
	util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		if req.Action == ActionReject {
			if err := s.repo.UpdateStatus(ctx, tx, inv.ID, StatusRejected, inv.EntityID); err != nil {
				return err
			}
			res.Status = StatusRejected
			res.IsFound = false
			s.logInvitationAction(ctx, inv, auditctx.ActionInviteRejected, beforeState)
			return nil
		}

		if req.Action == ActionAccept {
			accountantID, err := s.repo.GetAccountantIDByEmail(ctx, inv.Email)
			if err != nil {
				return fmt.Errorf("failed to check accountant existence: %w", err)
			}

			if accountantID != nil {
				targetStatus = StatusCompleted
				res.IsFound = true
			} else {
				targetStatus = StatusAccepted
				res.IsFound = false
			}

			if err := s.repo.UpdateStatus(ctx, tx, inv.ID, targetStatus, accountantID); err != nil {
				return fmt.Errorf("failed to update invitation status: %w", err)
			}

		}
		return nil
	})

	res.Status = targetStatus
	s.notifyInvitationAccepted(ctx, inv, accountantID)
	s.logInvitationAction(ctx, inv, auditctx.ActionInviteAccepted, beforeState)
	return res, nil

}

func (s *service) notifyInvitationAccepted(ctx context.Context, inv *Invitation, accountantID *uuid.UUID) {
	if s.notification == nil {
		return
	}

	body := json.RawMessage(fmt.Sprintf(`"%s accepted your invitation."`, inv.Email))
	extraData := map[string]interface{}{"invite_id": inv.ID.String()}
	payload := notification.BuildNotificationPayload("Invitation Accepted", body, nil, nil, &extraData)
	payloadBytes, _ := json.Marshal(payload)
	senderType := notification.ActorAccountant
	rq := notification.RqNotification{
		ID:            uuid.New(),
		RecipientID:   inv.PractitionerID,
		RecipientType: notification.ActorPractitioner,
		SenderID:      accountantID,
		SenderType:    &senderType,
		EventType:     notification.EventInviteAccepted,
		EntityType:    notification.EntityInvite,
		EntityID:      inv.ID,
		Status:        notification.StatusUnread,
		Payload:       payloadBytes,
		CreatedAt:     time.Now(),
	}
	if err := s.notification.Publish(ctx, rq); err != nil {
		fmt.Printf("[ERROR] failed to publish invite.accepted notification: %v\n", err)
	}
}

func (s *service) logInvitationAction(ctx context.Context, inv *Invitation, action string, beforeState interface{}) {
	if s.auditSvc == nil {
		return
	}

	meta := auditctx.GetMetadata(ctx)
	pIDStr := inv.PractitionerID.String()
	entityID := inv.ID.String()
	entityType := auditctx.EntityInvitation

	updatedInv, _ := s.repo.GetByID(ctx, inv.ID)

	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID:  &pIDStr,
		UserID:      meta.UserID,
		Module:      auditctx.ModuleBusiness,
		Action:      action,
		EntityType:  &entityType,
		EntityID:    &entityID,
		BeforeState: beforeState,
		AfterState:  updatedInv,
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
	})
}

func (s *service) FinalizeRegistrationInternal(ctx context.Context, tx *sqlx.Tx, email string, entityID uuid.UUID) error {
	inv, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return err
	}

	if inv == nil {
		return nil
	}

	if inv.Status != StatusAccepted && inv.Status != StatusSent {
		return nil
	}

	// Update Invitation Status
	if err := s.repo.UpdateStatus(ctx, tx, inv.ID, StatusCompleted, &entityID); err != nil {
		return err
	}

	// Update accountant_id in tbl_invite_permissions for all entries with this email to map permissions with accountant_id
	if err := s.repo.LinkPermissionsToAccountant(ctx, tx, email, entityID); err != nil {
		s.auditSvc.LogSystemIssue(ctx, auditctx.ActionSystemError, "invitation.link_permissions",
			err, "", entityID.String(), auditctx.EntityPermission, auditctx.ModuleBusiness,
		)
		return fmt.Errorf("failed to link permissions: %w", err)
	}

	// Audit log: invitation completed
	meta := auditctx.GetMetadata(ctx)
	pIDStr := inv.PractitionerID.String()
	entityIDStr := inv.ID.String()
	entityType := auditctx.EntityInvitation
	s.auditSvc.LogAsync(&audit.LogEntry{
		//PracticeID: meta.PracticeID,
		PracticeID:  &pIDStr,
		UserID:      meta.UserID,
		Module:      auditctx.ModuleBusiness,
		Action:      auditctx.ActionInviteCompleted,
		EntityType:  &entityType,
		EntityID:    &entityIDStr,
		BeforeState: inv,
		AfterState:  "COMPLETED",
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
	})
	return nil
}

func (s *service) ListInvitation(ctx context.Context, actorID *uuid.UUID, f *Filter) (*util.RsList, error) {
	var baseURL string
	if s.cfg.Env == "dev" {
		baseURL = s.cfg.DevUrl
	} else {
		baseURL = s.cfg.LocalUrl
	}

	// Accountant path: query by email with practitioner details
	if f.Role == util.RoleAccountant && actorID != nil {
		email, err := s.repo.GetEmailByAccountantID(ctx, *actorID)
		if err != nil {
			return nil, fmt.Errorf("resolve accountant email: %w", err)
		}

		ft := f.MapToFilterAccountant()

		listRows, err := s.repo.ListForAccountant(ctx, email, ft)
		if err != nil {
			return nil, err
		}
		total, err := s.repo.CountByEmail(ctx, email, ft)
		if err != nil {
			return nil, err
		}

		// Add invite links for SENT status
		for _, row := range listRows {
			if row.Status == StatusSent {
				row.InviteLink = fmt.Sprintf("%s/accept-invite?token=%s", baseURL, row.ID)
			}
		}

		var rsList util.RsList
		rsList.MapToList(listRows, total, *ft.Offset, *ft.Limit)
		return &rsList, nil
	}

	// Practitioner path: same response structure for consistency
	ft := f.MapToFilter(actorID)
	listRows, err := s.repo.ListForPractitioner(ctx, *actorID, ft)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.Count(ctx, ft)
	if err != nil {
		return nil, err
	}

	// Add invite links for SENT status
	for _, row := range listRows {
		if row.Status == StatusSent {
			row.InviteLink = fmt.Sprintf("%s/accept-invite?token=%s", baseURL, row.ID)
		}
	}

	var rsList util.RsList
	rsList.MapToList(listRows, total, *ft.Offset, *ft.Limit)
	return &rsList, nil
}

func (s *service) ResendInvite(ctx context.Context, practitionerID uuid.UUID, inviteID uuid.UUID) (*RsInvitation, error) {
	oldInv, err := s.repo.GetByID(ctx, inviteID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch invitation: %w", err)
	}

	if oldInv == nil {
		return nil, errors.New("invitation not found")
	}

	if oldInv.PractitionerID != practitionerID {
		return nil, errors.New("unauthorized: you did not send this invitation")
	}

	if err := s.checkInvitationLimit(ctx, practitionerID, oldInv.Email); err != nil {
		return nil, err
	}

	if oldInv.Status == StatusCompleted {
		return nil, fmt.Errorf("cannot resend: invitation is already %s", oldInv.Status)
	}

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := s.repo.UpdateStatus(ctx, tx, oldInv.ID, StatusResent, oldInv.EntityID); err != nil {
			return fmt.Errorf("failed to invalidate old invitation: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Resend invitation - log after successful resend
	newInvite, err := s.SendInvite(ctx, practitionerID, &RqSendInvitation{
		Email: oldInv.Email,
	})
	if err == nil {
		// Audit log for resend (use new invite ID)
		meta := auditctx.GetMetadata(ctx)
		newEntityID := newInvite.ID.String()
		entityType := auditctx.EntityInvitation
		s.auditSvc.LogAsync(&audit.LogEntry{
			PracticeID: meta.PracticeID,
			UserID:     meta.UserID,
			Module:     auditctx.ModuleBusiness,
			Action:     auditctx.ActionInviteSent,
			EntityType: &entityType,
			EntityID:   &newEntityID,
			IPAddress:  meta.IPAddress,
			UserAgent:  meta.UserAgent,
		})
	}
	return newInvite, err
}

func (s *service) checkInvitationLimit(ctx context.Context, pID uuid.UUID, email string) error {
	count, err := s.repo.CountDailyInvitesByEmail(ctx, pID, email)
	if err != nil {
		return fmt.Errorf("failed to check invitation limit: %w", err)
	}

	if count >= s.inviteConfig.DailyInviteLimit {
		return ErrDailyLimitReached
	}
	return nil
}

func (s *service) RevokeInvite(ctx context.Context, practitionerID uuid.UUID, inviteID uuid.UUID) error {
	var err error
	inv, err := s.repo.GetByID(ctx, inviteID)
	if err != nil || inv == nil {
		return ErrInvitationNotFound
	}

	if inv.PractitionerID != practitionerID {
		return ErrUnauthorizedInvite
	}

	if inv.Status == StatusRevoked {
		return ErrInvitationAlreadyUsed
	}

	if inv.Status != StatusAccepted && inv.Status != StatusCompleted {
		return ErrCannotRevokeStatus
	}

	accountantID := *inv.EntityID

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := s.repo.UpdateStatus(ctx, tx, inviteID, StatusRevoked, inv.EntityID); err != nil {
			return fmt.Errorf("revoke invitation status update: %w", err)
		}

		if err := s.repo.DeleteAllPermissionsForAccountant(ctx, tx, practitionerID, accountantID); err != nil {
			return fmt.Errorf("revoke invitation permissions cleanup: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	s.logInvitationAction(ctx, inv, auditctx.ActionInviteRevoked, inv)
	return nil
}

func (s *service) UpdatePermissions(ctx context.Context, practitionerID uuid.UUID, req *RqUpdatePermissions) (*Permissions, error) {
	// Validate permissions
	if err := req.Permissions.Validate(); err != nil {
		return nil, fmt.Errorf("invalid permissions: %w", err)
	}

	// Determine if we're updating by accountant_id or email
	var accountantID *uuid.UUID
	var useEmail bool

	// First, try to resolve the accountant by email (most reliable)
	accID, err := s.repo.GetAccountantIDByEmail(ctx, req.Email)
	if err == nil && accID != nil {
		// Accountant exists and is registered, use their ID
		accountantID = accID
		useEmail = false
	} else {
		// Accountant doesn't exist yet (pending invitation), use email
		accountantID = nil
		useEmail = true
	}

	// If user provided accountant_id, verify it matches what we found
	if req.AccountantID != nil && *req.AccountantID != uuid.Nil {
		if accountantID != nil && *accountantID != *req.AccountantID {
			// The provided ID doesn't match the actual accountant ID
			// This might be an entity_id from invitation table, ignore it and use what we found
			fmt.Printf("Warning: provided accountant_id %s doesn't match actual accountant_id %s for email %s\n",
				req.AccountantID.String(), accountantID.String(), req.Email)
		}
	}

	// Check if the accountant/email is linked to this practitioner via invitation
	if accountantID != nil {
		isLinked, err := s.repo.IsAccountantLinkedToPractitioner(ctx, practitionerID, *accountantID)
		if err != nil {
			return nil, fmt.Errorf("failed to verify accountant link: %w", err)
		}
		if !isLinked {
			return nil, fmt.Errorf("accountant is not linked to this practitioner")
		}
	} else {
		// For pending invitations, check if there's an invitation for this email
		inv, err := s.repo.GetByEmail(ctx, req.Email)
		if err != nil || inv == nil || inv.PractitionerID != practitionerID {
			return nil, fmt.Errorf("no invitation found for email %s from this practitioner", req.Email)
		}
	}

	// Get old permissions for audit log
	var oldPerms *Permissions
	if accountantID != nil {
		oldPerms, _ = s.repo.GetPermissionsByPractitionerAndAccountant(ctx, practitionerID, *accountantID)
	} else {
		oldPerms, _ = s.repo.GetPermission(ctx, nil, practitionerID, &req.Email)
	}

	// Update permissions in a transaction
	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		if useEmail {
			// For pending invitations, pass nil accountant_id and the email
			return s.repo.GrantEntityPermission(ctx, tx, practitionerID, nil, req.Email, *req.Permissions)
		} else {
			// For registered accountants, pass the accountant_id
			return s.repo.GrantEntityPermission(ctx, tx, practitionerID, accountantID, req.Email, *req.Permissions)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update permissions: %w", err)
	}

	// Audit log
	meta := auditctx.GetMetadata(ctx)
	pIDStr := practitionerID.String()
	entityType := auditctx.EntityPermission
	var entityID string
	if accountantID != nil {
		entityID = accountantID.String()
	} else {
		entityID = req.Email
	}

	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID:  &pIDStr,
		UserID:      meta.UserID,
		Module:      auditctx.ModuleBusiness,
		Action:      auditctx.ActionPermissionUpdated,
		EntityType:  &entityType,
		EntityID:    &entityID,
		BeforeState: oldPerms,
		AfterState:  req.Permissions,
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
	})

	return req.Permissions, nil
}

// Helper to centralize permission logic

// ListAccountantPermission implements [Service].
func (s *service) ListPermissions(ctx context.Context, accId uuid.UUID, f *Filter) (*RsPermission, error) {
	var filter common.Filter
	filter = f.MapToFilterAccountant()

	permission, err := s.repo.ListPermission(ctx, filter)
	if err != nil {
		return nil, err
	}

	rs := permission.ToRsPermission()

	return rs, nil
}
