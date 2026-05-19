package auth

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
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
)

var (
	ErrEmailTaken      = errors.New("email already in use")
	ErrInvalidPassword = errors.New("invalid email or password")
	ErrNotFound        = errors.New("user not found")
)

type Service interface {
	Register(ctx context.Context, req *RqRegisterClinic) (*RsClinicDetail, error)
	Login(ctx context.Context, req *RqLoginClinic) (*RsToken, error)
	Logout(ctx context.Context, clinicID uuid.UUID, refreshToken string) error
	GetProfile(ctx context.Context, clinicID uuid.UUID) (*RsClinicDetail, error)
	VerifyEmail(ctx context.Context, tokenStr string) error
}

type service struct {
	repo     Repository
	cfg      *config.Config
	db       *sqlx.DB
	auditSvc audit.Service
}

func NewService(repo Repository, cfg *config.Config, db *sqlx.DB, auditSvc audit.Service) Service {
	return &service{
		repo:     repo,
		cfg:      cfg,
		db:       db,
		auditSvc: auditSvc,
	}
}

func (s *service) Register(ctx context.Context, req *RqRegisterClinic) (*RsClinicDetail, error) {
	// Verify if email is taken across this module table
	existing, err := s.repo.FindByEmail(ctx, req.Email)
	if err == nil && existing != nil {
		return nil, ErrEmailTaken
	}

	// Hash raw password payload using Argon2id/Bcrypt wrapper
	if req.Password == nil || *req.Password == "" {
		return nil, errors.New("password is required")
	}
	hashedPassword, err := util.GenerateHash(*req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to process credentials: %w", err)
	}

	// Assemble base structures
	clinicModel := &Clinic{
		ClinicName:  req.ClinicName,
		Description: req.Description,
		Email:       req.Email,
		Password:    &hashedPassword,
		Role:        req.Role,
		DocumentID:  req.DocumentID,
		ABN:         req.ABN,
		ACN:         req.ACN,
	}

	var createdClinic *Clinic
	var createdAddresses []ClinicAddress
	var createdContacts []ClinicContact
	var tokenID uuid.UUID

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		var txErr error
		createdClinic, txErr = s.repo.CreateClinic(ctx, clinicModel, tx)
		if txErr != nil {
			return txErr
		}

		// Store associated addresses
		for _, addr := range req.Addresses {
			dbAddr := &ClinicAddress{
				ClinicID:  createdClinic.ID,
				Address:   addr.Address,
				City:      addr.City,
				State:     addr.State,
				Postcode:  addr.Postcode,
				IsPrimary: addr.IsPrimary,
			}
			savedAddr, txErr := s.repo.CreateAddress(ctx, dbAddr, tx)
			if txErr != nil {
				return fmt.Errorf("failed to write clinic location breakdown line: %w", txErr)
			}
			createdAddresses = append(createdAddresses, *savedAddr)
		}

		// Store associated contacts
		for _, cont := range req.Contacts {
			dbCont := &ClinicContact{
				ClinicID:    createdClinic.ID,
				ContactType: cont.ContactType,
				Value:       cont.Value,
				Label:       cont.Label,
				IsPrimary:   cont.IsPrimary,
			}
			savedCont, txErr := s.repo.CreateContact(ctx, dbCont, tx)
			if txErr != nil {
				return fmt.Errorf("failed to write clinic identity routing point: %w", txErr)
			}
			createdContacts = append(createdContacts, *savedCont)
		}

		tokenID = uuid.New()
		vToken := &VerificationToken{
			ID:        tokenID,
			ClinicID:  createdClinic.ID,
			Role:      createdClinic.Role,
			Status:    TokenStatusPending,
			ExpiresAt: time.Now().Add(10 * time.Hour),
		}

		if err := s.repo.CreateVerificationToken(ctx, vToken, tx); err != nil {
			return fmt.Errorf("create verification token: %w", err)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	go func() {
		if err := s.sendVerificationEmail(createdClinic.Email, createdClinic.ClinicName, tokenID); err != nil {
			fmt.Printf("[CLINIC ERROR] Failed to send verification email: %v\n", err)
			s.auditSvc.LogSystemIssue(context.Background(), auditctx.ActionSystemError, "clinic.send_verification_email",
				err, createdClinic.ID.String(), createdClinic.ID.String(), auditctx.EntityUser, auditctx.ModuleAuth,
			)
		}
	}()

	meta := auditctx.GetMetadata(ctx)
	clinicIDStr := createdClinic.ID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: &clinicIDStr,
		UserID:     &clinicIDStr, // Clinic id
		Action:     auditctx.ActionClinicRegistered,
		Module:     auditctx.ModuleInvoice,
		EntityType: strPtr(auditctx.EntityInvoiceClinic),
		EntityID:   &clinicIDStr,
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	return toRsClinicDetail(createdClinic, createdAddresses, createdContacts), nil
}

func (s *service) Login(ctx context.Context, req *RqLoginClinic) (*RsToken, error) {
	clinic, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		_ = util.CompareHash(req.Password, "$2a$10$dummyhashfortimingnormalization000000000000000000000000")
		return nil, ErrInvalidPassword
	}

	if clinic.Password == nil || *clinic.Password == "" {
		return nil, ErrInvalidPassword
	}

	if err := util.CompareHash(req.Password, *clinic.Password); err != nil {
		return nil, ErrInvalidPassword
	}

	meta := auditctx.GetMetadata(ctx)
	clinicIDStr := clinic.ID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: &clinicIDStr,
		UserID:     &clinicIDStr,
		Action:     auditctx.ActionClinicLoggedIn,
		Module:     auditctx.ModuleInvoice,
		EntityType: strPtr(auditctx.EntitySession),
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	return s.issueTokens(ctx, clinic, clinicIDStr)
}

