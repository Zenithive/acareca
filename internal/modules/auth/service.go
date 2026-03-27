package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/business/invitation"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const providerGoogle = "google"

var (
	ErrEmailTaken      = errors.New("email already in use")
	ErrInvalidPassword = errors.New("invalid credentials")
	ErrOAuthOnly       = errors.New("account uses Google sign-in; password login is not available")
)

type OnUserCreated func(ctx context.Context, userID string) error

type Service interface {
	Register(ctx context.Context, req *RqUser) (*RsUser, error)
	Login(ctx context.Context, req *RqLogin) (*RsToken, error)
	Logout(ctx context.Context, userID uuid.UUID, refreshToken string) error
	GoogleAuthURL(state string) *RsGoogleAuthURL
	GoogleCallback(ctx context.Context, code string) (*RsToken, error)

	VerifyEmail(ctx context.Context, tokenStr string) error
	ChangePassword(ctx context.Context, pracID uuid.UUID, req *RqChangePassword) error

	UpdateProfile(ctx context.Context, userID uuid.UUID, req *RqUpdateUser) (*RsUser, error)
	DeleteUser(ctx context.Context, userID uuid.UUID) error

	ForgotPassword(ctx context.Context, req *RqForgotPassword) error
	ResetPassword(ctx context.Context, req *RqResetPassword) error
}

type service struct {
	repo             Repository
	cfg              *config.Config
	db               *sqlx.DB
	oauthConfig      *oauth2.Config
	practitionerSvc  practitioner.IService
	auditSvc         audit.Service
	invitationSvc    invitation.Service
	practitionerRepo practitioner.Repository
	accountantSvc    accountant.IService
	inviteRepo       invitation.Repository
}

func NewService(repo Repository, cfg *config.Config, db *sqlx.DB, practitionerSvc practitioner.IService, auditSvc audit.Service, invitationSvc invitation.Service, practitionerRepo practitioner.Repository, accountantSvc accountant.IService, inviteRepo invitation.Repository) Service {
	oauthCfg := &oauth2.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.GoogleRedirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
	return &service{
		repo:             repo,
		cfg:              cfg,
		oauthConfig:      oauthCfg,
		db:               db,
		practitionerSvc:  practitionerSvc,
		auditSvc:         auditSvc,
		invitationSvc:    invitationSvc,
		practitionerRepo: practitionerRepo,
		accountantSvc:    accountantSvc,
		inviteRepo:       inviteRepo}
}

