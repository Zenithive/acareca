package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/mail"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
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
	ChangePassword(ctx context.Context, clinicID uuid.UUID, req *RqChangePassword) error
	UpdateProfile(ctx context.Context, clinicID uuid.UUID, req *RqUpdateClinic) (*RsClinicDetail, error)
	DeleteClinic(ctx context.Context, clinicID uuid.UUID) error
	ForgotPassword(ctx context.Context, req *RqForgotPassword) error
	ResetPassword(ctx context.Context, req *RqResetPassword) error
}

type service struct {
	repo     Repository
	cfg      *config.Config
	db       *sqlx.DB
	auditSvc audit.Service
	mailer   *mail.Client
	template template.IService
}

func NewService(repo Repository, cfg *config.Config, db *sqlx.DB, auditSvc audit.Service) Service {
	return &service{
		repo:     repo,
		cfg:      cfg,
		db:       db,
		auditSvc: auditSvc,
		mailer:   mail.NewClient(cfg.ResendAPIKey, cfg.SenderEmail),
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
		Verified:    false, // Explicitly set to false until email is verified
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
			ExpiresAt: time.Now().Add(24 * time.Hour), // 24 hours expiry
		}

		if err := s.repo.CreateVerificationToken(ctx, vToken, tx); err != nil {
			return fmt.Errorf("create verification token: %w", err)
		}

		_, err = s.template.BulkCreate(ctx, createdClinic.ID)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	baseUrl, err := s.cfg.GetBaseURL()
	if err == nil {
		verificationLink := fmt.Sprintf("%s/verify-email?token=%s", baseUrl, tokenID)
		go func() {
			if err := s.mailer.SendVerificationEmail(createdClinic.Email, createdClinic.ClinicName, verificationLink); err != nil {
				fmt.Printf("[CLINIC ERROR] Failed to send verification email: %v\n", err)
				s.auditSvc.LogSystemIssue(context.Background(), auditctx.ActionSystemError, "clinic.send_verification_email",
					err, createdClinic.ID.String(), createdClinic.ID.String(), auditctx.EntityUser, auditctx.ModuleAuth,
				)
			}
		}()
	}

	meta := auditctx.GetMetadata(ctx)
	clinicIDStr := createdClinic.ID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: &clinicIDStr,
		UserID:     &clinicIDStr, // Clinic id
		Action:     auditctx.ActionClinicRegistered,
		Module:     auditctx.ModuleInvoice,
		EntityType: lo.ToPtr(auditctx.EntityInvoiceClinic),
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

	// Check if email is verified
	if !clinic.Verified {
		return nil, errors.New("email not verified. Please check your email for the verification link")
	}

	meta := auditctx.GetMetadata(ctx)
	clinicIDStr := clinic.ID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: &clinicIDStr,
		UserID:     &clinicIDStr,
		Action:     auditctx.ActionClinicLoggedIn,
		Module:     auditctx.ModuleInvoice,
		EntityType: lo.ToPtr(auditctx.EntitySession),
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
		EntityType: lo.ToPtr(auditctx.EntityClinicSession),
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
		EntityType: lo.ToPtr(auditctx.EntityClinicSession),
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
		EntityType: lo.ToPtr(auditctx.EntityVerificationToken),
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
		EntityType: lo.ToPtr(auditctx.EntityClinicSession),
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

func (s *service) ChangePassword(ctx context.Context, clinicID uuid.UUID, req *RqChangePassword) error {
	clinic, err := s.repo.FindByID(ctx, clinicID)
	if err != nil {
		return fmt.Errorf("clinic not found: %w", err)
	}

	if clinic.Password == nil || *clinic.Password == "" {
		return errors.New("password change not available for this account")
	}

	newHashedPassword, err := util.GenerateHash(req.NewPassword)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}

	if err := s.repo.UpdatePassword(ctx, clinicID, newHashedPassword); err != nil {
		s.auditSvc.LogSystemIssue(ctx, auditctx.ActionSystemError, "clinic.update_password",
			err, clinicID.String(), clinicID.String(), auditctx.EntityInvoiceClinic, auditctx.ModuleInvoice,
		)
		return err
	}

	meta := auditctx.GetMetadata(ctx)
	clinicIDStr := clinicID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		UserID:     &clinicIDStr,
		Action:     auditctx.ActionPasswordChanged,
		Module:     auditctx.ModuleInvoice,
		EntityType: lo.ToPtr(auditctx.EntityInvoiceClinic),
		EntityID:   &clinicIDStr,
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	return nil
}