func (s *service) Logout(ctx context.Context, clinicID uuid.UUID, refreshToken string) error {
	sess, err := s.repo.FindSessionByRefreshToken(ctx, refreshToken)
	if err != nil {
		return err
	}

	// Security check: Don't let User A log out User B's session
	if sess.ClinicID != clinicID {
		s.auditSvc.LogSystemIssue(ctx, auditctx.ActionSystemError, "session.unauthorized_logout",
			errors.New("unauthorized session access attempt"),
			clinicID.String(), sess.ID.String(), auditctx.EntitySession, auditctx.ModuleAuth,
		)
		return errors.New("unauthorized session access")
	}

	if err := s.repo.DeleteSession(ctx, sess.ID); err != nil {
		return err
	}

	// Audit log: user logged out
	meta := auditctx.GetMetadata(ctx)
	sessIDStr := sess.ID.String()
	clinicIDStr := sess.ClinicID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: meta.PracticeID,
		UserID:     &clinicIDStr,
		Action:     auditctx.ActionClinicLoggedOut,
		Module:     auditctx.ModuleInvoice,
		EntityType: strPtr(auditctx.EntityClinicSession),
		EntityID:   &sessIDStr,
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	// Audit log:  Session revoked
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: meta.PracticeID,
		UserID:     &clinicIDStr,
		Action:     auditctx.ActionClinicSessionRevoked,
		Module:     auditctx.ModuleInvoice,
		EntityType: strPtr(auditctx.EntityClinicSession),
		EntityID:   &sessIDStr,
		BeforeState: map[string]interface{}{
			"session_id": sessIDStr,
			"clinic_id":  clinicIDStr,
			"revoked_at": time.Now(),
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})

	return nil
}

func (s *service) GetProfile(ctx context.Context, clinicID uuid.UUID) (*RsClinicDetail, error) {
	clinic, err := s.repo.FindByID(ctx, clinicID)
	if err != nil {
		return nil, ErrNotFound
	}

	addresses, err := s.repo.ListAddressesByClinicID(ctx, clinicID)
	if err != nil {
		return nil, err
	}

	contacts, err := s.repo.ListContactsByClinicID(ctx, clinicID)
	if err != nil {
		return nil, err
	}

	return toRsClinicDetail(clinic, addresses, contacts), nil
}

func (s *service) VerifyEmail(ctx context.Context, tokenStr string) error {
	tokenID, err := uuid.Parse(tokenStr)
	if err != nil {
		return errors.New("invalid token format")
	}

	token, err := s.repo.GetToken(ctx, tokenID)
	if err != nil {
		return errors.New("verification link not found")
	}

	if token.Status != TokenStatusPending {
		return fmt.Errorf("this link has already been %s", strings.ToLower(token.Status))
	}

	if time.Now().After(token.ExpiresAt) {
		return errors.New("verification link has expired")
	}

	if err := s.repo.MarkUserVerified(ctx, token); err != nil {
		return err
	}

	// Audit log: Email Verified
	meta := auditctx.GetMetadata(ctx)
	userIDStr := token.ClinicID.String()
	tokenIDStr := token.ID.String()

	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: meta.PracticeID,
		UserID:     &userIDStr,
		Action:     auditctx.ActionEmailVerified,
		Module:     auditctx.ModuleAuth,
		EntityType: strPtr(auditctx.EntityVerificationToken),
		EntityID:   &tokenIDStr,
		BeforeState: map[string]interface{}{
			"status": token.Status,
		},
		AfterState: map[string]interface{}{
			"status": "USED",
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})
	return nil
}

// ==========================================
// HELPERS
// ==========================================

