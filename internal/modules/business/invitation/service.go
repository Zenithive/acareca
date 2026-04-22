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
	ListInvitations(ctx context.Context, actorID *uuid.UUID, f *Filter) (*util.RsList, error)
	ResendInvite(ctx context.Context, practitionerID uuid.UUID, inviteID uuid.UUID) (*RsInvitation, error)
	RevokeInvite(ctx context.Context, practitionerID uuid.UUID, inviteID uuid.UUID) error
	GetInvitationByEmailInternal(ctx context.Context, email string) (*Invitation, error)

	GetPermissionsForAccountant(ctx context.Context, accountantID uuid.UUID, practitionerID uuid.UUID) (*Permissions, error)
	GrantEntityPermissionTx(ctx context.Context, tx *sqlx.Tx, pID, aID, eID uuid.UUID, eType string, perms Permissions) error
	DeletePermissionsByEntityTx(ctx context.Context, tx *sqlx.Tx, entityID uuid.UUID) error
	IsAccountantLinkedToPractitioner(ctx context.Context, practitionerID, accountantID uuid.UUID) (bool, error)
	GetFirstPractitionerLinkedToAccountant(ctx context.Context, accountantID uuid.UUID) (uuid.UUID, error)
	// GrantEntityPermission(ctx context.Context, pID uuid.UUID, aID *uuid.UUID, eID uuid.UUID, email string, eType string, perms Permissions) (*Permissions, error)
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

	// Process and validate permissions
	// processedPerms := s.processPermissions(req.Permissions)

	// // Validate permissions
	// if err := ValidatePermissions(processedPerms); err != nil {
	// 	return nil, fmt.Errorf("invalid permissions: %w", err)
	// }

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {

		if err := s.repo.CreateTx(ctx, tx, invite); err != nil {
			return fmt.Errorf("failed to save invite: %w", err)
		}

		err = s.repo.GrantEntityPermissionTx(ctx, tx, practitionerID, existingAccID, *req.Permissions)
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
	// _, err = s.repo.GetAllAccountantPermissions(ctx, inv.PractitionerID, inv.Email, accountantID)
	// if err != nil {
	// 	dbPerms = [] // Ensure it's not nil
	// 	return nil, fmt.Errorf("failed to fetch permissions: %w", err)
	// }
	// if dbPerms == nil {
	// 	dbPerms = []RqPermissionDetail{} // Initialize to empty slice for JSON []
	// }

	// processedPerm := s.processPermissions(DefaultAccountantPermissions())

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
		SentTo: recipient,
		// Permissions: processedPerm,
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

func (s *service) ListInvitations(ctx context.Context, actorID *uuid.UUID, f *Filter) (*util.RsList, error) {
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

func (s *service) GetPermissionsForAccountant(ctx context.Context, accountantID uuid.UUID, practitionerID uuid.UUID) (*Permissions, error) {
	return s.repo.GetPermissionsByPractitionerAndAccountant(ctx, practitionerID, accountantID)
}

func (s *service) GrantEntityPermissionTx(ctx context.Context, tx *sqlx.Tx, pID, aID, eID uuid.UUID, eType string, perms Permissions) error {
	// processedPerms := s.processPermissions(&perms)
	return s.repo.GrantEntityPermissionTx(ctx, tx, pID, &aID, nil)
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

// func (s *service) GrantEntityPermission(ctx context.Context, pID uuid.UUID, aID *uuid.UUID, eID uuid.UUID, email string, eType string, perms Permissions) (*Permissions, error) {
// 	associated := false

// 	if aID != nil && *aID != uuid.Nil {
// 		// Check formal link
// 		isLinked, _ := s.repo.IsAccountantLinkedToPractitioner(ctx, pID, *aID)
// 		if isLinked {
// 			associated = true
// 		} else {
// 			// Fallback: Check if they have an invitation under this email for this practitioner
// 			inv, err := s.repo.GetByEmail(ctx, email)
// 			if err == nil && inv != nil && inv.PractitionerID == pID {
// 				associated = true
// 			}
// 		}
// 	} else if email != "" {
// 		// Check invitation by Email only
// 		inv, err := s.repo.GetByEmail(ctx, email)
// 		if err == nil && inv != nil && inv.PractitionerID == pID {
// 			associated = true
// 		}
// 	}

// 	if !associated {
// 		return nil, ErrUnauthorizedAssociation
// 	}

// 	// Capture state BEFORE update for Audit Logs
// 	var oldPerms *Permissions

// 	if aID != nil && *aID != uuid.Nil {
// 		// Registered user: Get by ID
// 		oldPerms, _ = s.repo.GetPermissions(ctx, *aID, eID)
// 	} else if email != "" {
// 		// Invited user: Get one permission by email/entity
// 		oldPerms, _ = s.repo.GetPermissionsByEmailAndEntity(ctx, pID, email, eID)
// 	}

// 	// Process Permissions
// 	// finalDisplay := s.processPermissions(&perms)

// 	// Validate permissions
// 	// if err := ValidatePermissions(finalDisplay); err != nil {
// 	// 	return nil, fmt.Errorf("invalid permissions: %w", err)
// 	// }

// 	var dbAccID *uuid.UUID
// 	if aID != nil && *aID != uuid.Nil {
// 		// Check if this ID actually exists in the tbl_accountant table
// 		exists, err := s.repo.AccountantExists(ctx, *aID)
// 		if err == nil && exists {
// 			dbAccID = aID
// 		} else {
// 			// If the ID doesn't exist in tbl_accountant, we MUST use NULL (nil)
// 			// so the permission is stored against the Email instead.
// 			dbAccID = nil
// 		}
// 	}
// 	// Save to DB and return finalDisplay
// 	// if err := s.repo.GrantEntityPermission(ctx, pID, dbAccID, &email, eID, eType, *finalDisplay); err != nil {
// 	if err := s.repo.GrantEntityPermission(ctx, pID, dbAccID, &email, eID, eType, nil); err != nil {
// 		s.auditSvc.LogSystemIssue(ctx, auditctx.ActionSystemError, "invitation.grant_permission",
// 			err, pID.String(), eID.String(), auditctx.EntityPermission, auditctx.ModuleBusiness,
// 		)
// 		return nil, err
// 	}

// 	// Audit Log (Async)
// 	meta := auditctx.GetMetadata(ctx)
// 	pIDStr := pID.String()
// 	eIDStr := eID.String()
// 	// aIDStr := aID.String()
// 	entityType := auditctx.EntityPermission

// 	s.auditSvc.LogAsync(&audit.LogEntry{
// 		PracticeID:  &pIDStr,
// 		UserID:      meta.UserID,
// 		Module:      auditctx.ModuleBusiness,
// 		Action:      auditctx.ActionPermissionUpdated,
// 		EntityType:  &entityType,
// 		EntityID:    &eIDStr,
// 		BeforeState: oldPerms,
// 		// AfterState:  finalDisplay,
// 		IPAddress: meta.IPAddress,
// 		UserAgent: meta.UserAgent,
// 	})

// 	return finalDisplay, nil
// }

// Helper to centralize permission logic

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