func (s *service) UpdateProfile(ctx context.Context, clinicID uuid.UUID, req *RqUpdateClinic) (*RsClinicDetail, error) {
	clinic, err := s.repo.FindByID(ctx, clinicID)
	if err != nil {
		return nil, err
	}

	if req.ClinicName != nil {
		clinic.ClinicName = *req.ClinicName
	}
	if req.Description != nil {
		clinic.Description = req.Description
	}
	if req.DocumentID != nil {
		clinic.DocumentID = req.DocumentID
	}
	if req.ABN != nil {
		clinic.ABN = req.ABN
	}
	if req.ACN != nil {
		clinic.ACN = req.ACN
	}

	// Pre-validate address changeset before opening a transaction
	if req.Addresses != nil {
		cs := req.Addresses
		// Enforce: at most 5 active addresses total after changes
		// Enforce: exactly 1 primary across all surviving + created addresses
		// Enforce: cannot delete all addresses
		if err := validateAddressChangeset(cs); err != nil {
			return nil, err
		}
	}

	// Pre-validate contact changeset
	if req.Contacts != nil {
		cs := req.Contacts
		if err := validateContactChangeset(cs); err != nil {
			return nil, err
		}
	}

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		var txErr error

		_, txErr = s.repo.UpdateClinic(ctx, clinic, tx)
		if txErr != nil {
			return fmt.Errorf("update clinic profile: %w", txErr)
		}

		if req.Addresses != nil {
			cs := req.Addresses

			// Delete first so counts are accurate
			for _, idStr := range cs.Delete {
				addrID, parseErr := uuid.Parse(idStr)
				if parseErr != nil {
					return fmt.Errorf("invalid address id %q: %w", idStr, parseErr)
				}
				if txErr = s.repo.DeleteAddressByID(ctx, addrID, tx); txErr != nil {
					return fmt.Errorf("delete address %s: %w", idStr, txErr)
				}
			}

			// Count remaining active addresses
			remaining, txErr := s.repo.CountActiveAddresses(ctx, clinicID, tx)
			if txErr != nil {
				return txErr
			}

			// Guard: must always have at least 1 address
			if remaining == 0 && len(cs.Create) == 0 {
				return errors.New("clinic must have at least one address")
			}

			// Guard: max 5 addresses
			if remaining+len(cs.Create) > 5 {
				return fmt.Errorf("clinic cannot have more than 5 addresses (currently %d active, adding %d)", remaining, len(cs.Create))
			}

			for _, u := range cs.Update {
				addrID, parseErr := uuid.Parse(u.ID)
				if parseErr != nil {
					return fmt.Errorf("invalid address id %q: %w", u.ID, parseErr)
				}
				if _, txErr = s.repo.UpdateAddress(ctx, &ClinicAddress{
					ID:        addrID,
					Address:   u.Address,
					City:      u.City,
					State:     u.State,
					Postcode:  u.Postcode,
					IsPrimary: u.IsPrimary,
				}, tx); txErr != nil {
					return fmt.Errorf("update address %s: %w", u.ID, txErr)
				}
			}

			for _, a := range cs.Create {
				if _, txErr = s.repo.CreateAddress(ctx, &ClinicAddress{
					ClinicID:  clinicID,
					Address:   a.Address,
					City:      a.City,
					State:     a.State,
					Postcode:  a.Postcode,
					IsPrimary: a.IsPrimary,
				}, tx); txErr != nil {
					return fmt.Errorf("create address: %w", txErr)
				}
			}
		}

		if req.Contacts != nil {
			cs := req.Contacts

			for _, idStr := range cs.Delete {
				contID, parseErr := uuid.Parse(idStr)
				if parseErr != nil {
					return fmt.Errorf("invalid contact id %q: %w", idStr, parseErr)
				}
				if txErr = s.repo.DeleteContactByID(ctx, contID, tx); txErr != nil {
					return fmt.Errorf("delete contact %s: %w", idStr, txErr)
				}
			}

			remaining, txErr := s.repo.CountActiveContacts(ctx, clinicID, tx)
			if txErr != nil {
				return txErr
			}

			if remaining == 0 && len(cs.Create) == 0 {
				return errors.New("clinic must have at least one contact")
			}

			if remaining+len(cs.Create) > 2 {
				return fmt.Errorf("clinic cannot have more than 2 contacts (currently %d active, adding %d)", remaining, len(cs.Create))
			}

			for _, u := range cs.Update {
				contID, parseErr := uuid.Parse(u.ID)
				if parseErr != nil {
					return fmt.Errorf("invalid contact id %q: %w", u.ID, parseErr)
				}
				if _, txErr = s.repo.UpdateContact(ctx, &ClinicContact{
					ID:          contID,
					ContactType: u.ContactType,
					Value:       u.Value,
					Label:       u.Label,
					IsPrimary:   u.IsPrimary,
				}, tx); txErr != nil {
					return fmt.Errorf("update contact %s: %w", u.ID, txErr)
				}
			}

			for _, c := range cs.Create {
				if _, txErr = s.repo.CreateContact(ctx, &ClinicContact{
					ClinicID:    clinicID,
					ContactType: c.ContactType,
					Value:       c.Value,
					Label:       c.Label,
					IsPrimary:   c.IsPrimary,
				}, tx); txErr != nil {
					return fmt.Errorf("create contact: %w", txErr)
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Re-fetch the final state
	updatedClinic, err := s.repo.FindByID(ctx, clinicID)
	if err != nil {
		return nil, err
	}
	addresses, _ := s.repo.ListAddressesByClinicID(ctx, clinicID)
	contacts, _ := s.repo.ListContactsByClinicID(ctx, clinicID)

	meta := auditctx.GetMetadata(ctx)
	clinicIDStr := clinicID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: &clinicIDStr,
		UserID:     &clinicIDStr,
		Action:     auditctx.ActionClinicUpdated,
		Module:     auditctx.ModuleInvoice,
		EntityType: lo.ToPtr(auditctx.EntityInvoiceClinic),
		EntityID:   &clinicIDStr,
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	return toRsClinicDetail(updatedClinic, addresses, contacts), nil
}

// validateAddressChangeset checks primary-field rules before the transaction.
func validateAddressChangeset(cs *RqAddressChangeset) error {
	primaryCount := 0
	for _, a := range cs.Create {
		if a.IsPrimary {
			primaryCount++
		}
	}
	for _, a := range cs.Update {
		if a.IsPrimary {
			primaryCount++
		}
	}
	// Only enforce primary uniqueness when the caller is explicitly setting primaries
	if primaryCount > 1 {
		return errors.New("only one address can be marked as primary")
	}
	return nil
}

// validateContactChangeset checks primary-field rules before the transaction.
func validateContactChangeset(cs *RqContactChangeset) error {
	primaryCount := 0
	for _, c := range cs.Create {
		if c.IsPrimary {
			primaryCount++
		}
	}
	for _, c := range cs.Update {
		if c.IsPrimary {
			primaryCount++
		}
	}
	if primaryCount > 1 {
		return errors.New("only one contact can be marked as primary")
	}
	return nil
}

func (s *service) DeleteClinic(ctx context.Context, clinicID uuid.UUID) error {
	if err := s.repo.DeleteClinic(ctx, clinicID); err != nil {
		return fmt.Errorf("delete clinic: %w", err)
	}

	meta := auditctx.GetMetadata(ctx)
	clinicIDStr := clinicID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: &clinicIDStr,
		UserID:     &clinicIDStr,
		Action:     auditctx.ActionClinicDeleted,
		Module:     auditctx.ModuleInvoice,
		EntityType: lo.ToPtr(auditctx.EntityInvoiceClinic),
		EntityID:   &clinicIDStr,
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	return nil
}

func (s *service) ForgotPassword(ctx context.Context, req *RqForgotPassword) error {
	clinic, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		// Return nil to avoid user enumeration
		return nil
	}

	rawToken := uuid.New().String()
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	expiresAt := time.Now().Add(15 * time.Minute)
	if err := s.repo.SaveResetToken(ctx, clinic.ID.String(), tokenHash, expiresAt); err != nil {
		s.auditSvc.LogSystemIssue(ctx, auditctx.ActionSystemError, "clinic.save_reset_token",
			err, clinic.ID.String(), clinic.ID.String(), auditctx.EntityInvoiceClinic, auditctx.ModuleInvoice,
		)
		return err
	}

	baseUrl, err := s.cfg.GetBaseURL()
	if err != nil {
		return err
	}

	resetLink := fmt.Sprintf("%s/clinic/reset-password?token=%s", baseUrl, rawToken)
	return s.mailer.SendPasswordResetEmail(clinic.Email, clinic.ClinicName, resetLink)
}

func (s *service) ResetPassword(ctx context.Context, req *RqResetPassword) error {
	hash := sha256.Sum256([]byte(req.Token))
	tokenHash := hex.EncodeToString(hash[:])

	newPasswordHash, err := util.GenerateHash(req.NewPassword)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	return s.repo.CompletePasswordReset(ctx, tokenHash, newPasswordHash)
}
