package invitation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/notification"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/mail"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
)

type Config interface {
	GetBaseURL() (string, error)
}

type Service interface {
	SendInvite(ctx context.Context, practitionerID uuid.UUID, req *RqSendInvitation) (*RsInvitation, error)
	GetInvitation(ctx context.Context, inviteID uuid.UUID) (*RsInviteDetails, error)
	ProcessInvitation(ctx context.Context, req *RqProcessAction) (*RsInviteProcess, error)
	FinalizeRegistrationInternal(ctx context.Context, tx *sqlx.Tx, email string, entityID uuid.UUID) error
	ListInvitations(ctx context.Context, actorID *uuid.UUID, f *Filter) (*util.RsList, error)
	ResendInvite(ctx context.Context, practitionerID uuid.UUID, inviteID uuid.UUID) (*RsInvitation, error)
	RevokeInvite(ctx context.Context, practitionerID uuid.UUID, inviteID uuid.UUID) error
	GetInvitationByEmailInternal(ctx context.Context, email string) (*Invitation, error)
	GetPermissionsForAccountant(ctx context.Context, accountantID uuid.UUID, practitionerID uuid.UUID) (*Permissions, error)
	UpdatePermissions(ctx context.Context, practitionerID uuid.UUID, req *RqUpdatePermissions) (*Permissions, error)
	GrantEntityPermission(ctx context.Context, tx *sqlx.Tx, pID, aID uuid.UUID, email string, perms Permissions, invitationID uuid.UUID) error
	DeletePermission(ctx context.Context, tx *sqlx.Tx, entityID uuid.UUID) error
	IsAccountantLinkedToPractitioner(ctx context.Context, practitionerID, accountantID uuid.UUID) (bool, error)
	GetPractitionersLinkedToAccountant(ctx context.Context, accountantID uuid.UUID) ([]uuid.UUID, error)
	ListPermissions(ctx context.Context, accountantID uuid.UUID, f *Filter) ([]map[string]interface{}, error)
}

const (
	ActionAccept = "ACCEPT"
	ActionReject = "REJECT"
)

type service struct {
	repo            Repository
	cfg             *config.Config
	inviteConfig    util.InvitationConfig
	notification    notification.Service
	notificationPub *sharednotification.Publisher
	auditSvc        audit.Service
	db              *sqlx.DB
	mailer          *mail.Client
}

func NewService(repo Repository, cfg *config.Config, notificationSvc notification.Service, auditSvc audit.Service, db *sqlx.DB) Service {
	return &service{
		repo:            repo,
		cfg:             cfg,
		inviteConfig:    util.InviteDefaultConfig(),
		notification:    notificationSvc,
		notificationPub: sharednotification.NewPublisher(notification.NewServiceAdapter(notificationSvc)),
		auditSvc:        auditSvc,
		db:              db,
		mailer:          mail.NewClient(cfg.ResendAPIKey, cfg.SenderEmail),
	}
}

