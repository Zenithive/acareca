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
	"github.com/iamarpitzala/acareca/internal/shared/common"
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
	Register(ctx context.Context, req *RqRegister) (*RsClinic, error)
	Login(ctx context.Context, req *RqLogin) (*RsToken, error)
	Logout(ctx context.Context, clinicID uuid.UUID, refreshToken string) error

	GetProfile(ctx context.Context, clinicID uuid.UUID) (*RsClinic, error)
	VerifyEmail(ctx context.Context, tokenStr string) error
	ChangePassword(ctx context.Context, clinicID uuid.UUID, req *RqChangePassword) error
	UpdateProfile(ctx context.Context, req *RqUpdate) (*RsClinic, error)

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

func NewService(repo Repository, cfg *config.Config, db *sqlx.DB, auditSvc audit.Service, tmp template.IService) Service {
	return &service{
		repo:     repo,
		cfg:      cfg,
		db:       db,
		auditSvc: auditSvc,
		mailer:   mail.NewClient(cfg.ResendAPIKey, cfg.SenderEmail),
		template: tmp,
	}
}

func (s *service) Register(ctx context.Context, req *RqRegister) (*RsClinic, error) {
	existing, err := s.repo.FindByEmail(ctx, req.Email)
	if err == nil && existing != nil {
		return nil, ErrEmailTaken
	}

	if req.Password == nil || *req.Password == "" {
		return nil, errors.New("password is required")
	}
	hashedPassword, err := util.GenerateHash(*req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to process credentials: %w", err)
	}

	clinic := req.MapToDB(hashedPassword)

	var clinicDB *Clinic
	var tokenID uuid.UUID
	var addresses []Address
	var contacts []Contact
	var document *common.Document

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		var txErr error
		clinicDB, txErr = s.repo.CreateClinic(ctx, &clinic, tx)
		if txErr != nil {
			return txErr
		}

		for _, addr := range req.Addresses {
			AddrDB := addr.MapToDB(clinicDB.ID)
			_, txErr := s.repo.CreateAddress(ctx, &AddrDB, tx)
			if txErr != nil {
				return fmt.Errorf("failed to write clinic location breakdown line: %w", txErr)
			}

			addresses = append(addresses, AddrDB)
		}

		for _, cont := range req.Contacts {
			contactDB := cont.MapToDB(clinicDB.ID)
			_, txErr := s.repo.CreateContact(ctx, &contactDB, tx)
			if txErr != nil {
				return fmt.Errorf("failed to write clinic identity routing point: %w", txErr)
			}

			contacts = append(contacts, contactDB)
		}

		tokenID = uuid.New()
		vToken := &VerificationToken{
			ID:        tokenID,
			ClinicID:  clinicDB.ID,
			Role:      clinicDB.Role,
			Status:    TokenStatusPending,
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		if err := s.repo.CreateVerificationToken(ctx, vToken, tx); err != nil {
			return fmt.Errorf("create verification token: %w", err)
		}

		if clinicDB.DocumentID != nil {
			document, err = s.repo.GetDocumentByID(ctx, clinicDB.DocumentID)
			if err != nil {
				fmt.Printf("[WARN] Failed to fetch document %s: %v\n", *clinicDB.DocumentID, err)
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	baseUrl, err := s.cfg.GetBaseURL()
	if err == nil {
		verificationLink := fmt.Sprintf("%s/clinic/verify-email?token=%s", baseUrl, tokenID)
		go func() {
			if err := s.mailer.SendVerificationEmail(clinicDB.Email, clinicDB.ClinicName, verificationLink); err != nil {
				fmt.Printf("[CLINIC ERROR] Failed to send verification email: %v\n", err)
				s.auditSvc.LogSystemIssue(context.Background(), auditctx.ActionSystemError, "clinic.send_verification_email",
					err, clinicDB.ID.String(), clinicDB.ID.String(), auditctx.EntityUser, auditctx.ModuleAuth,
				)
			}
		}()
	}

	clinicIDStr := clinicDB.ID.String()
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		PracticeID: nil,
		UserID:     &clinicIDStr,
		Action:     auditctx.ActionClinicRegistered,
		Module:     auditctx.ModuleInvoice,
		EntityType: lo.ToPtr(auditctx.EntityInvoiceClinic),
		EntityID:   &clinicIDStr,
	})

	var rsDocument *common.RsDocument
	if document != nil {
		rsDocument = document.ToRsDocument()
	}

	rsClinic := clinicDB.MapToRs(addresses, contacts, rsDocument)

	return &rsClinic, nil
}

func (s *service) Login(ctx context.Context, req *RqLogin) (*RsToken, error) {
	clinic, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		_ = util.CompareHash(req.Password, s.cfg.Hash)
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

	clinicIDStr := clinic.ID.String()
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		PracticeID: &clinicIDStr,
		UserID:     &clinicIDStr,
		Action:     auditctx.ActionClinicLoggedIn,
		Module:     auditctx.ModuleInvoice,
		EntityType: lo.ToPtr(auditctx.EntitySession),
	})

	return s.issueTokens(ctx, clinic, clinicIDStr)
}