func toRsClinicDetail(c *Clinic, addrs []ClinicAddress, conts []ClinicContact) *RsClinicDetail {
	rsAddrs := make([]RsClinicAddress, 0, len(addrs))
	for _, a := range addrs {
		rsAddrs = append(rsAddrs, RsClinicAddress{
			ID:        a.ID,
			Address:   a.Address,
			City:      a.City,
			State:     a.State,
			Postcode:  a.Postcode,
			IsPrimary: a.IsPrimary,
		})
	}

	rsConts := make([]RsClinicContact, 0, len(conts))
	for _, ct := range conts {
		rsConts = append(rsConts, RsClinicContact{
			ID:          ct.ID,
			ContactType: ct.ContactType,
			Value:       ct.Value,
			Label:       ct.Label,
			IsPrimary:   ct.IsPrimary,
		})
	}

	return &RsClinicDetail{
		ID:          c.ID,
		ClinicName:  c.ClinicName,
		Email:       c.Email,
		Role:        c.Role,
		Verified:    c.Verified,
		Description: c.Description,
		DocumentID:  c.DocumentID,
		ABN:         c.ABN,
		ACN:         c.ACN,
		Addresses:   rsAddrs,
		Contacts:    rsConts,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

func strPtr(s string) *string {
	return &s
}

func (s *service) issueTokens(ctx context.Context, clinic *Clinic, clinicID string) (*RsToken, error) {
	roleString := util.RoleClinic // Defult to Role CLINIC
	if clinic.Role != nil {
		roleString = *clinic.Role
	}

	accessToken, err := util.SignToken(clinic.ID.String(), clinicID, roleString, 15*time.Hour, s.cfg.JWTSecret)
	if err != nil {
		return nil, err
	}

	refreshToken, err := util.SignToken(clinic.ID.String(), clinicID, roleString, 7*24*time.Hour, s.cfg.JWTRefreshSecret)
	if err != nil {
		return nil, err
	}

	ua := middleware.UserAgentFromCtx(ctx)
	ip := middleware.IPFromCtx(ctx)

	sess := &Session{
		ID:           uuid.New(),
		ClinicID:     clinic.ID,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
	}
	if ua != "" {
		sess.UserAgent = &ua
	}
	if ip != "" {
		sess.IPAddress = &ip
	}

	if _, err := s.repo.CreateSession(ctx, sess); err != nil {
		s.auditSvc.LogSystemIssue(ctx, auditctx.ActionSystemError, "clinic.create_session",
			err, clinic.ID.String(), clinic.ID.String(), auditctx.EntityClinicSession, auditctx.ModuleInvoice,
		)
		return nil, err
	}

	// Audit log : Session Created
	meta := auditctx.GetMetadata(ctx)
	sessIDStr := sess.ID.String()
	clinicIDStr := clinic.ID.String()

	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: &clinicID,
		UserID:     &clinicIDStr,
		Action:     auditctx.ActionClinicSessionCreated,
		Module:     auditctx.ModuleInvoice,
		EntityType: strPtr(auditctx.EntityClinicSession),
		EntityID:   &sessIDStr,
		AfterState: map[string]interface{}{
			"session_id": sessIDStr,
			"expires_at": sess.ExpiresAt,
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})

	return &RsToken{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Role:         clinic.Role,
	}, nil
}

// Helper function for sending verification email via Resend API
func (s *service) sendVerificationEmail(to string, clinicName string, tokenID uuid.UUID) error {
	url := "https://api.resend.com/emails"
	apikey := s.cfg.ResendAPIKey

	baseUrl, err := s.cfg.GetBaseURL()
	if err != nil {
		return err
	}

	verificationLink := fmt.Sprintf("%s/verify-email?token=%s", baseUrl, tokenID)
	expiryTime := "10 minutes"

	payload := map[string]interface{}{
		"from":    "Acareca <hardik@zenithive.digital>",
		"to":      []string{to},
		"subject": "Verify your Acareca account",
		"html": fmt.Sprintf(`
			<div style="font-family: sans-serif; color: #333; max-width: 600px; margin: auto; border: 1px solid #eee; padding: 20px;">
				<h2 style="color: #1a73e8;">Verify your email</h2>
				<p>Hi %s,</p>
				<p>Thank you for signing up with Acareca! To complete your registration and activate your account, please verify your email address by clicking the button below:</p>
				<div style="text-align: center; margin: 30px 0;">
					<a href="%s" style="background-color: #1a73e8; color: white; padding: 14px 28px; text-decoration: none; border-radius: 4px; font-weight: bold; display: inline-block;">
						Verify My Account
					</a>
				</div>
				<p style="font-size: 14px; color: #666;">If the button above doesn’t work, you can also copy and paste the following link into your browser:</p>
				<p style="font-size: 12px; word-break: break-all; color: #1a73e8;">%s</p>
				<p style="font-size: 14px; color: #666;">This verification link will expire in <strong>%s</strong>.</p>
				<hr style="border: none; border-top: 1px solid #eee; margin: 20px 0;" />
				<p style="font-size: 12px; color: #888;">If you did not create this account, you can safely ignore this email.</p>
				<p style="font-size: 12px; color: #888;">Best regards,<br>The Acareca Team</p>
			</div>
		`, clinicName, verificationLink, verificationLink, expiryTime),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+apikey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend error: %s", string(body))
	}

	return nil
}
