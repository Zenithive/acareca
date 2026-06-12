package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/business/admin"
	"github.com/iamarpitzala/acareca/internal/modules/business/invitation"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	"github.com/iamarpitzala/acareca/internal/modules/business/subscription"
	filemod "github.com/iamarpitzala/acareca/internal/modules/file"
	"github.com/iamarpitzala/acareca/internal/modules/notification/preference"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/mail"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
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
	VerifyEmail(ctx context.Context, tokenStr string) (string, error)
	ChangePassword(ctx context.Context, pracID uuid.UUID, req *RqChangePassword) error
	GetProfile(ctx context.Context, userID uuid.UUID) (*RsUser, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, req *RqUpdateUser) (*RsUser, error)
	DeleteUser(ctx context.Context, userID uuid.UUID) error
	ForgotPassword(ctx context.Context, req *RqForgotPassword) error
	ResetPassword(ctx context.Context, req *RqResetPassword) error

	GetUserByID(ctx context.Context, entityID uuid.UUID, EntityType util.ActorType) (*User, error)
}

type service struct {
	repo                Repository
	cfg                 *config.Config
	db                  *sqlx.DB
	oauthConfig         *oauth2.Config
	mailer              *mail.Client
	practitionerSvc     practitioner.IService
	auditSvc            audit.Service
	invitationSvc       invitation.Service
	practitionerRepo    practitioner.Repository
	accountantSvc       accountant.IService
	adminSvc            admin.IService
	inviteRepo          invitation.Repository
	fileRepo            filemod.Repository
	PreferenceSvc       preference.IService
	practitionerSubRepo subscription.Repository
}

func NewService(repo Repository, cfg *config.Config, db *sqlx.DB, practitionerSvc practitioner.IService, auditSvc audit.Service, invitationSvc invitation.Service, practitionerRepo practitioner.Repository, accountantSvc accountant.IService, adminSvc admin.IService, inviteRepo invitation.Repository, fileRepo filemod.Repository, preferenceSvc preference.IService, practitionerSubRepo subscription.Repository) Service {
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
		repo:                repo,
		cfg:                 cfg,
		oauthConfig:         oauthCfg,
		db:                  db,
		mailer:              mail.NewClient(cfg.ResendAPIKey, cfg.SenderEmail),
		practitionerSvc:     practitionerSvc,
		auditSvc:            auditSvc,
		invitationSvc:       invitationSvc,
		practitionerRepo:    practitionerRepo,
		accountantSvc:       accountantSvc,
		adminSvc:            adminSvc,
		inviteRepo:          inviteRepo,
		fileRepo:            fileRepo,
		PreferenceSvc:       preferenceSvc,
		practitionerSubRepo: practitionerSubRepo,
	}
}