func (s *service) Logout(ctx context.Context, clinicID uuid.UUID, refreshToken string) error {
	sess, err := s.repo.FindSessionByRefreshToken(ctx, refreshToken)
	if err != nil {
		return err
	}

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

	sessIDStr := sess.ID.String()
	clinicIDStr := sess.ClinicID.String()
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		UserID:     &clinicIDStr,
		Action:     auditctx.ActionClinicLoggedOut,
		Module:     auditctx.ModuleInvoice,
		EntityType: lo.ToPtr(auditctx.EntityClinicSession),
		EntityID:   &sessIDStr,
	})

	// Audit log:  Session revoked
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
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
	})

	return nil
}

func (s *service) GetProfile(ctx context.Context, clinicID uuid.UUID) (*RsClinic, error) {
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

	var document *common.Document
	if clinic.DocumentID != nil {
		document, err = s.repo.GetDocumentByID(ctx, clinic.DocumentID)
		if err != nil {
			fmt.Printf("[WARN] Failed to fetch document %s: %v\n", *clinic.DocumentID, err)
		}
	}

	var rsDocument *common.RsDocument
	if document != nil {
		rsDocument = document.ToRsDocument()
	}

	rsClinic := clinic.MapToRs(addresses, contacts, rsDocument)

	return &rsClinic, nil
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
	userIDStr := token.ClinicID.String()
	tokenIDStr := token.ID.String()

	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
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
	})
	return nil
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

	refreshToken, err := util.SignToken(clinic.ID.String(), clinicID, roleString, 7*24*time.Hour, s.cfg.JWTSecret)
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
	sessIDStr := sess.ID.String()
	clinicIDStr := clinic.ID.String()

	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
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

	clinicIDStr := clinicID.String()
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		UserID:     &clinicIDStr,
		Action:     auditctx.ActionPasswordChanged,
		Module:     auditctx.ModuleInvoice,
		EntityType: lo.ToPtr(auditctx.EntityInvoiceClinic),
		EntityID:   &clinicIDStr,
	})

	return nil
}