func (s *service) Register(ctx context.Context, req *RqUser) (*RsUser, error) {
	existing, err := s.repo.FindByEmail(ctx, req.Email)
	if err == nil && existing != nil {
		// Check if existing user is already verified
		isVerified, _ := s.isUserVerified(ctx, existing)
		if isVerified {
			return nil, ErrEmailTaken
		}
		// If they exist but aren't verified, redirect them to verify
		return nil, errors.New("this email is registered but not verified. Please check your email to verify your account")
	}

	// Role Check: Check if email is in tbl_invitation
	invite, err := s.invitationSvc.GetInvitationByEmailInternal(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to verify invitation status: %w", err)
	}

	// Determine Role: If invitation exists, role is accountant.
	if invite != nil {
		req.Role = util.RoleAccountant
	} else { // Default role if no invitation is found.
		req.Role = util.RolePractitioner
	}

	hashedPassword, err := util.GenerateHash(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	u := req.ToDBModel()
	u.Password = &hashedPassword
	u.Role = req.Role

	var created *User
	var entityID uuid.UUID
	var p *practitioner.RsPractitioner
	var a *accountant.RsAccountant
	var tokenID uuid.UUID

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		var txErr error
		created, txErr = s.repo.CreateUser(ctx, u, tx)
		if txErr != nil {
			if errors.Is(txErr, ErrEmailTaken) {
				return ErrEmailTaken
			}
			return txErr
		}

		switch created.Role {
		case util.RolePractitioner:
			p, txErr = s.practitionerSvc.CreatePractitioner(ctx, &practitioner.RqCreatePractitioner{UserID: created.ID.String()}, tx)
			if txErr == nil {
				entityID = p.ID
			}
		case util.RoleAccountant:
			a, txErr = s.accountantSvc.CreateAccountant(ctx, &accountant.RqCreateAccountant{UserID: created.ID.String()}, tx)
			if txErr == nil {
				entityID = a.ID
			}
		}
		if txErr != nil {
			return fmt.Errorf("create role: %w", txErr)
		}

		// Generate Verification Token
		tokenID = uuid.New()
		vToken := &VerificationToken{
			ID:        tokenID,
			EntityID:  entityID,
			Role:      &created.Role,
			Status:    TokenStatusPending,
			ExpiresAt: time.Now().Add(10 * time.Hour),
		}

		if err := s.repo.CreateVerificationToken(ctx, tx, vToken); err != nil {
			return fmt.Errorf("create verification token: %w", err)
		}

		if created.Role == util.RoleAccountant {
			if a == nil {
				return errors.New("failed to retrieve accountant profile for invitation finalization")
			}
			// This function looks for an invitation by email and links it to the accountant id
			err = s.invitationSvc.FinalizeRegistrationInternal(ctx, created.Email, a.ID)
			if err != nil {
				return fmt.Errorf("[DEBUG] finalize invitation: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Send Email Asynchronously
	go func() {
		if err := s.sendVerificationEmail(created.Email, created.FirstName, tokenID); err != nil {
			fmt.Printf("[AUTH ERROR] Failed to send verification email: %v\n", err)
		}
	}()

	// Audit log: user registration
	meta := auditctx.GetMetadata(ctx)
	userIDStr := created.ID.String()
	entityIDStr := entityID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: &entityIDStr,
		UserID:     &userIDStr,
		Action:     auditctx.ActionUserRegistered,
		Module:     auditctx.ModuleAuth,
		EntityType: strPtr(auditctx.EntityUser),
		EntityID:   &userIDStr,
		AfterState: sanitizeUser(created),
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	return created.ToRsUser(), nil
}

func (s *service) Login(ctx context.Context, req *RqLogin) (*RsToken, error) {
	user, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		// Run a dummy hash comparison to prevent timing-based user enumeration
		_ = util.CompareHash(req.Password, "$2a$10$dummyhashfortimingnormalization000000000000000000000000")
		return nil, ErrInvalidPassword
	}

	if user.Password == nil || *user.Password == "" {
		_ = util.CompareHash(req.Password, "$2a$10$dummyhashfortimingnormalization000000000000000000000000")
		return nil, ErrOAuthOnly
	}

	if err := util.CompareHash(req.Password, *user.Password); err != nil {
		return nil, ErrInvalidPassword
	}

	// User Verification Check
	isVerified, err := s.isUserVerified(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("verification check: %w", err)
	}
	if !isVerified {
		return nil, errors.New("account not verified. Please check your email to verify your account")
	}

	// Resolve the specific Entity ID for the JWT
	entityID, err := s.resolveEntityID(ctx, user)
	if err != nil {
		return nil, err
	}

	// Audit log: user login
	meta := auditctx.GetMetadata(ctx)
	userIDStr := user.ID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: &entityID,
		UserID:     &userIDStr,
		Action:     auditctx.ActionUserLoggedIn,
		Module:     auditctx.ModuleAuth,
		EntityType: strPtr(auditctx.EntitySession),
		EntityID:   nil,
		AfterState: map[string]interface{}{"email": user.Email},
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	return s.issueTokens(ctx, user, entityID)
}

func (s *service) Logout(ctx context.Context, userID uuid.UUID, refreshToken string) error {
	sess, err := s.repo.FindSessionByRefreshToken(ctx, refreshToken)
	if err != nil {
		return err
	}

	// Security check: Don't let User A log out User B's session
	if sess.UserID != userID {
		return errors.New("unauthorized session access")
	}

	if err := s.repo.DeleteSession(ctx, sess.ID); err != nil {
		return err
	}

	// Audit log: user logged out
	meta := auditctx.GetMetadata(ctx)
	sessIDStr := sess.ID.String()
	userIDStr := sess.UserID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: meta.PracticeID,
		UserID:     &userIDStr,
		Action:     auditctx.ActionUserLoggedOut,
		Module:     auditctx.ModuleAuth,
		EntityType: strPtr(auditctx.EntitySession),
		EntityID:   &sessIDStr,
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	// Audit log:  Session revoked
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: meta.PracticeID,
		UserID:     &userIDStr,
		Action:     auditctx.ActionSessionRevoked,
		Module:     auditctx.ModuleAuth,
		EntityType: strPtr(auditctx.EntitySession),
		EntityID:   &sessIDStr,
		BeforeState: map[string]interface{}{
			"session_id": sessIDStr,
			"user_id":    userIDStr,
			"revoked_at": time.Now(),
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})

	return nil
}

func (s *service) GoogleAuthURL(state string) *RsGoogleAuthURL {
	url := s.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	return &RsGoogleAuthURL{URL: url}
}

func (s *service) GoogleCallback(ctx context.Context, code string) (*RsToken, error) {
	oauthToken, err := s.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange oauth code: %w", err)
	}

	googleUser, err := s.fetchGoogleUserInfo(ctx, oauthToken)
	if err != nil {
		return nil, err
	}

	user, findErr := s.repo.FindByEmail(ctx, googleUser.Email)
	isNewUser := false

	if err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		if findErr != nil {
			if !errors.Is(findErr, ErrNotFound) {
				return findErr
			}
			var createErr error
			user, createErr = s.repo.CreateUser(ctx, &User{
				Email:     googleUser.Email,
				FirstName: googleUser.FirstName,
				LastName:  googleUser.LastName,
			}, tx)
			if createErr != nil {
				return createErr
			}
			isNewUser = true
		}

		expiresAt := oauthToken.Expiry
		accessTokenStr := oauthToken.AccessToken
		refreshTokenStr := oauthToken.RefreshToken

		ap := &AuthProvider{
			UserID:         user.ID,
			Provider:       providerGoogle,
			AccessToken:    &accessTokenStr,
			TokenExpiresAt: &expiresAt,
		}
		if refreshTokenStr != "" {
			ap.RefreshToken = &refreshTokenStr
		}

		if _, err := s.repo.UpsertAuthProvider(ctx, ap, tx); err != nil {
			return err
		}

		if isNewUser {
			_, err = s.practitionerSvc.CreatePractitioner(ctx, &practitioner.RqCreatePractitioner{UserID: user.ID.String()}, tx)
			if err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("google oauth transaction: %w", err)
	}

	// Audit log: OAuth login/registration — only reached after a successful commit
	meta := auditctx.GetMetadata(ctx)
	userIDStr := user.ID.String()
	action := auditctx.ActionUserLoggedIn
	if isNewUser {
		action = auditctx.ActionUserRegistered
	}
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: meta.PracticeID,
		UserID:     &userIDStr,
		Action:     action,
		Module:     auditctx.ModuleAuth,
		EntityType: strPtr(auditctx.EntityUser),
		EntityID:   &userIDStr,
		AfterState: map[string]interface{}{"email": user.Email, "provider": "google"},
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	practitionerID, err := s.practitionerSvc.GetPractitionerByUserID(ctx, user.ID.String())
	if err != nil {
		return nil, err
	}

	return s.issueTokens(ctx, user, practitionerID.ID.String())
}

func (s *service) fetchGoogleUserInfo(ctx context.Context, token *oauth2.Token) (*GoogleUserInfo, error) {
	client := s.oauthConfig.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("fetch google user info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read google user info response: %w", err)
	}

	var info GoogleUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("parse google user info: %w", err)
	}
	return &info, nil
}

