package file

import (
	"context"
	"errors"
	"fmt"
	"log"
	"mime/multipart"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/business/admin"
	"github.com/iamarpitzala/acareca/internal/modules/business/invitation"
	"github.com/iamarpitzala/acareca/internal/modules/notification"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
	"github.com/iamarpitzala/acareca/internal/shared/upload"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
)

// UserInfo represents user information needed for notifications
type UserInfo struct {
	FirstName string
	LastName  string
}

// AuthService defines the interface for auth operations needed by file service
type AuthService interface {
	GetUserByID(ctx context.Context, entityID uuid.UUID, EntityType util.ActorType) (*UserInfo, error)
}

type Service interface {
	GetDocument(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*RsDocument, error)
	ListDocuments(ctx context.Context, userID uuid.UUID, filters *RqListDocuments) (*util.RsList, error)
	UpdateDocument(ctx context.Context, id uuid.UUID, req *RqUpdateDocument, userID uuid.UUID) (*RsDocument, error)
	DeleteDocument(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	GeneratePresignedUploadURL(ctx context.Context, req *RqGeneratePresignedUploadURL, ownerID uuid.UUID, ownerRole string, file multipart.File, header *multipart.FileHeader) (*RsPresignedUploadURL, error)
}

type service struct {
	repo            Repository
	storage         upload.StorageProvider
	validator       *upload.FileValidator
	cfg             *config.Config
	db              *sqlx.DB
	auditSvc        audit.Service
	bucket          string
	notificationPub *sharednotification.Publisher
	invitationRepo  invitation.Repository
	authSvc         AuthService
}

func NewService(repo Repository, storage upload.StorageProvider, validator *upload.FileValidator, cfg *config.Config, db *sqlx.DB, auditSvc audit.Service, invitationRepo invitation.Repository, authSvc AuthService, notificationSvc notification.Service, adminRepo admin.Repository) Service {
	return &service{
		repo:            repo,
		storage:         storage,
		validator:       validator,
		cfg:             cfg,
		db:              db,
		auditSvc:        auditSvc,
		bucket:          cfg.R2BucketName,
		notificationPub: sharednotification.NewPublisher(notification.NewServiceAdapter(notificationSvc), adminRepo),
		invitationRepo:  invitationRepo,
		authSvc:         authSvc,
	}
}

// GetDocument retrieves document metadata
func (s *service) GetDocument(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*RsDocument, error) {
	doc, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if !s.canAccessDocument(doc, userID) {
		return nil, ErrUnauthorizedAccess
	}

	return doc.ToRsDocument(), nil
}

// ListDocuments lists documents for a user
func (s *service) ListDocuments(ctx context.Context, userID uuid.UUID, filters *RqListDocuments) (*util.RsList, error) {

	docs, total, err := s.repo.FindByOwner(ctx, userID, filters)
	if err != nil {
		return nil, err
	}

	rsDocs := make([]RsDocument, len(docs))
	for i, doc := range docs {
		rsDocs[i] = *doc.ToRsDocument()
	}

	result := &util.RsList{}
	result.MapToList(rsDocs, int(total), *filters.filter.Offset, *filters.filter.Limit)
	return result, nil
}

// UpdateDocument updates document metadata
func (s *service) UpdateDocument(ctx context.Context, id uuid.UUID, req *RqUpdateDocument, userID uuid.UUID) (*RsDocument, error) {
	doc, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if doc.OwnerID != userID {
		return nil, ErrUnauthorizedAccess
	}

	// Snapshot the state before mutation for the audit log.
	beforeName := doc.OriginalName
	beforeIsPublic := doc.IsPublic

	if req.OriginalName != nil {
		doc.OriginalName = upload.SanitizeFilename(*req.OriginalName)
	}

	if req.IsPublic != nil {
		doc.IsPublic = *req.IsPublic
	}

	var updated *Document
	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		var txErr error
		updated, txErr = s.repo.Update(ctx, doc, tx)
		return txErr
	})
	if err != nil {
		return nil, err
	}

	s.logFileUpdate(ctx, updated, userID, map[string]interface{}{
		"filename":  beforeName,
		"is_public": beforeIsPublic,
	})

	return updated.ToRsDocument(), nil
}

