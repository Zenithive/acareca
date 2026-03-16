package invitation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type Service interface {
	SendInvite(ctx context.Context, practitionerID uuid.UUID, req *RqSendInvitation) (*RsInvitation, error)
	ProcessInvitation(ctx context.Context, inviteID uuid.UUID, action string) (*RsInviteProcess, error)
	FinalizeRegistrationInternal(ctx context.Context, email string, userID uuid.UUID) error
}

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
	inviteLink := fmt.Sprintf("https://acareca.com/invite/%s", invite.ID)

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

	// Resend API payload structure
	payload := map[string]interface{}{
		"from":    "Acareca <onboarding@resend.dev>", // Note: for testing
		"to":      []string{to},
		"subject": fmt.Sprintf("Invitation: Manage %s's files on Acareca", senderName),
		"html": fmt.Sprintf(`
			<div style="font-family: sans-serif; color: #333; line-height: 1.6;">
				<h2>Hello,</h2>
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
		`, senderName, link, senderName, to),
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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("resend api returned status: %d", resp.StatusCode)
	}

	return nil
}

func (s *service) ProcessInvitation(ctx context.Context, inviteID uuid.UUID, action string) (*RsInviteProcess, error) {
	inv, err := s.repo.GetByID(ctx, inviteID)
	if err != nil || inv == nil {
		return nil, errors.New("Invitation not found")
	}

	if time.Now().After(inv.ExpiresAt) {
		return nil, errors.New("Invitation expired")
	}

	res := &RsInviteProcess{InvitationID: inv.ID}

	// Safety Check
	if inv.Status == StatusRejected || inv.Status == StatusCompleted {
		res.Status = inv.Status
		res.IsFound = (inv.Status == StatusCompleted)
		return res, nil
	}

	// Handle STATUS REJECTED
	if action == "reject" {
		if err := s.repo.UpdateStatus(ctx, inviteID, StatusRejected, nil); err != nil {
			return nil, err
		}
		res.Status = StatusRejected
		return res, nil
	}

	existingUserID, err := s.repo.GetUserIDByEmail(ctx, inv.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check user existence: %w", err)
	}

	if existingUserID != nil {
		// User exists in tbl_user
		if err := s.repo.UpdateStatus(ctx, inviteID, StatusCompleted, existingUserID); err != nil {
			return nil, err
		}
		res.Status = StatusCompleted
		res.IsFound = true // Redirect to login
	} else {
		// User does not exist
		if err := s.repo.UpdateStatus(ctx, inviteID, StatusAccepted, nil); err != nil {
			return nil, err
		}
		res.Status = StatusAccepted
		res.IsFound = false // Redirect to register
	}

	return res, nil
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