func (s *service) issueTokens(ctx context.Context, user *User, entityID string) (*RsToken, error) {
	accessToken, err := util.SignToken(user.ID.String(), entityID, user.Role, 15*time.Hour, s.cfg.JWTSecret)
	if err != nil {
		return nil, err
	}

	refreshToken, err := util.SignToken(user.ID.String(), entityID, user.Role, 7*24*time.Hour, s.cfg.JWTRefreshSecret)
	if err != nil {
		return nil, err
	}

	ua := middleware.UserAgentFromCtx(ctx)
	ip := middleware.IPFromCtx(ctx)

	sess := &Session{
		ID:           uuid.New(),
		UserID:       user.ID,
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
		return nil, err
	}

	// Audit log : Session Created
	meta := auditctx.GetMetadata(ctx)
	sessIDStr := sess.ID.String()
	userIDStr := user.ID.String()

	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: &entityID,
		UserID:     &userIDStr,
		Action:     auditctx.ActionSessionCreated,
		Module:     auditctx.ModuleAuth,
		EntityType: strPtr(auditctx.EntitySession),
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
		Role:         &user.Role,
	}, nil
}

// Helper functions for audit logging

func strPtr(s string) *string {
	return &s
}

func sanitizeUser(u *User) map[string]interface{} {
	return map[string]interface{}{
		"id":         u.ID.String(),
		"email":      u.Email,
		"first_name": u.FirstName,
		"last_name":  u.LastName,
	}
}