func (s *service) Register(ctx context.Context, req *RqUser) (*RsUser, error) {
	existing, err := s.repo.FindByEmail(ctx, req.Email)
	if err == nil && existing != nil {
		isVerified, _ := s.isUserVerified(ctx, existing)
		if isVerified {
			return nil, ErrEmailTaken
		}
		return nil, errors.New("this email is registered but not verified. Please check your email to verify your account")
	}

	invite, err := s.invitationSvc.GetInvitationByEmailInternal(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to verify invitation status: %w", err)
	}

	assignedRole := util.RolePractitioner
	if invite != nil {
		assignedRole = util.RoleAccountant
	}

	hashedPassword, err := util.GenerateHash(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	u := req.ToDBModel()
	u.Password = &hashedPassword
	u.Role = assignedRole

	var created *User
	var entityID uuid.UUID
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
			p, txErr2 := s.practitionerSvc.CreatePractitioner(ctx, &practitioner.RqCreatePractitioner{
				UserID:     created.ID.String(),
				EntityType: req.EntityType,
				EntityName: req.EntityName,
				ABN:        req.ABN,
				ACN:        req.ACN,
				Address:    req.Address,
				Profession: req.Profession,
			}, tx)
			if txErr2 != nil {
				return fmt.Errorf("create practitioner: %w", txErr2)
			}
			entityID = p.ID
		case util.RoleAccountant:
			a, txErr = s.accountantSvc.CreateAccountant(ctx, &accountant.RqCreateAccountant{
				UserID:     created.ID.String(),
				EntityType: req.EntityType,
				EntityName: req.EntityName,
				ABN:        req.ABN,
				ACN:        req.ACN,
				Address:    req.Address,
				Profession: req.Profession,
			}, tx)
			if txErr == nil {
				entityID = a.ID
			}
		}
		if txErr != nil {
			return fmt.Errorf("create role: %w", txErr)
		}

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
			if err := s.invitationSvc.FinalizeRegistrationInternal(ctx, tx, created.Email, a.ID); err != nil {
				return fmt.Errorf("finalize invitation: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	go func() {
		baseUrl, err := s.cfg.GetBaseURL()
		if err != nil {
			s.logSystemError(context.Background(), "auth.send_verification_email", err, created.ID.String(), entityID.String())
			return
		}
		link := fmt.Sprintf("%s/auth/verify-email?token=%s", baseUrl, tokenID)
		if err := s.mailer.SendVerificationEmail(created.Email, created.FirstName, link); err != nil {
			fmt.Printf("[AUTH ERROR] Failed to send verification email: %v\n", err)
			s.logSystemError(context.Background(), "auth.send_verification_email", err, created.ID.String(), entityID.String())
		}
	}()

	userIDStr := created.ID.String()
	entityIDStr := entityID.String()
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		PracticeID: &entityIDStr,
		UserID:     &userIDStr,
		Action:     auditctx.ActionUserRegistered,
		Module:     auditctx.ModuleAuth,
		EntityType: lo.ToPtr(auditctx.EntityUser),
		EntityID:   &userIDStr,
		AfterState: sanitizeUser(created),
	})

	return created.ToRsUser(), nil
}

func (s *service) Login(ctx context.Context, req *RqLogin) (*RsToken, error) {
	user, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
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

	// Check email verification first
	if user.Role != util.RoleAdmin {
		isVerified, err := s.isUserVerified(ctx, user)
		if err != nil {
			return nil, fmt.Errorf("verification check: %w", err)
		}
		if !isVerified {
			return nil, err
		}
	}

	entityID, err := s.resolveEntityID(ctx, user)
	if err != nil {
		return nil, err
	}

	userIDStr := user.ID.String()
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		PracticeID: &entityID,
		UserID:     &userIDStr,
		Action:     auditctx.ActionUserLoggedIn,
		Module:     auditctx.ModuleAuth,
		EntityType: lo.ToPtr(auditctx.EntitySession),
		AfterState: map[string]interface{}{"email": user.Email},
	})

	rs, err := s.issueTokens(ctx, user, entityID)
	if err != nil {
		return nil, fmt.Errorf("issue tokens: %w", err)
	}
	if user.Role == util.RoleAdmin {
		err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
			if err := s.PreferenceSvc.PreferenceSetting(ctx, tx, user.ID, uuid.MustParse(entityID), user.Role); err != nil {
				log.Printf("failed to set notification preferences for user %s: %v", user.ID, err)
			}
			return nil
		})

	}
	return rs, err
}

