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
	"github.com/iamarpitzala/acareca/internal/modules/notification"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type Service interface {
	SendInvite(ctx context.Context, practitionerID uuid.UUID, req *RqSendInvitation) (*RsInvitation, error)
	GetInvitationDetails(ctx context.Context, inviteID uuid.UUID) (*RsInviteDetails, error)
	ProcessInvitation(ctx context.Context, req *RqProcessAction) (*RsInviteProcess, error)
	FinalizeRegistrationInternal(ctx context.Context, email string, entityID uuid.UUID) error
	ListInvitations(ctx context.Context, pID, aID *uuid.UUID, f *Filter) (*util.RsList, error)
	ResendInvite(ctx context.Context, practitionerID uuid.UUID, inviteID uuid.UUID) (*RsInvitation, error)
}

const (
	ActionAccept = "ACCEPT"
	ActionReject = "REJECT"
)

type service struct {
	repo         Repository
	cfg          *config.Config
	notification notification.Service
}

func NewService(repo Repository, cfg *config.Config, notification notification.Service) Service {
	return &service{
		repo:         repo,
		cfg:          cfg,
		notification: notification,
	}
}

func (s *service) SendInvite(ctx context.Context, practitionerID uuid.UUID, req *RqSendInvitation) (*RsInvitation, error) {
	senderName, err := s.repo.GetPractitionerName(ctx, practitionerID)
	if err != nil {
		return nil, err
	}

	var baseURL string
	if s.cfg.Env == "dev" {
		baseURL = s.cfg.DevUrl
	} else {
		baseURL = s.cfg.LocalUrl
	}

	if baseURL == "" {
		fmt.Printf("[CRITICAL] Configuration Error: Frontend URL is missing for ENV=%s\n", s.cfg.Env)
		return nil, fmt.Errorf("system configuration error: frontend application URL is not defined")
	}

	invite := &Invitation{
		ID:             uuid.New(),
		PractitionerID: practitionerID,
		Email:          req.Email,
		Status:         StatusSent,
		ExpiresAt:      time.Now().AddDate(0, 0, 7),
	}

	if err := s.repo.Create(ctx, invite); err != nil {
		return nil, fmt.Errorf("[DEBUG] failed to save invite: %w", err)
	}

	inviteLink := fmt.Sprintf("%s/accept-invite?token=%s", baseURL, invite.ID)

	go func() {
		if err := s.sendEmailViaResend(invite.Email, inviteLink, senderName); err != nil {
			fmt.Printf("[DEBUG] Resend Error: %v\n", err)
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

	return &RsInvitation{
		ID:         invite.ID,
		Email:      invite.Email,
		InviteLink: inviteLink,
		Status:     invite.Status,
		ExpiresAt:  invite.ExpiresAt,
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

	client := &http.Client{Timeout: 10 * time.Second}
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

	if time.Now().After(inv.ExpiresAt) {
		return nil, errors.New("Invitation expired")
	}

	recipient := UserDetails{Email: inv.Email}

	queryUser, _ := s.repo.GetUserDetailsByEmail(ctx, inv.Email)
	isFound := false
	if queryUser != nil {
		recipient.FirstName = queryUser.FirstName
		recipient.LastName = queryUser.LastName
		isFound = true
	}

	return &RsInviteDetails{
		InvitationID: inv.ID,
		Status:       inv.Status,
		IsFound:      isFound,
		SenderRole:   util.RolePractitioner,
		SentBy: UserDetails{
			FirstName: inv.SenderFirstName,
			LastName:  inv.SenderLastName,
			Email:     inv.SenderEmail,
		},
		SentTo: recipient,
	}, nil
}

func (s *service) ProcessInvitation(ctx context.Context, req *RqProcessAction) (*RsInviteProcess, error) {
	inv, err := s.repo.GetByID(ctx, req.TokenID)
	if err != nil || inv == nil {
		return nil, errors.New("invitation not found")
	}

	if time.Now().After(inv.ExpiresAt) {
		return nil, errors.New("invitation expired")
	}

	if inv.Status == StatusResent {
		return nil, errors.New("this invitation link is no longer valid as a new one has been sent")
	}

	res := &RsInviteProcess{InvitationID: inv.ID, PractitionerID: inv.PractitionerID, Email: inv.Email}

	if inv.Status == StatusRejected || inv.Status == StatusCompleted {
		return nil, fmt.Errorf("action not allowed: invitation is already %s", inv.Status)
	}

	if req.Action == ActionReject {
		if err := s.repo.UpdateStatus(ctx, inv.ID, StatusRejected, nil); err != nil {
			return nil, err
		}
		res.Status = StatusRejected
		res.IsFound = false
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
		res.IsFound = false

		// Notify the practitioner live
		if s.notification != nil {
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

		return res, nil
	}

	return nil, errors.New("invalid action: must be ACCEPT or REJECT")
}

func (s *service) FinalizeRegistrationInternal(ctx context.Context, email string, entityID uuid.UUID) error {
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

	return s.repo.UpdateStatus(ctx, inv.ID, StatusCompleted, &entityID)
}

func (s *service) ListInvitations(ctx context.Context, pID, aID *uuid.UUID, f *Filter) (*util.RsList, error) {
	ft := f.MapToFilter(pID, aID)

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

	return s.SendInvite(ctx, practitionerID, &RqSendInvitation{Email: oldInv.Email})
}

func (s *service) checkInvitationLimit(ctx context.Context, pID uuid.UUID, email string) error {
	count, err := s.repo.CountDailyInvitesByEmail(ctx, pID, email)
	if err != nil {
		return fmt.Errorf("failed to check invitation limit: %w", err)
	}

	if count >= 5 {
		return errors.New("daily invitation limit reached for this email (max 5 per 24h)")
	}
	return nil
}
