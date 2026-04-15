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
	GetInvitationDetails(ctx context.Context, inviteID uuid.UUID) (*RsInviteDetails, error)
	ProcessInvitation(ctx context.Context, req *RqProcessAction) (*RsInviteProcess, error)
	FinalizeRegistrationInternal(ctx context.Context, tx *sqlx.Tx, email string, entityID uuid.UUID) error
	ListInvitations(ctx context.Context, pID, aID *uuid.UUID, f *Filter) (*util.RsList, error)
	ResendInvite(ctx context.Context, practitionerID uuid.UUID, inviteID uuid.UUID) (*RsInvitation, error)
	RevokeInvite(ctx context.Context, practitionerID uuid.UUID, inviteID uuid.UUID) error
	GetInvitationByEmailInternal(ctx context.Context, email string) (*Invitation, error)

	GetPermissionsForAccountant(ctx context.Context, accountantID uuid.UUID, entityID uuid.UUID) (*Permissions, error)
	GrantEntityPermissionTx(ctx context.Context, tx *sqlx.Tx, pID, aID, eID uuid.UUID, eType string, perms Permissions) error
	DeletePermissionsByEntityTx(ctx context.Context, tx *sqlx.Tx, entityID uuid.UUID) error
	IsAccountantLinkedToPractitioner(ctx context.Context, practitionerID, accountantID uuid.UUID) (bool, error)
	GetFirstPractitionerLinkedToAccountant(ctx context.Context, accountantID uuid.UUID) (uuid.UUID, error)
	GrantEntityPermission(ctx context.Context, pID uuid.UUID, aID *uuid.UUID, eID uuid.UUID, email string, eType string, perms Permissions) (*Permissions, error)
	ListAccountantPermissions(ctx context.Context, accountantID uuid.UUID, f *Filter) (*util.RsList, error)

	ListAccountantPermission(ctx context.Context, accId uuid.UUID) (*[]Permissions, int, error)
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
}

func NewService(repo Repository, cfg *config.Config, notification notification.Service, auditSvc audit.Service) Service {
	return &service{
		repo:         repo,
		cfg:          cfg,
		inviteConfig: util.InviteDefaultConfig(),
		notification: notification,
		auditSvc:     auditSvc,
	}
}