// Helper function for sending verification email via Resend API
func (s *service) sendVerificationEmail(to string, firstName string, tokenID uuid.UUID) error {
	url := "https://api.resend.com/emails"
	apikey := s.cfg.ResendAPIKey

	var baseUrl string
	if s.cfg.Env == "dev" {
		baseUrl = s.cfg.DevUrl
	} else {
		baseUrl = s.cfg.LocalUrl
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
		`, firstName, verificationLink, verificationLink, expiryTime),
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
	userIDStr := token.EntityID.String()
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

func (s *service) ChangePassword(ctx context.Context, pracID uuid.UUID, req *RqChangePassword) error {
	// Find user by practitioner ID
	user, err := s.repo.FindByPractitionerID(ctx, pracID)
	if err != nil {
		return fmt.Errorf("user not found for practitioner: %w", err)
	}

	// Prevent password change for OAuth-only accounts
	if user.Password == nil || *user.Password == "" {
		return ErrOAuthOnly
	}

	// Hash new password
	newHashedPassword, err := util.GenerateHash(req.NewPassword)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}

	// Update new password in DB
	if err := s.repo.UpdatePassword(ctx, user.ID, newHashedPassword); err != nil {
		return err
	}

	// Audit log : Password Change
	meta := auditctx.GetMetadata(ctx)
	userIDStr := user.ID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		UserID:     &userIDStr,
		Action:     auditctx.ActionPasswordChanged,
		Module:     auditctx.ModuleAuth,
		EntityType: strPtr(auditctx.EntityUser),
		EntityID:   &userIDStr,
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	return nil
}

func (s *service) UpdateProfile(ctx context.Context, userID uuid.UUID, req *RqUpdateUser) (*RsUser, error) {
	// 1. Fetch existing user
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	beforeState := sanitizeUser(user)

	// 2. Map new data ONLY if provided (Partial Update)
	if req.Email != nil {
		// check if email is different and if it's already taken
		if *req.Email != user.Email {
			if _, err := s.repo.FindByEmail(ctx, *req.Email); err == nil {
				return nil, ErrEmailTaken
			}
		}
		user.Email = *req.Email
	}
	if req.FirstName != nil {
		user.FirstName = *req.FirstName
	}
	if req.LastName != nil {
		user.LastName = *req.LastName
	}
	if req.Phone != nil {
		user.Phone = req.Phone // Phone is *string in User model, so no dereference needed
	}

	updated, err := s.repo.UpdateUser(ctx, user, nil)
	if err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}

	// 4. Audit log: Profile Update
	meta := auditctx.GetMetadata(ctx)
	userIDStr := updated.ID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID:  meta.PracticeID,
		UserID:      &userIDStr,
		Action:      auditctx.ActionUserUpdated,
		Module:      auditctx.ModuleAuth,
		EntityType:  strPtr(auditctx.EntityUser),
		EntityID:    &userIDStr,
		BeforeState: beforeState,
		AfterState:  sanitizeUser(updated),
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
	})

	return updated.ToRsUser(), nil
}

func (s *service) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	// 1. Fetch user to confirm existence and for audit logging
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	beforeState := sanitizeUser(user)

	// 2. Perform the soft delete in repository
	if err := s.repo.DeleteUser(ctx, userID, nil); err != nil {
		return fmt.Errorf("delete user service: %w", err)
	}

	if err := s.practitionerRepo.DeleteByUserID(ctx, userID); err != nil {
		// We log the error but might not want to fail the whole request
		// if the user was just a standard user without a practitioner profile.
		fmt.Printf("INFO: No practitioner profile deleted for user %s: %v\n", userID, err)
	}

	// 3. Audit Log
	meta := auditctx.GetMetadata(ctx)
	userIDStr := userID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID:  meta.PracticeID,
		UserID:      &userIDStr,
		Action:      auditctx.ActionUserDeleted,
		Module:      auditctx.ModuleAuth,
		EntityType:  strPtr(auditctx.EntityUser),
		EntityID:    &userIDStr,
		BeforeState: beforeState,
		AfterState:  nil,
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
	})

	return nil
}

// Helper to resolve PractitionerID or AccountantID based on role
func (s *service) resolveEntityID(ctx context.Context, user *User) (string, error) {
	switch user.Role {
	case util.RolePractitioner:
		p, err := s.practitionerSvc.GetPractitionerByUserID(ctx, user.ID.String())
		if err != nil {
			return "", err
		}
		return p.ID.String(), nil
	case util.RoleAccountant:
		acc, err := s.accountantSvc.GetAccountantByUserID(ctx, user.ID.String())
		if err != nil {
			return "", err
		}
		return acc.ID.String(), nil
	default:
		return user.ID.String(), nil
	}
}

func (s *service) SendForgotPasswordEmail(to string, firstName string, token string, role string) error {
	url := "https://api.resend.com/emails"
	apikey := s.cfg.ResendAPIKey

	var baseURL string

	if s.cfg.Env == "dev" {
		baseURL = s.cfg.DevUrl
	} else {
		baseURL = s.cfg.LocalUrl
	}

	// 2. Fallback Safety Net: In case envs are missing
	if baseURL == "" {

		return fmt.Errorf("system configuration error: frontend application URL is not defined")
	}

	resetLink := fmt.Sprintf("%s/reset-password?token=%s", baseURL, token)
	expiryTime := "15 minutes"

	payload := map[string]interface{}{
		"from":    "Acareca <hardik@zenithive.digital>",
		"to":      []string{to},
		"subject": "Reset your Acareca password",
		"html": fmt.Sprintf(`
            <div style="font-family: sans-serif; color: #333; max-width: 600px; margin: auto; border: 1px solid #eee; padding: 20px;">
                <h2 style="color: #d93025;">Password Reset Request</h2>
                <p>Hi %s,</p>
                <p>We received a request to reset your password for your Acareca account. Click the button below to choose a new password:</p>
                <div style="text-align: center; margin: 30px 0;">
                    <a href="%s" style="background-color: #d93025; color: white; padding: 14px 28px; text-decoration: none; border-radius: 4px; font-weight: bold; display: inline-block;">
                        Reset My Password
                    </a>
                </div>
                <p style="font-size: 14px; color: #666;">If the button above doesn’t work, copy and paste this link into your browser:</p>
                <p style="font-size: 12px; word-break: break-all; color: #d93025;">%s</p>
                <p style="font-size: 14px; color: #666;">This link will expire in <strong>%s</strong> for security reasons.</p>
                <hr style="border: none; border-top: 1px solid #eee; margin: 20px 0;" />
                <p style="font-size: 12px; color: #888;">If you did not request a password reset, you can safely ignore this email; your password will remain unchanged.</p>
                <p style="font-size: 12px; color: #888;">Best regards,<br>The Acareca Team</p>
            </div>
        `, firstName, resetLink, resetLink, expiryTime),
	}

	// Standard HTTP request logic (same as your verification helper)
	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
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

func (s *service) ForgotPassword(ctx context.Context, req *RqForgotPassword) error {
	// 1. Find user and their role
	user, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil
	}

	// 2. NEW: Invitation Status Guard for Accountants
	if user.Role == util.RoleAccountant {
		// You likely have a method to get the invitation by email
		inv, err := s.inviteRepo.GetByEmail(ctx, req.Email)
		if err != nil || inv == nil {
			return errors.New("no active account found")
		}

		// Block if the invitation isn't finished yet
		if inv.Status != "COMPLETED" {
			return errors.New("Please complete your account setup via the invitation link first")
		}
	}

	// 2. Generate Raw Token and Hash it
	rawToken := uuid.New().String()

	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	// 3. Save to DB (expires in 15 mins)
	expiresAt := time.Now().Add(15 * time.Minute)
	err = s.repo.SaveResetToken(ctx, user.ID.String(), tokenHash, expiresAt)
	if err != nil {
		return err
	}

	// 4. Send Email via Resend helper
	return s.SendForgotPasswordEmail(user.Email, user.FirstName, rawToken, user.Role)
}

func (s *service) ResetPassword(ctx context.Context, req *RqResetPassword) error {

	// This must match the SHA-256 hash we saved in ForgotPassword
	hash := sha256.Sum256([]byte(req.Token))
	tokenHash := hex.EncodeToString(hash[:])

	// 2. Hash the NEW password using Argon2id
	newPasswordHash, err := util.GenerateHash(req.NewPassword)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	return s.repo.CompletePasswordReset(ctx, tokenHash, newPasswordHash)
}

// Helper to check if user is verified
func (s *service) isUserVerified(ctx context.Context, user *User) (bool, error) {
	var verified bool
	var err error

	switch user.Role {
	case util.RolePractitioner:
		err = s.db.GetContext(ctx, &verified, "SELECT verified FROM tbl_practitioner WHERE user_id = $1", user.ID)
	case util.RoleAccountant:
		err = s.db.GetContext(ctx, &verified, "SELECT verified FROM tbl_accountant WHERE user_id = $1", user.ID)
	default:
		return false, fmt.Errorf("verification check failed: '%s %s'", user.ID, user.Role)
	}

	if err != nil {
		return false, fmt.Errorf("could not verify account status: %w", err)
	}
	return verified, nil
}