// DeleteDocument deletes a document
func (s *service) DeleteDocument(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	var doc *Document
	err := util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		var txErr error
		doc, txErr = s.repo.FindByID(ctx, id)
		if txErr != nil {
			return txErr
		}

		if doc.OwnerID != userID {
			return ErrUnauthorizedAccess
		}

		return s.repo.Delete(ctx, id, tx)
	})
	if err != nil {
		return err
	}

	go func() {
		if err := s.storage.Delete(context.Background(), doc.ObjectKey); err != nil {
			fmt.Printf("[file service] failed to delete object %q from storage (document %s): %v — manual cleanup may be required\n",
				doc.ObjectKey, doc.ID, err)
		}
	}()

	s.logFileDelete(ctx, doc, userID)

	// Send notification for document deletion only if accountant
	actorType := util.ActorPractitioner
	if doc.OwnerRole == util.RoleAccountant {
		actorType = util.ActorAccountant
		if err := s.notifyDocument(ctx, doc.ID, userID, actorType, util.EventDocumentUploaded, doc.OriginalName); err != nil {
			log.Printf("[WARN] failed to send document deletion notification: %v", err)
		}
	}

	return nil
}

// GeneratePresignedUploadURL generates a presigned URL for direct client upload to R2.
func (s *service) GeneratePresignedUploadURL(ctx context.Context, req *RqGeneratePresignedUploadURL, ownerID uuid.UUID, ownerRole string, file multipart.File, header *multipart.FileHeader) (*RsPresignedUploadURL, error) {
	presignedProvider, ok := s.storage.(upload.PresignedURLProvider)
	if !ok {
		return nil, errors.New("presigned URLs not supported by current storage provider")
	}

	// Validate file size and detect MIME type from actual file bytes (not client header).
	if err := s.validator.Validate(file, header); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	objectKey := s.storage.GenerateObjectKey(ownerID, header.Filename)

	expiresInSec := 900
	if req.ExpiresIn != nil {
		expiresInSec = *req.ExpiresIn
	}
	expiresIn := time.Duration(expiresInSec) * time.Second

	// Use the detected MIME type (set by Validate) rather than the client-supplied header.
	mimeType := header.Header.Get("Content-Type")

	uploadURL, err := presignedProvider.GeneratePresignedUploadURL(objectKey, mimeType, expiresIn)
	if err != nil {
		return nil, fmt.Errorf("generate presigned upload URL: %w", err)
	}

	ext := upload.GetFileExtension(header.Filename)
	doc := &Document{
		OwnerID:      ownerID,
		OwnerRole:    ownerRole,
		ObjectKey:    objectKey,
		Bucket:       s.bucket,
		OriginalName: upload.SanitizeFilename(header.Filename),
		Extension:    &ext,
		MimeType:     mimeType,
		SizeBytes:    header.Size,
		Status:       StatusUploaded,
		IsPublic:     false,
	}

	updatedAt := time.Now()
	doc.UploadedAt = &updatedAt

	// Calculate presigned URL expiration for response
	expiresAt := time.Now().Add(expiresIn)

	var created *Document
	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		var txErr error
		created, txErr = s.repo.Create(ctx, doc, tx)
		if txErr != nil {
			return fmt.Errorf("save document record: %w", txErr)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.logFileUpload(ctx, created, ownerID)

	// Send notification for document upload only if accountant
	actorType := util.ActorPractitioner
	if ownerRole == util.RoleAccountant {
		actorType = util.ActorAccountant
		if err := s.notifyDocument(ctx, created.ID, ownerID, actorType, util.EventDocumentUploaded, created.OriginalName); err != nil {
			log.Printf("[WARN] failed to send document upload notification: %v", err)
		}
	}

	_, err = s.cfg.GetBaseURL()
	if err != nil {
		return nil, fmt.Errorf("get base URL: %w", err)
	}

	return &RsPresignedUploadURL{
		UploadURL:   uploadURL,
		ObjectKey:   objectKey,
		ExpiresAt:   expiresAt,
		DocumentID:  created.ID,
		ContentType: mimeType,
	}, nil
}

// canAccessDocument checks if user can access a document
func (s *service) canAccessDocument(doc *Document, userID uuid.UUID) bool {
	if doc.OwnerID == userID {
		return true
	}
	if doc.IsPublic {
		return true
	}
	return false
}

// Audit logging helpers

func (s *service) logFileUpload(ctx context.Context, doc *Document, userID uuid.UUID) {
	userIDStr := userID.String()
	docIDStr := doc.ID.String()

	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		UserID:     &userIDStr,
		Action:     auditctx.ActionFileUploaded,
		Module:     auditctx.ModuleFile,
		EntityType: lo.ToPtr(auditctx.EntityFile),
		EntityID:   &docIDStr,
		AfterState: map[string]interface{}{
			"filename":  doc.OriginalName,
			"size":      doc.SizeBytes,
			"mime_type": doc.MimeType,
		},
	})
}