func (s *service) Logout(ctx context.Context, userID uuid.UUID, refreshToken string) error {
	sess, err := s.repo.FindSessionByRefreshToken(ctx, refreshToken)
	if err != nil {
		return err
	}

	if sess.UserID != userID {
		s.logSystemError(ctx, "session.unauthorized_logout",
			errors.New("unauthorized session access attempt"),
			userID.String(), sess.ID.String())
		return errors.New("unauthorized session access")
	}

	if err := s.repo.DeleteSession(ctx, sess.ID); err != nil {
		return err
	}

	sessIDStr := sess.ID.String()
	userIDStr := sess.UserID.String()

	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		UserID:     &userIDStr,
		Action:     auditctx.ActionUserLoggedOut,
		Module:     auditctx.ModuleAuth,
		EntityType: lo.ToPtr(auditctx.EntitySession),
		EntityID:   &sessIDStr,
	})

	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		UserID:     &userIDStr,
		Action:     auditctx.ActionSessionRevoked,
		Module:     auditctx.ModuleAuth,
		EntityType: lo.ToPtr(auditctx.EntitySession),
		EntityID:   &sessIDStr,
		BeforeState: map[string]interface{}{
			"session_id": sessIDStr,
			"user_id":    userIDStr,
			"revoked_at": time.Now(),
		},
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
			user, err = s.repo.CreateUser(ctx, &User{
				Email:     googleUser.Email,
				FirstName: googleUser.FirstName,
				LastName:  googleUser.LastName,
				Role:      util.RolePractitioner,
			}, tx)
			if err != nil {
				return err
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
			if _, err = s.practitionerSvc.CreatePractitioner(ctx, &practitioner.RqCreatePractitioner{UserID: user.ID.String()}, tx); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("google oauth transaction: %w", err)
	}

	userIDStr := user.ID.String()
	action := auditctx.ActionUserLoggedIn
	if isNewUser {
		action = auditctx.ActionUserRegistered
	}
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		UserID:     &userIDStr,
		Action:     action,
		Module:     auditctx.ModuleAuth,
		EntityType: lo.ToPtr(auditctx.EntityUser),
		EntityID:   &userIDStr,
		AfterState: map[string]interface{}{"email": user.Email, "provider": "google"},
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

func (s *service) VerifyEmail(ctx context.Context, tokenStr string) (string, error) {
	tokenID, err := uuid.Parse(tokenStr)
	if err != nil {
		return "", errors.New("invalid token format")
	}

	token, err := s.repo.GetToken(ctx, tokenID)
	if err != nil {
		return "", errors.New("verification link not found")
	}

	if token.Status != TokenStatusPending {
		return "", fmt.Errorf("this link has already been %s", strings.ToLower(token.Status))
	}

	if time.Now().After(token.ExpiresAt) {
		return "", errors.New("verification link has expired")
	}
	var redirectURL string
	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		user_id, err := s.repo.MarkUserVerified(ctx, token, tx)
		if err != nil {
			return err
		}
		id, _ := uuid.Parse(user_id)

		if err := s.PreferenceSvc.PreferenceSetting(ctx, tx, id, token.EntityID, *token.Role); err != nil {
			return fmt.Errorf("failed to set notification preferences: %w", err)
		}

		return nil
	})

	if err != nil {
		return "", errors.New("failed to verified" + err.Error())
	}
	userIDStr := token.EntityID.String()
	tokenIDStr := token.ID.String()
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		UserID:      &userIDStr,
		Action:      auditctx.ActionEmailVerified,
		Module:      auditctx.ModuleAuth,
		EntityType:  lo.ToPtr(auditctx.EntityVerificationToken),
		EntityID:    &tokenIDStr,
		BeforeState: map[string]interface{}{"status": token.Status},
		AfterState:  map[string]interface{}{"status": "USED"},
	})
	return redirectURL, nil
}

func (s *service) ChangePassword(ctx context.Context, pracID uuid.UUID, req *RqChangePassword) error {
	user, err := s.repo.FindByPractitionerID(ctx, pracID)
	if err != nil {
		return fmt.Errorf("user not found for practitioner: %w", err)
	}

	if user.Password == nil || *user.Password == "" {
		return ErrOAuthOnly
	}

	newHashedPassword, err := util.GenerateHash(req.NewPassword)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}

	if err := s.repo.UpdatePassword(ctx, user.ID, newHashedPassword); err != nil {
		s.logSystemError(ctx, "auth.update_password", err, user.ID.String(), user.ID.String())
		return err
	}

	userIDStr := user.ID.String()
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		UserID:     &userIDStr,
		Action:     auditctx.ActionPasswordChanged,
		Module:     auditctx.ModuleAuth,
		EntityType: lo.ToPtr(auditctx.EntityUser),
		EntityID:   &userIDStr,
	})

	return nil
}