func (s *service) SendInvite(ctx context.Context, practitionerID uuid.UUID, req *RqSendInvitation) (*RsInvitation, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(req.Email))
	existingCount, err := s.repo.CountInvitationPerPractitionerAndEmail(ctx, practitionerID, normalizedEmail)
	if err != nil {
		return nil, fmt.Errorf("failed to check duplicate invitation history: %w", err)
	}
	if existingCount > 0 {
		return nil, ErrInvitationAlreadyExists
	}

	senderName, err := s.repo.GetPractitionerName(ctx, practitionerID)
	if err != nil {
		return nil, err
	}

	existingAccID, err := s.repo.GetAccountantIDByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing accountant: %w", err)
	}

	baseURL, err := s.cfg.GetBaseURL()
	if err != nil {
		return nil, err
	}

	invite := &Invitation{
		ID:             uuid.New(),
		PractitionerID: practitionerID,
		AccountantID:   existingAccID,
		Email:          strings.ToLower(strings.TrimSpace(req.Email)),
		Status:         StatusSent,
		ExpiresAt:      time.Now().AddDate(0, 0, s.inviteConfig.ExpirationDays),
	}

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := s.repo.Create(ctx, tx, invite); err != nil {
			return fmt.Errorf("failed to save invite: %w", err)
		}
		if req.Permissions != nil {
			if err := s.repo.GrantEntityPermission(ctx, tx, practitionerID, existingAccID, invite.Email, *req.Permissions, invite.ID); err != nil {
				return fmt.Errorf("failed to save permissions: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	inviteLink := fmt.Sprintf("%s/accept-invite?token=%s", baseURL, invite.ID)

	go func(email, name, link string, pID uuid.UUID, invID uuid.UUID) {
		if err := s.mailer.SendInvitationEmail(email, name, link); err != nil {
			s.auditSvc.LogSystemIssue(context.Background(), auditctx.ActionSystemError, "invitation.send_email",
				err, pID.String(), invID.String(), auditctx.EntityInvitation, auditctx.ModuleBusiness,
			)
		}
	}(invite.Email, senderName, inviteLink, practitionerID, invite.ID)

	s.notifyInvitation(ctx, invite, invite.AccountantID, util.EventInviteSent, invite.ID, util.ActorPractitioner, "invite_id", "New Collaboration Invitation", fmt.Sprintf("%s invited you to collaborate.", senderName))

	meta := auditctx.GetMetadata(ctx)
	pIDStr := practitionerID.String()
	invIDStr := invite.ID.String()

	s.submitAuditLog(*meta, &pIDStr, auditctx.ActionInviteSent, auditctx.EntityInvitation, invIDStr, nil, invite)
	if req.Permissions != nil {
		s.submitAuditLog(*meta, &pIDStr, auditctx.ActionPermissionAssigned, auditctx.EntityPermission, invIDStr, nil, req.Permissions)
	}

	return &RsInvitation{
		ID:           invite.ID,
		Email:        invite.Email,
		AccountantID: existingAccID,
		InviteLink:   inviteLink,
		Status:       invite.Status,
		ExpiresAt:    invite.ExpiresAt,
	}, nil
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
		accountantID, _ = s.repo.GetAccountantIDByEmail(ctx, inv.Email)
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

	beforeState := *inv
	if time.Now().After(inv.ExpiresAt) {
		return nil, ErrInvitationExpired
	}

	switch inv.Status {
	case StatusRevoked:
		return nil, ErrInvitatationRevoked
	case StatusRejected, StatusCompleted:
		return nil, ErrInvitationAlreadyUsed
	}

	res := &RsInviteProcess{InvitationID: inv.ID, PractitionerID: inv.PractitionerID, Email: inv.Email}

	switch req.Action {
	case ActionReject:
		err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
			if err := s.repo.UpdateStatus(ctx, tx, inv.ID, StatusRejected, inv.AccountantID, inv.ExpiresAt); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to update status of invitation: %w", err)
		}
		res.Status = StatusRejected
		res.IsFound = false
		s.logInvitationAction(ctx, inv, auditctx.ActionInviteRejected, beforeState)
		return res, nil
	case ActionAccept:
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

		err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
			if err := s.repo.UpdateStatus(ctx, tx, inv.ID, targetStatus, accountantID, inv.ExpiresAt); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to update status of invitation: %w", err)
		}

		res.Status = targetStatus
		s.notifyInvitation(ctx, inv, accountantID, util.EventInviteAccepted, inv.ID, util.ActorAccountant, "invite_id", "Invitation Accepted", fmt.Sprintf("%s accepted your invitation.", inv.Email))
		s.logInvitationAction(ctx, inv, auditctx.ActionInviteAccepted, beforeState)
		return res, nil
	}

	return nil, ErrInvalidAction
}