func (s *service) logFileUpdate(ctx context.Context, doc *Document, userID uuid.UUID, beforeState map[string]interface{}) {
	userIDStr := userID.String()
	docIDStr := doc.ID.String()

	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		UserID:      &userIDStr,
		Action:      auditctx.ActionFileUpdated,
		Module:      auditctx.ModuleFile,
		EntityType:  lo.ToPtr(auditctx.EntityFile),
		EntityID:    &docIDStr,
		BeforeState: beforeState,
		AfterState: map[string]interface{}{
			"filename":  doc.OriginalName,
			"is_public": doc.IsPublic,
		},
	})
}

func (s *service) logFileDelete(ctx context.Context, doc *Document, userID uuid.UUID) {
	userIDStr := userID.String()
	docIDStr := doc.ID.String()

	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		UserID:     &userIDStr,
		Action:     auditctx.ActionFileDeleted,
		Module:     auditctx.ModuleFile,
		EntityType: lo.ToPtr(auditctx.EntityFile),
		EntityID:   &docIDStr,
		BeforeState: map[string]interface{}{
			"filename":   doc.OriginalName,
			"object_key": doc.ObjectKey,
		},
	})
}

// notifyDocument sends notifications to linked users about document operations
func (s *service) notifyDocument(ctx context.Context, docID uuid.UUID, actorID uuid.UUID, actorType util.ActorType, eventType util.EventType, filename string) error {
	if s.notificationPub == nil {
		log.Printf("[WARN] notification publisher is nil, skipping document notification")
		return nil
	}

	if s.authSvc == nil {
		log.Printf("[WARN] auth service is nil, skipping document notification")
		return nil
	}

	// Get sender information
	user, err := s.authSvc.GetUserByID(ctx, actorID, actorType)
	if err != nil {
		log.Printf("[WARN] failed to get user for notification: %v", err)
		return nil
	}
	senderName := user.FirstName + " " + user.LastName

	// Build recipients list
	recipients := []sharednotification.RecipientWithPreferences{}

	switch actorType {
	case util.ActorPractitioner:
		// If the sender is a practitioner, notify all their linked accountants
		accountants, err := s.invitationRepo.GetAccountantsLinkedToPractitioner(ctx, actorID)
		if err != nil {
			log.Printf("[WARN] failed to get linked accountants for practitioner %s: %v", actorID, err)
			return nil
		}

		for _, acc := range accountants {
			recipients = append(recipients, sharednotification.RecipientWithPreferences{
				RecipientID:   acc.AccountantID,
				RecipientType: util.ActorAccountant,
				UserID:        acc.UserID,
			})
		}

	case util.ActorAccountant:
		// If the sender is an accountant, notify all linked practitioners
		practitionerIDs, err := s.invitationRepo.GetPractitionersLinkedToAccountant(ctx, actorID)
		if err != nil {
			log.Printf("[WARN] failed to get practitioners for accountant %s: %v", actorID, err)
			return nil
		}

		// Notify each linked practitioner
		for _, practitionerID := range practitionerIDs {
			practitionerUserID, err := s.invitationRepo.GetPractitionerUserIDByID(ctx, practitionerID)
			if err != nil {
				log.Printf("[WARN] failed to get user ID for practitioner %s: %v", practitionerID, err)
				continue
			}

			recipients = append(recipients, sharednotification.RecipientWithPreferences{
				RecipientID:   practitionerID,
				RecipientType: util.ActorPractitioner,
				UserID:        practitionerUserID,
			})
		}

	default:
		log.Printf("[WARN] unsupported actor type for document notification: %s", actorType)
		return nil
	}

	// If no recipients, don't send notification
	if len(recipients) == 0 {
		log.Printf("[INFO] no recipients found for document notification")
		return nil
	}

	// Send notifications with preferences using the publisher
	err = s.notificationPub.Publish(ctx, sharednotification.PublishRequest{
		Recipients: recipients,
		SenderID:   actorID,
		SenderType: actorType,
		SenderName: senderName,
		EventType:  eventType,
		EntityType: util.EntityDocument,
		EntityID:   docID,
		EntityKey:  "document_id",
		Title:      "Document Uploaded",
		Body:       fmt.Sprintf("Document uploaded: %s by %s", filename, senderName),
		ExtraData:  map[string]interface{}{"filename": filename},
	})

	if err != nil {
		log.Printf("[WARN] failed to send document notification: %v", err)
	}

	return nil
}