func (s *service) SendInvite(ctx context.Context, practitionerID uuid.UUID, req *RqSendInvitation) (*RsInvitation, error) {
	senderName, err := s.repo.GetPractitionerName(ctx, practitionerID)
	if err != nil {
		return nil, err
	}

	// Check if an accountant already exists for this email
	existingAccID, _ := s.repo.GetAccountantIDByEmail(ctx, req.Email)

	var baseURL string
	if s.cfg.Env == "dev" {
		baseURL = s.cfg.DevUrl
	} else {
		baseURL = s.cfg.LocalUrl
	}

	if baseURL == "" {
		return nil, ErrConfigMissing
	}

	// Start Transaction
	tx, err := s.repo.(*repository).db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	invite := &Invitation{
		ID:             uuid.New(),
		PractitionerID: practitionerID,
		EntityID:       existingAccID,
		Email:          strings.ToLower(strings.TrimSpace(req.Email)),
		Status:         StatusSent,
		ExpiresAt:      time.Now().AddDate(0, 0, s.inviteConfig.ExpirationDays),
	}

	if err := s.repo.CreateTx(ctx, tx, invite); err != nil {
		return nil, fmt.Errorf("failed to save invite: %w", err)
	}

	// We'll create a slice to hold the "finalized" permissions (after processing)
	var processedPermissions []RqPermissionDetail

	// Process and Save Permissions
	for _, pDetail := range req.Permissions {
		finalDisplay := s.processPermissions(pDetail.Permissions)

		// We use the Email because the AccountantID doesn't exist yet
		err := s.repo.GrantEntityPermissionTx(ctx, tx, practitionerID, existingAccID, &req.Email, pDetail.EntityID, pDetail.EntityType, finalDisplay)
		if err != nil {
			fmt.Printf("[ERROR] failed to save pending permission: %v\n", err)
		}

		// Add to our response slice
		processedPermissions = append(processedPermissions, RqPermissionDetail{
			EntityID:    pDetail.EntityID,
			EntityType:  pDetail.EntityType,
			Permissions: finalDisplay,
		})
	}

	// Commit DB changes
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	inviteLink := fmt.Sprintf("%s/accept-invite?token=%s", baseURL, invite.ID)

	go func() {
		if err := s.sendEmailViaResend(invite.Email, inviteLink, senderName); err != nil {
			fmt.Printf("[ERROR] Failed to send invitation email: %v\n", err)
		}
	}()

	// Notify the invitee (accountant) if they already have an account
	if s.notification != nil {
		recipientID, _ := s.repo.GetAccountantIDByEmail(ctx, invite.Email)
		if recipientID != nil && *recipientID != practitionerID {
			body := json.RawMessage(fmt.Sprintf(`"%s invited you to collaborate."`, senderName))
			extraData := map[string]interface{}{"invite_id": invite.ID.String()}
			payload := notification.BuildNotificationPayload("Invitation received", body, nil, nil, &extraData)
			payloadBytes, _ := json.Marshal(payload)
			senderType := notification.ActorPractitioner
			rq := notification.RqNotification{
				ID:            uuid.New(),
				RecipientID:   *recipientID,
				RecipientType: notification.ActorAccountant,
				SenderID:      &practitionerID,
				SenderType:    &senderType,
				EventType:     notification.EventInviteSent,
				EntityType:    notification.EntityInvite,
				EntityID:      invite.ID,
				Status:        notification.StatusUnread,
				Payload:       payloadBytes,
				CreatedAt:     time.Now(),
			}
			if err := s.notification.Publish(ctx, rq); err != nil {
				fmt.Printf("[ERROR] failed to publish invite.sent notification: %v\n", err)
			}
		}
	}

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

	return &RsInvitation{
		ID:           invite.ID,
		Email:        invite.Email,
		AccountantID: existingAccID,
		InviteLink:   inviteLink,
		Status:       invite.Status,
		ExpiresAt:    invite.ExpiresAt,
		Permissions:  processedPermissions,
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

func (s *service) GetInvitationDetails(ctx context.Context, inviteID uuid.UUID) (*RsInviteDetails, error) {
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

	// Fetch Permissions associated with this email
	dbPerms, err := s.repo.GetAllAccountantPermissions(ctx, inv.PractitionerID, inv.Email, accountantID)
	if err != nil {
		dbPerms = []RqPermissionDetail{} // Ensure it's not nil
		return nil, fmt.Errorf("failed to fetch permissions: %w", err)
	}
	if dbPerms == nil {
		dbPerms = []RqPermissionDetail{} // Initialize to empty slice for JSON []
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
		SentTo:      recipient,
		Permissions: dbPerms,
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

	if req.Action == ActionReject {
		if err := s.repo.UpdateStatus(ctx, inv.ID, StatusRejected, nil); err != nil {
			return nil, err
		}
		res.Status = StatusRejected
		res.IsFound = false
		s.logInvitationAction(ctx, inv, auditctx.ActionInviteRejected, beforeState)
		return res, nil
	}

	if req.Action == ActionAccept {
		accountantID, err := s.repo.GetAccountantIDByEmail(ctx, inv.Email)
		if err != nil {
			return nil, fmt.Errorf("failed to check accountant existence: %w", err)
		}

		var targetStatus InvitationStatus
		if accountantID != nil {
			targetStatus = StatusCompleted
			res.IsFound = true
		} else {
			targetStatus = StatusAccepted
			res.IsFound = false
		}

		if err := s.repo.UpdateStatus(ctx, inv.ID, targetStatus, accountantID); err != nil {
			return nil, err
		}

		res.Status = targetStatus
		s.notifyInvitationAccepted(ctx, inv, accountantID)
		s.logInvitationAction(ctx, inv, auditctx.ActionInviteAccepted, beforeState)
		return res, nil
	}

	return nil, ErrInvalidAction
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
	if err := s.repo.UpdateStatusTx(ctx, tx, inv.ID, StatusCompleted, &entityID); err != nil {
		return err
	}

	// Update accountant_id in tbl_invite_permissions for all entries with this email to map permissions with accountant_id
	if err := s.repo.LinkPermissionsToAccountantTx(ctx, tx, email, entityID); err != nil {
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

func (s *service) ListInvitations(ctx context.Context, pID, aID *uuid.UUID, f *Filter) (*util.RsList, error) {
	// Accountant path: query by email so SENT/REJECTED (entity_id = NULL) are also visible
	if aID != nil {
		email, err := s.repo.GetEmailByAccountantID(ctx, *aID)
		if err != nil {
			return nil, fmt.Errorf("resolve accountant email: %w", err)
		}

		ft := f.MapToFilterAccountant()

		list, err := s.repo.ListByEmail(ctx, email, ft)
		if err != nil {
			return nil, err
		}
		total, err := s.repo.CountByEmail(ctx, email, ft)
		if err != nil {
			return nil, err
		}

		var rsList util.RsList
		rsList.MapToList(list, total, *ft.Offset, *ft.Limit)
		return &rsList, nil
	}

	// Practitioner path: unchanged
	ft := f.MapToFilter(pID, nil)
	list, err := s.repo.List(ctx, ft)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.Count(ctx, ft)
	if err != nil {
		return nil, err
	}

	var rsList util.RsList
	rsList.MapToList(list, total, *ft.Offset, *ft.Limit)
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

	if err := s.repo.UpdateStatus(ctx, oldInv.ID, StatusResent, oldInv.EntityID); err != nil {
		return nil, fmt.Errorf("failed to invalidate old invitation: %w", err)
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
	tx, err := s.repo.(*repository).db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	if err := s.repo.UpdateStatusTx(ctx, tx, inviteID, StatusRevoked, inv.EntityID); err != nil {
		return fmt.Errorf("revoke invitation status update: %w", err)
	}

	if err := s.repo.DeleteAllPermissionsForAccountantTx(ctx, tx, practitionerID, accountantID); err != nil {
		return fmt.Errorf("revoke invitation permissions cleanup: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit revocation: %w", err)
	}

	s.logInvitationAction(ctx, inv, auditctx.ActionInviteRevoked, inv)
	return nil
}

func (s *service) GetInvitationByEmailInternal(ctx context.Context, email string) (*Invitation, error) {
	return s.repo.GetByEmail(ctx, email)
}

func (s *service) GetPermissionsForAccountant(ctx context.Context, accountantID uuid.UUID, entityID uuid.UUID) (*Permissions, error) {
	return s.repo.GetPermissions(ctx, accountantID, entityID)
}

func (s *service) GrantEntityPermissionTx(ctx context.Context, tx *sqlx.Tx, pID, aID, eID uuid.UUID, eType string, perms Permissions) error {
	// Ensure they at least have "read" access even if the clinic didn't have it.
	if !perms.All {
		perms.Read = true
	}

	perms = s.processPermissions(perms)

	return s.repo.GrantEntityPermissionTx(ctx, tx, pID, &aID, nil, eID, eType, perms)
}

func (s *service) DeletePermissionsByEntityTx(ctx context.Context, tx *sqlx.Tx, entityID uuid.UUID) error {
	return s.repo.DeletePermissionsByEntityTx(ctx, tx, entityID)
}

func (s *service) IsAccountantLinkedToPractitioner(ctx context.Context, practitionerID, accountantID uuid.UUID) (bool, error) {
	return s.repo.IsAccountantLinkedToPractitioner(ctx, practitionerID, accountantID)
}

func (s *service) GetFirstPractitionerLinkedToAccountant(ctx context.Context, accountantID uuid.UUID) (uuid.UUID, error) {
	return s.repo.GetFirstPractitionerLinkedToAccountant(ctx, accountantID)
}

func (s *service) ListAccountantPermissions(ctx context.Context, aID uuid.UUID, f *Filter) (*util.RsList, error) {

	ft := f.MapToFilterAccountant()

	// Add the mandatory filters to the filter object
	ft.Where = append(ft.Where, common.Condition{
		Field:    "accountant_id",
		Operator: common.OpEq,
		Value:    aID,
	}, common.Condition{
		Field:    "deleted_at",
		Operator: common.OpIsNull,
	})

	res, err := s.repo.ListAccountantPermissions(ctx, ft)
	if err != nil {
		return nil, fmt.Errorf("list accountant permissions: %w", err)
	}

	var resultList []AccountantPermissionRes

	for _, row := range res {

		// 4. Map the row to the response struct
		resultList = append(resultList, AccountantPermissionRes{
			ID:             row.ID,
			EntityID:       row.EntityID,
			EntityType:     row.EntityType,
			PractitionerID: row.PractitionerID,
			AccountantID:   row.AccountantID,
			Permissions:    row.Permissions,
			CreatedAt:      row.CreatedAt,
			UpdatedAt:      row.UpdatedAt,
		})
	}

	total, err := s.repo.CountAccountantPermissions(ctx, ft)
	if err != nil {
		return nil, fmt.Errorf("count accountant permissions: %w", err)
	}

	var rsList util.RsList
	rsList.MapToList(resultList, total, *ft.Offset, *ft.Limit)
	return &rsList, nil

}

func (s *service) GrantEntityPermission(ctx context.Context, pID uuid.UUID, aID *uuid.UUID, eID uuid.UUID, email string, eType string, perms Permissions) (*Permissions, error) {
	if aID == nil || *aID == uuid.Nil {
		// If AccountantID is missing, check the invitation by Email instead
		inv, err := s.repo.GetByEmail(ctx, email)
		if err != nil || inv == nil || inv.PractitionerID != pID {
			return nil, ErrUnauthorizedAssociation
		}
	} else {
		// Standard check for registered users - verify this specific practitioner-accountant relationship exists
		isLinked, err := s.repo.IsAccountantLinkedToPractitioner(ctx, pID, *aID)
		if err != nil {
			return nil, ErrUnauthorizedAssociation
		}
		if !isLinked {
			return nil, ErrUnauthorizedAssociation
		}
	}

	// Capture state BEFORE update for Audit Logs
	var oldPerms *Permissions

	if aID != nil && *aID != uuid.Nil {
		// Registered user: Get by ID
		oldPerms, _ = s.repo.GetPermissions(ctx, *aID, eID)
	} else if email != "" {
		// Invited user: Get one permission by email/entity
		oldPerms, _ = s.repo.GetPermissionsByEmailAndEntity(ctx, pID, email, eID)
	}

	// Check if this is an "Empty" permission set
	if !perms.All && !perms.Read && !perms.Create && !perms.Update && !perms.Delete {
		// We return nil for the *Permissions and the error from the repo
		err := s.repo.DeletePermission(ctx, pID, eID, aID, email)
		return nil, err
	}

	// Process Permissions
	finalDisplay := s.processPermissions(perms)

	// Save to DB and return finalDisplay
	if err := s.repo.GrantEntityPermission(ctx, pID, aID, &email, eID, eType, finalDisplay); err != nil {
		return nil, err
	}

	// Audit Log (Async)
	meta := auditctx.GetMetadata(ctx)
	pIDStr := pID.String()
	eIDStr := eID.String()
	// aIDStr := aID.String()
	entityType := auditctx.EntityPermission

	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID:  &pIDStr,
		UserID:      meta.UserID,
		Module:      auditctx.ModuleBusiness,
		Action:      auditctx.ActionPermissionUpdated,
		EntityType:  &entityType,
		EntityID:    &eIDStr,
		BeforeState: oldPerms,
		AfterState:  finalDisplay,
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
	})

	return &finalDisplay, nil
}

// Helper to centralize permission logic
func (s *service) processPermissions(perms Permissions) Permissions {
	finalDisplay := Permissions{}

	if perms.All {
		// If All is true, everything is true
		finalDisplay = Permissions{All: true, Read: true, Create: true, Update: true, Delete: true}
	} else {
		finalDisplay.Read = true // Always grant read at minimum

		if perms.Create {
			finalDisplay.Create = true
		}
		if perms.Update {
			finalDisplay.Update = true
		}
		if perms.Delete {
			finalDisplay.Delete = true
		}
	}

	return finalDisplay
}

// ListAccountantPermission implements [Service].
func (s *service) ListAccountantPermission(ctx context.Context, accId uuid.UUID) (*[]Permissions, int, error) {
	var filter common.Filter

	permissionRows, err := s.repo.ListAccountantPermissions(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	var perms []Permissions
	for _, v := range permissionRows {
		perms = append(perms, v.Permissions)
	}
	return &perms, 0, nil
}