func (s *service) FinalizeRegistrationInternal(ctx context.Context, tx *sqlx.Tx, email string, entityID uuid.UUID) error {
	inv, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return err
	}

	if inv == nil {
		return nil
	}

	if time.Now().After(inv.ExpiresAt) {
		return ErrInvitationExpired
	}

	var targetStatus InvitationStatus
	var dbAccountantID *uuid.UUID
	switch inv.Status {
	case StatusAccepted:
		targetStatus = StatusCompleted
		dbAccountantID = &entityID
	case StatusSent:
		targetStatus = StatusSent
		dbAccountantID = nil
	default:
		return nil
	}

	if err := s.repo.UpdateStatus(ctx, tx, inv.ID, targetStatus, dbAccountantID, inv.ExpiresAt); err != nil {
		return err
	}

	if targetStatus == StatusCompleted {
		afterState := *inv
		afterState.Status = StatusCompleted

		meta := auditctx.GetMetadata(ctx)
		pIDStr := inv.PractitionerID.String()
		s.submitAuditLog(*meta, &pIDStr, auditctx.ActionInviteCompleted, auditctx.EntityInvitation, inv.ID.String(), inv, afterState)
	}
	return nil
}

func (s *service) ListInvitations(ctx context.Context, actorID *uuid.UUID, f *Filter) (*util.RsList, error) {
	baseUrl, err := s.cfg.GetBaseURL()
	if err != nil {
		return nil, err
	}

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

		for _, row := range listRows {
			if row.Status == StatusSent {
				row.InviteLink = fmt.Sprintf("%s/accept-invite?token=%s", baseUrl, row.ID)
			}
		}

		var rsList util.RsList
		rsList.MapToList(listRows, total, *ft.Offset, *ft.Limit)
		return &rsList, nil
	}

	ft := f.MapToFilter(actorID)
	listRows, err := s.repo.ListForPractitioner(ctx, *actorID, ft)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.Count(ctx, ft)
	if err != nil {
		return nil, err
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

	permissions, err := s.repo.GetPermission(ctx, oldInv.AccountantID, oldInv.PractitionerID, &oldInv.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch permissions: %w", err)
	}

	senderName, err := s.repo.GetPractitionerName(ctx, practitionerID)
	if err != nil {
		return nil, err
	}

	baseURL, err := s.cfg.GetBaseURL()
	if err != nil {
		return nil, err
	}

	newExpiration := time.Now().AddDate(0, 0, s.inviteConfig.ExpirationDays)

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := s.repo.UpdateStatus(ctx, tx, oldInv.ID, StatusSent, oldInv.AccountantID, newExpiration); err != nil {
			return fmt.Errorf("failed to update status of invitation: %w", err)
		}
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to resend invitation: %w", err)
	}

	inviteLink := fmt.Sprintf("%s/accept-invite?token=%s", baseURL, oldInv.ID)

	go func(email, name, link string, pID uuid.UUID, invID uuid.UUID) {
		if err := s.mailer.SendInvitationEmail(email, name, link); err != nil {
			s.auditSvc.LogSystemIssue(context.Background(), auditctx.ActionSystemError, "invitation.resend_email",
				err, pID.String(), invID.String(), auditctx.EntityInvitation, auditctx.ModuleBusiness,
			)
		}
	}(oldInv.Email, senderName, inviteLink, practitionerID, oldInv.ID)

	if err == nil {
		meta := auditctx.GetMetadata(ctx)
		s.submitAuditLog(*meta, meta.PracticeID, auditctx.ActionInviteSent, auditctx.EntityInvitation, oldInv.ID.String(), nil, nil)
	}

	return &RsInvitation{
		ID:           oldInv.ID,
		Email:        oldInv.Email,
		AccountantID: oldInv.AccountantID,
		InviteLink:   inviteLink,
		Status:       StatusSent,
		ExpiresAt:    newExpiration,
		Permissions:  permissions,
	}, err
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

	var accountantID uuid.UUID
	if inv.AccountantID != nil {
		accountantID = *inv.AccountantID
	}

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := s.repo.UpdateStatus(ctx, tx, inviteID, StatusRevoked, inv.AccountantID, inv.ExpiresAt); err != nil {
			return fmt.Errorf("revoke invitation status update: %w", err)
		}

		if err := s.repo.DeleteAllPermissionsForAccountant(ctx, tx, practitionerID, accountantID); err != nil {
			return fmt.Errorf("revoke invitation permissions cleanup: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to revoke invitation: %w", err)
	}

	s.logInvitationAction(ctx, inv, auditctx.ActionInviteRevoked, inv)
	return nil
}