func (s *service) UpdateProfile(ctx context.Context, req *RqUpdate) (*RsClinic, error) {
	clinic, err := s.repo.FindByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		var txErr error

		_, txErr = s.repo.UpdateClinic(ctx, clinic, tx)
		if txErr != nil {
			return fmt.Errorf("update clinic profile: %w", txErr)
		}

		if req.Addresses != nil {
			cs := req.Addresses
			for _, addrID := range cs.Delete {
				if txErr = s.repo.DeleteAddressByID(ctx, addrID, tx); txErr != nil {
					return fmt.Errorf("delete address %s: %w", addrID, txErr)
				}
			}

			remaining, txErr := s.repo.CountActiveAddresses(ctx, clinic.ID, tx)
			if txErr != nil {
				return txErr
			}

			if remaining == 0 && len(cs.Create) == 0 {
				return errors.New("clinic must have at least one address")
			}

			if remaining+len(cs.Create) > 5 {
				return fmt.Errorf("clinic cannot have more than 5 addresses (currently %d active, adding %d)", remaining, len(cs.Create))
			}

			for _, u := range cs.Update {
				addr, err := s.repo.GetAddressByID(ctx, u.ID, tx)
				if err != nil {
					return err
				}

				ad := u.MapToDB(*addr)
				if _, txErr = s.repo.UpdateAddress(ctx, &ad, tx); txErr != nil {
					return fmt.Errorf("update address %s: %w", u.ID, txErr)
				}
			}

			for _, a := range cs.Create {
				add := a.MapToDB(clinic.ID)
				if _, txErr = s.repo.CreateAddress(ctx, &add, tx); txErr != nil {
					return fmt.Errorf("create address: %w", txErr)
				}
			}
		}

		if req.Contacts != nil {
			cs := req.Contacts

			for _, id := range cs.Delete {
				if txErr = s.repo.DeleteContactByID(ctx, id, tx); txErr != nil {
					return fmt.Errorf("delete contact %s: %w", id, txErr)
				}
			}

			remaining, txErr := s.repo.CountActiveContacts(ctx, clinic.ID, tx)
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
				contacts, err := s.repo.GetContactByID(ctx, u.ID, tx)
				if err != nil {
					return err
				}

				ctl := u.MapToDB(*contacts)
				if _, txErr = s.repo.UpdateContact(ctx, &ctl, tx); txErr != nil {
					return fmt.Errorf("update contact %s: %w", u.ID, txErr)
				}
			}

			for _, ct := range cs.Create {
				ctl := ct.MapToDB(clinic.ID)
				if _, txErr = s.repo.CreateContact(ctx, &ctl, tx); txErr != nil {
					return fmt.Errorf("create contact: %w", txErr)
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	updatedClinic, err := s.repo.FindByID(ctx, clinic.ID)
	if err != nil {
		return nil, err
	}
	addresses, err := s.repo.ListAddressesByClinicID(ctx, clinic.ID)
	if err != nil {
		return nil, err
	}
	contacts, err := s.repo.ListContactsByClinicID(ctx, clinic.ID)
	if err != nil {
		return nil, err
	}

	var document *common.Document
	if updatedClinic.DocumentID != nil {
		document, err = s.repo.GetDocumentByID(ctx, updatedClinic.DocumentID)
		if err != nil {
			fmt.Printf("[WARN] Failed to fetch document %s: %v\n", *updatedClinic.DocumentID, err)
		}
	}

	clinicIDStr := clinic.ID.String()
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		PracticeID: &clinicIDStr,
		UserID:     &clinicIDStr,
		Action:     auditctx.ActionClinicUpdated,
		Module:     auditctx.ModuleInvoice,
		EntityType: lo.ToPtr(auditctx.EntityInvoiceClinic),
		EntityID:   &clinicIDStr,
	})

	var rsDocument *common.RsDocument
	if document != nil {
		rsDocument = document.ToRsDocument()
	}

	rsClinic := clinic.MapToRs(addresses, contacts, rsDocument)

	return &rsClinic, nil
}

func (s *service) DeleteClinic(ctx context.Context, clinicID uuid.UUID) error {
	if err := s.repo.DeleteClinic(ctx, clinicID); err != nil {
		return fmt.Errorf("delete clinic: %w", err)
	}

	clinicIDStr := clinicID.String()
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		PracticeID: &clinicIDStr,
		UserID:     &clinicIDStr,
		Action:     auditctx.ActionClinicDeleted,
		Module:     auditctx.ModuleInvoice,
		EntityType: lo.ToPtr(auditctx.EntityInvoiceClinic),
		EntityID:   &clinicIDStr,
	})

	return nil
}

func (s *service) ForgotPassword(ctx context.Context, req *RqForgotPassword) error {
	clinic, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		return err
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

	resetLink := fmt.Sprintf("%s/clinic-portal/reset-password?token=%s", baseUrl, rawToken)
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