func (s *service) GetProfile(ctx context.Context, userID uuid.UUID) (*RsUser, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		doc, err := s.repo.GetDocumentByUserID(ctx, tx, userID)
		if err != nil {
			return err
		}
		user.Document = doc
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load user profile image: %w", err)
	}

	rs := user.ToRsUser()

	switch user.Role {
	case util.RolePractitioner:
		p, err := s.practitionerSvc.GetPractitionerByUserID(ctx, userID.String())
		if err == nil {
			rs.EntityType = p.EntityType
			rs.EntityName = p.EntityName
			rs.ABN = p.ABN
			rs.ACN = p.ACN
			rs.Address = p.Address
			rs.Profession = p.Profession
		}
	case util.RoleAccountant:
		acc, err := s.accountantSvc.GetAccountantByUserID(ctx, userID.String())
		if err == nil {
			rs.TaxAgentNumber = acc.TaxAgentNumber
			rs.EntityType = acc.EntityType
			rs.EntityName = acc.EntityName
			rs.ABN = acc.ABN
			rs.ACN = acc.ACN
			rs.Address = acc.Address
			rs.Profession = acc.Profession
		}
	}

	return rs, nil
}

func (s *service) UpdateProfile(ctx context.Context, userID uuid.UUID, req *RqUpdateUser) (*RsUser, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	beforeState := sanitizeUser(user)

	if req.Email != nil && *req.Email != user.Email {
		if _, err := s.repo.FindByEmail(ctx, *req.Email); err == nil {
			return nil, ErrEmailTaken
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
		user.Phone = req.Phone
	}

	if req.DocumentId != nil {
		if *req.DocumentId == "" {
			user.Document = nil
		} else if docID, err := uuid.Parse(*req.DocumentId); err == nil {
			user.Document, _ = s.fileRepo.FindByID(ctx, docID)
		}
	}

	updated, err := s.repo.UpdateUser(ctx, user, nil)
	if err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}

	// Update Role-Specific Entity Details
	switch user.Role {
	case util.RolePractitioner:
		// We pass the relevant fields from the request to the practitioner service
		err := s.practitionerSvc.UpdatePractitionerProfile(ctx, userID, &practitioner.RqUpdatePractitioner{
			ABN:        req.ABN,
			EntityType: req.EntityType,
			EntityName: req.EntityName,
			ACN:        req.ACN,
			Address:    req.Address,
			Profession: req.Profession,
		})
		if err != nil {
			return nil, fmt.Errorf("update practitioner profile: %w", err)
		}

	case util.RoleAccountant:
		err := s.accountantSvc.UpdateAccountantProfile(ctx, userID, &accountant.RqUpdateAccountant{
			ABN:            req.ABN,
			EntityType:     req.EntityType,
			EntityName:     req.EntityName,
			ACN:            req.ACN,
			Address:        req.Address,
			Profession:     req.Profession,
			TaxAgentNumber: req.TaxAgentNumber,
		})
		if err != nil {
			return nil, fmt.Errorf("update accountant profile: %w", err)
		}
	}

	userIDStr := updated.ID.String()
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		UserID:      &userIDStr,
		Action:      auditctx.ActionUserUpdated,
		Module:      auditctx.ModuleAuth,
		EntityType:  lo.ToPtr(auditctx.EntityUser),
		EntityID:    &userIDStr,
		BeforeState: beforeState,
		AfterState:  sanitizeUser(updated),
	})

	return s.GetProfile(ctx, userID)
}

func (s *service) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	beforeState := sanitizeUser(user)

	if err := s.repo.DeleteUser(ctx, userID, nil); err != nil {
		return fmt.Errorf("delete user service: %w", err)
	}

	if err := s.practitionerRepo.DeleteByUserID(ctx, userID); err != nil {
		fmt.Printf("INFO: No practitioner profile deleted for user %s: %v\n", userID, err)
	}

	userIDStr := userID.String()
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		UserID:      &userIDStr,
		Action:      auditctx.ActionUserDeleted,
		Module:      auditctx.ModuleAuth,
		EntityType:  lo.ToPtr(auditctx.EntityUser),
		EntityID:    &userIDStr,
		BeforeState: beforeState,
	})

	return nil
}

