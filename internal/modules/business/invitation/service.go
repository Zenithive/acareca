package invitation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type Service interface {
	SendInvite(ctx context.Context, practitionerID uuid.UUID, req *RqSendInvitation) (*RsInvitation, error)
	GetInvitationDetails(ctx context.Context, inviteID uuid.UUID) (*RsInviteProcess, error)
	ProcessInvitation(ctx context.Context, req *RqProcessAction) (*RsInviteProcess, error)
	FinalizeRegistrationInternal(ctx context.Context, email string, userID uuid.UUID) error
}

const (
	ActionAccept = "ACCEPT"
	ActionReject = "REJECT"
)

type service struct {
	repo   Repository
	apiKey string // Resend API Key
}

func NewService(repo Repository, apiKey string) Service {
	return &service{
		repo:   repo,
		apiKey: apiKey,
	}
}

func (s *service) SendInvite(ctx context.Context, practitionerID uuid.UUID, req *RqSendInvitation) (*RsInvitation, error) {
	// Fetch Practitioner Name for the email
	senderName, err := s.repo.GetPractitionerName(ctx, practitionerID)
	if err != nil {
		return nil, err // If we can't identify the sender, we shouldn't send the invite
	}

	// Determine Base URL based on ENV
	var baseURL string
	env := os.Getenv("ENV")
	if env == "dev" {
		baseURL = os.Getenv("DEV_API_URL")
	} else {
		baseURL = os.Getenv("LOCAL_API_URL")
	}

	// Fallback in case envs are missing
	if baseURL == "" {
		baseURL = "http://localhost:5173"
	}

	invite := &Invitation{
		ID:             uuid.New(),
		PractitionerID: practitionerID,
		Email:          req.Email,
		Status:         StatusSent,
		ExpiresAt:      time.Now().AddDate(0, 0, 7),
	}

	// Save to Database
	if err := s.repo.Create(ctx, invite); err != nil {
		return nil, fmt.Errorf("[DEBUG] failed to save invite: %w", err)
	}

	// Format the URL
	inviteLink := fmt.Sprintf("%s/accept-invite?token=%s", baseURL, invite.ID)

	// Trigger Email via Resend
	go func() {
		// We run this in a goroutine so the API response isn't delayed by the email sending
		err := s.sendEmailViaResend(invite.Email, inviteLink, senderName)
		if err != nil {
			fmt.Printf("[DEBUG] Resend Error: %v\n", err)
		}
	}()

	return &RsInvitation{
		ID:         invite.ID,
		Email:      invite.Email,
		InviteLink: inviteLink,
		Status:     invite.Status,
		ExpiresAt:  invite.ExpiresAt,
	}, nil
}

// sendEmailViaResend handles the HTTP call to Resend's API
func (s *service) sendEmailViaResend(to string, link string, senderName string) error {
	url := "https://api.resend.com/emails"

	// Extract name from email (e.g., "john.doe@gmail.com" -> "John Doe")
	namePart := strings.Split(to, "@")[0]
	// Replace dots or underscores with spaces for a cleaner look
	namePart = strings.ReplaceAll(namePart, ".", " ")
	namePart = strings.ReplaceAll(namePart, "_", " ")
	// Capitalize the first letter of each word
	recipientName := cases.Title(language.English).String(namePart)

	// Resend API payload structure
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

	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
	// 	return fmt.Errorf("resend api returned status: %d", resp.StatusCode)
	// }

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		// Read the actual error message from the body
		bodyBytes, _ := io.ReadAll(resp.Body)

		return fmt.Errorf("resend api returned status: %d, detail: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func (s *service) GetInvitationDetails(ctx context.Context, inviteID uuid.UUID) (*RsInviteProcess, error) {
	inv, err := s.repo.GetByID(ctx, inviteID)
	if err != nil || inv == nil {
		return &RsInviteProcess{InvitationID: inviteID, IsFound: false}, nil
	}

	if time.Now().After(inv.ExpiresAt) {
		return nil, errors.New("Invitation expired")
	}

	// Check if user already exists in system
	userID, _ := s.repo.GetUserIDByEmail(ctx, inv.Email)

	return &RsInviteProcess{
		InvitationID: inv.ID,
		Status:       inv.Status,
		IsFound:      userID != nil,
	}, nil
}

func (s *service) ProcessInvitation(ctx context.Context, req *RqProcessAction) (*RsInviteProcess, error) {
	// Fetch the invitation
	inv, err := s.repo.GetByID(ctx, req.TokenID)
	if err != nil || inv == nil {
		return nil, errors.New("invitation not found")
	}

	// Expiration Check
	if time.Now().After(inv.ExpiresAt) {
		return nil, errors.New("invitation expired")
	}

	res := &RsInviteProcess{InvitationID: inv.ID}

	// Safety Check: If already Rejected or Completed, don't allow further changes
	if inv.Status == StatusRejected || inv.Status == StatusCompleted {
		return nil, fmt.Errorf("action not allowed: invitation is already %s", inv.Status)
	}

	// Handle STATUS REJECTED
	if req.Action == ActionReject {
		var actorUserID *uuid.UUID
		if s.n != nil {
			// Best-effort: resolve invitee user id from email for notification sender.
			actorUserID, _ = s.repo.GetUserIDByEmail(ctx, inv.Email)
		}

		if err := s.repo.UpdateStatus(ctx, inv.ID, StatusRejected, nil); err != nil {
			return nil, err
		}

		if s.n != nil {
			_ = s.n.NotifyInviteProcessed(ctx, actorUserID, inv.ID, inv.PractitionerID, "reject")
		}

		res.Status = StatusRejected
		res.IsFound = false
		return res, nil
	}

	// Handle ACCEPT Action
	if req.Action == ActionAccept {
		accountantID, err := s.repo.GetAccountantIDByEmail(ctx, inv.Email)
		if err != nil {
			return nil, fmt.Errorf("failed to check accountant existence: %w", err)
		}

		var targetStatus InvitationStatus
		if accountantID != nil {
			// User exists: mark as COMPLETED immediately
			targetStatus = StatusCompleted
			res.IsFound = true

			if s.n != nil {
				_ = s.n.NotifyInviteProcessed(ctx, inv.EntityID, inv.ID, inv.PractitionerID, "ACCEPTED")
			}
		} else {
			// User doesn't exist: mark as ACCEPTED (pending registration)
			targetStatus = StatusAccepted
			res.IsFound = false
		}

		if err := s.repo.UpdateStatus(ctx, inv.ID, targetStatus, accountantID); err != nil {
			return nil, err
		}

		res.Status = targetStatus
		res.IsFound = false // Redirect to register

		if s.n != nil {
			_ = s.n.NotifyInviteProcessed(ctx, nil, inv.ID, inv.PractitionerID, "ACCEPTED")
		}

		return res, nil

	}

	// Fallback for unexpected actions
	return nil, errors.New("invalid action: must be ACCEPT or REJECT")
}

// FinalizeRegistrationInternal is called by the Auth Service after a user registers
func (s *service) FinalizeRegistrationInternal(ctx context.Context, email string, userID uuid.UUID) error {
	// Look for an invitation with this email where status is currently 'ACCEPTED'
	inv, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return err
	}

	// If no invite exists for this email, they just signed up normally.
	if inv == nil {
		return nil
	}

	// If it's already COMPLETED or REJECTED, we don't need to do anything
	if inv.Status != StatusAccepted && inv.Status != StatusSent {
		return nil
	}

	// If they signed up after clicking the link, update status to COMPLETED and link the new User ID
	return s.repo.UpdateStatus(ctx, inv.ID, StatusCompleted, &userID)
}