func (s *service) UpdatePermissions(ctx context.Context, practitionerID uuid.UUID, req *RqUpdatePermissions) (*Permissions, error) {
	if err := req.Permissions.Validate(); err != nil {
		return nil, fmt.Errorf("invalid permissions: %w", err)
	}

	var accountantID *uuid.UUID
	var useEmail bool

	accID, err := s.repo.GetAccountantIDByEmail(ctx, req.Email)
	if err == nil && accID != nil {
		accountantID = accID
		useEmail = false
	} else {
		useEmail = true
	}

	if accountantID != nil {
		isLinked, err := s.repo.IsAccountantLinkedToPractitioner(ctx, practitionerID, *accountantID)
		if err != nil {
			return nil, fmt.Errorf("failed to verify accountant link: %w", err)
		}
		if !isLinked {
			return nil, fmt.Errorf("accountant is not linked to this practitioner")
		}
	} else {
		inv, err := s.repo.GetByEmail(ctx, req.Email)
		if err != nil || inv == nil || inv.PractitionerID != practitionerID {
			return nil, fmt.Errorf("no invitation found for email %s from this practitioner", req.Email)
		}
	}

	var oldPerms *Permissions
	if accountantID != nil {
		oldPerms, _ = s.repo.GetPermissionsByPractitionerAndAccountant(ctx, practitionerID, *accountantID)
	} else {
		oldPerms, _ = s.repo.GetPermission(ctx, nil, practitionerID, &req.Email)
	}

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		if useEmail {
			return s.repo.GrantEntityPermission(ctx, tx, practitionerID, nil, req.Email, *req.Permissions, uuid.Nil)
		}
		return s.repo.GrantEntityPermission(ctx, tx, practitionerID, accountantID, req.Email, *req.Permissions, uuid.Nil)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update permissions: %w", err)
	}

	meta := auditctx.GetMetadata(ctx)
	pIDStr := practitionerID.String()
	var entityID string
	if accountantID != nil {
		entityID = accountantID.String()
	} else {
		entityID = req.Email
	}

	inv := &Invitation{
		PractitionerID: practitionerID,
		AccountantID:   accountantID,
		Email:          req.Email,
	}
	s.notifyInvitation(ctx, inv, accountantID, util.EventPermissionUpdated, uuid.Nil, util.ActorAccountant, "permission_update", "Permissions Updated", fmt.Sprintf("Your permissions for practitioner %s have been updated.", pIDStr))

	s.submitAuditLog(*meta, &pIDStr, auditctx.ActionPermissionUpdated, auditctx.EntityPermission, entityID, oldPerms, req.Permissions)
	return req.Permissions, nil
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