func (s *service) ForgotPassword(ctx context.Context, req *RqForgotPassword) error {
	user, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil
	}

	if user.Role == util.RoleAccountant {
		inv, err := s.inviteRepo.GetByEmail(ctx, req.Email)
		if err != nil || inv == nil {
			return errors.New("no active account found")
		}
		if inv.Status != "COMPLETED" {
			return errors.New("Please complete your account setup via the invitation link first")
		}
	}

	rawToken := uuid.New().String()
	tokenHash := util.HashToken(rawToken)

	expiresAt := time.Now().Add(15 * time.Minute)
	if err := s.repo.SaveResetToken(ctx, user.ID.String(), tokenHash, expiresAt); err != nil {
		s.logSystemError(ctx, "auth.save_reset_token", err, user.ID.String(), user.ID.String())
		return err
	}

	baseUrl, err := s.cfg.GetBaseURL()
	if err != nil {
		return err
	}
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", baseUrl, rawToken)
	return s.mailer.SendPasswordResetEmail(user.Email, user.FirstName, resetLink)
}

func (s *service) ResetPassword(ctx context.Context, req *RqResetPassword) error {
	tokenHash := util.HashToken(req.Token)

	newPasswordHash, err := util.GenerateHash(req.NewPassword)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	return s.repo.CompletePasswordReset(ctx, tokenHash, newPasswordHash)
}

// isUserVerified checks the role-specific table to see if the user has verified their email.
func (s *service) isUserVerified(ctx context.Context, user *User) (bool, error) {
	var verified bool
	var query string

	switch user.Role {
	case util.RolePractitioner:
		query = "SELECT verified FROM tbl_practitioner WHERE user_id = $1"
	case util.RoleAccountant:
		query = "SELECT verified FROM tbl_accountant WHERE user_id = $1"
	default:
		return false, fmt.Errorf("verification check failed: '%s %s'", user.ID, user.Role)
	}

	if err := s.db.GetContext(ctx, &verified, query, user.ID); err != nil {
		return false, fmt.Errorf("could not verify account status: %w", err)
	}
	return verified, nil
}

// issueTokens creates access/refresh tokens and persists a new session.
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
		s.logSystemError(ctx, "auth.create_session", err, user.ID.String(), user.ID.String())
		return nil, err
	}

	sessIDStr := sess.ID.String()
	userIDStr := user.ID.String()
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		PracticeID: &entityID,
		UserID:     &userIDStr,
		Action:     auditctx.ActionSessionCreated,
		Module:     auditctx.ModuleAuth,
		EntityType: lo.ToPtr(auditctx.EntitySession),
		EntityID:   &sessIDStr,
		AfterState: map[string]interface{}{
			"session_id": sessIDStr,
			"expires_at": sess.ExpiresAt,
		},
	})

	return &RsToken{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Role:         &user.Role,
	}, nil
}

// resolveEntityID returns the role-specific entity ID (practitioner/accountant/admin) for JWT claims.
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
	case util.RoleAdmin:
		a, err := s.adminSvc.GetAdminByUserID(ctx, user.ID)
		if err != nil {
			return "", err
		}
		return a.ID.String(), nil
	default:
		return user.ID.String(), nil
	}
}

// logSystemError is a convenience wrapper for audit system error logging.
func (s *service) logSystemError(ctx context.Context, op string, err error, userID, entityID string) {
	s.auditSvc.LogSystemIssue(ctx, auditctx.ActionSystemError, op,
		err, userID, entityID, auditctx.EntityUser, auditctx.ModuleAuth,
	)
}

// sanitizeUser returns a safe map of user fields for audit logging (no password).
func sanitizeUser(u *User) map[string]interface{} {
	return map[string]interface{}{
		"id":         u.ID.String(),
		"email":      u.Email,
		"first_name": u.FirstName,
		"last_name":  u.LastName,
		"phone":      u.Phone,
	}
}

// GetUserByID implements [Service].
func (s *service) GetUserByID(ctx context.Context, entityID uuid.UUID, EntityType util.ActorType) (*User, error) {
	switch EntityType {
	case util.ActorPractitioner:
		return s.repo.FindByPractitionerID(ctx, entityID)
	case util.ActorAccountant:
		return s.repo.FindByAccountantID(ctx, entityID)
	case util.ActorAdmin:
		return s.repo.FindByAdminID(ctx, entityID)
	default:
		return nil, fmt.Errorf("unknown entity type: %s", EntityType)
	}
}