func (s *service) notifyInvitation(ctx context.Context, inv *Invitation, accountantID *uuid.UUID, eventType util.EventType, entityID uuid.UUID, recipientType util.ActorType, entityKey string, title string, body string) {
	if s.notification == nil {
		return
	}

	userID, err := s.repo.GetPractitionerUserIDByID(ctx, inv.PractitionerID)
	if err != nil {
		return
	}

	var recipients []sharednotification.RecipientWithPreferences

	switch recipientType {
	case util.ActorPractitioner:
		recipients = []sharednotification.RecipientWithPreferences{
			{
				RecipientID:   inv.PractitionerID,
				RecipientType: util.ActorPractitioner,
				UserID:        userID,
			},
		}
	case util.ActorAccountant:
		if accountantID == nil {
			return
		}
		recipients = []sharednotification.RecipientWithPreferences{
			{
				RecipientID:   *accountantID,
				RecipientType: util.ActorAccountant,
				UserID:        userID,
			},
		}
	default:
		return
	}

	senderName := inv.Email
	senderID := uuid.Nil
	if accountantID != nil {
		senderID = *accountantID
	}

	_ = s.notificationPub.Publish(ctx, sharednotification.PublishRequest{
		Recipients: recipients,
		SenderID:   senderID,
		SenderType: util.ActorAccountant,
		SenderName: senderName,
		EventType:  eventType,
		EntityType: util.EntityInvite,
		EntityID:   entityID,
		EntityKey:  entityKey,
		Title:      title,
		Body:       body,
	})
}

func (s *service) logInvitationAction(ctx context.Context, inv *Invitation, action string, beforeState interface{}) {
	if s.auditSvc == nil {
		return
	}
	meta := auditctx.GetMetadata(ctx)
	pIDStr := inv.PractitionerID.String()
	updatedInv, _ := s.repo.GetByID(ctx, inv.ID)

	s.submitAuditLog(*meta, &pIDStr, action, auditctx.EntityInvitation, inv.ID.String(), beforeState, updatedInv)
}

func (s *service) submitAuditLog(meta auditctx.Metadata, practiceID *string, action, entityType, entityID string, before, after interface{}) {
	if s.auditSvc == nil {
		return
	}
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID:  practiceID,
		UserID:      meta.UserID,
		Module:      auditctx.ModuleBusiness,
		Action:      action,
		EntityType:  &entityType,
		EntityID:    &entityID,
		BeforeState: before,
		AfterState:  after,
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
	})
}

func (s *service) GetInvitationByEmailInternal(ctx context.Context, email string) (*Invitation, error) {
	return s.repo.GetByEmail(ctx, email)
}

func (s *service) GetPermissionsForAccountant(ctx context.Context, accountantID uuid.UUID, practitionerID uuid.UUID) (*Permissions, error) {
	return s.repo.GetPermissionsByPractitionerAndAccountant(ctx, practitionerID, accountantID)
}

func (s *service) GrantEntityPermission(ctx context.Context, tx *sqlx.Tx, pID, aID uuid.UUID, email string, perms Permissions, invitationID uuid.UUID) error {
	return s.repo.GrantEntityPermission(ctx, tx, pID, &aID, email, perms, invitationID)
}

func (s *service) DeletePermission(ctx context.Context, tx *sqlx.Tx, entityID uuid.UUID) error {
	return s.repo.DeletePermission(ctx, tx, entityID)
}

func (s *service) IsAccountantLinkedToPractitioner(ctx context.Context, practitionerID, accountantID uuid.UUID) (bool, error) {
	return s.repo.IsAccountantLinkedToPractitioner(ctx, practitionerID, accountantID)
}

func (s *service) GetPractitionersLinkedToAccountant(ctx context.Context, accountantID uuid.UUID) ([]uuid.UUID, error) {
	return s.repo.GetPractitionersLinkedToAccountant(ctx, accountantID)
}

func (s *service) ListPermissions(ctx context.Context, accId uuid.UUID, f *Filter) ([]map[string]interface{}, error) {
	filter := f.MapToFilterAccountant()
	invWithPerms, err := s.repo.ListPermissions(ctx, accId, filter)
	if err != nil {
		return nil, err
	}

	results := make([]map[string]interface{}, 0, len(invWithPerms))
	for _, inv := range invWithPerms {
		results = append(results, map[string]interface{}{
			"id":              inv.ID,
			"practitioner_id": inv.PractitionerID,
			"accountant_id":   inv.AccountantID,
			"permissions":     inv.Permissions,
			"created_at":      inv.CreatedAt,
			"updated_at":      inv.UpdatedAt,
		})
	}
	return results, nil
}
