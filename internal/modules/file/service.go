package file

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/upload"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
)

type Service interface {
	GetDocument(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*RsDocument, error)
	ListDocuments(ctx context.Context, userID uuid.UUID, filters *RqListDocuments) (*util.RsList, error)
	UpdateDocument(ctx context.Context, id uuid.UUID, req *RqUpdateDocument, userID uuid.UUID) (*RsDocument, error)
	DeleteDocument(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	GeneratePresignedUploadURL(ctx context.Context, req *RqGeneratePresignedUploadURL, ownerID uuid.UUID, ownerRole string, file multipart.File, header *multipart.FileHeader) (*RsPresignedUploadURL, error)
}

type service struct {
	repo      Repository
	storage   upload.StorageProvider
	validator *upload.FileValidator
	cfg       *config.Config
	db        *sqlx.DB
	auditSvc  audit.Service
	bucket    string
}

func NewService(repo Repository, storage upload.StorageProvider, validator *upload.FileValidator, cfg *config.Config, db *sqlx.DB, auditSvc audit.Service) Service {
	return &service{
		repo:      repo,
		storage:   storage,
		validator: validator,
		cfg:       cfg,
		db:        db,
		auditSvc:  auditSvc,
		bucket:    cfg.R2BucketName,
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

	baseURL, _ := s.cfg.GetBaseURL()
	return doc.ToRsDocument(baseURL), nil
}

// ListDocuments lists documents for a user
func (s *service) ListDocuments(ctx context.Context, userID uuid.UUID, filters *RqListDocuments) (*util.RsList, error) {
	docs, total, err := s.repo.FindByOwner(ctx, userID, filters)
	if err != nil {
		return nil, err
	}

	baseURL, _ := s.cfg.GetBaseURL()
	rsDocs := make([]RsDocument, len(docs))
	for i, doc := range docs {
		rsDocs[i] = *doc.ToRsDocument(baseURL)
	}

	page := 1
	if filters.Page > 0 {
		page = filters.Page
	}
	limit := 20
	if filters.Limit > 0 {
		limit = filters.Limit
	}

	result := &util.RsList{}
	result.MapToList(rsDocs, int(total), page, limit)
	return result, nil
}

// UpdateDocument updates document metadata
func (s *service) UpdateDocument(ctx context.Context, id uuid.UUID, req *RqUpdateDocument, userID uuid.UUID) (*RsDocument, error) {
	doc, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Check access permission
	if doc.OwnerID != userID {
		return nil, ErrUnauthorizedAccess
	}

	// Update fields
	if req.OriginalName != nil {
		doc.OriginalName = upload.SanitizeFilename(*req.OriginalName)
	}
	if req.EntityType != nil {
		doc.EntityType = req.EntityType
	}
	if req.EntityID != nil {
		entityUUID, err := uuid.Parse(*req.EntityID)
		if err != nil {
			return nil, fmt.Errorf("invalid entity_id: %w", err)
		}
		doc.EntityID = &entityUUID
	}
	if req.IsPublic != nil {
		doc.IsPublic = *req.IsPublic
	}
	var updated *Document
	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		updated, err = s.repo.Update(ctx, doc, tx)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Audit log
	s.logFileUpdate(ctx, updated, userID)

	baseURL, _ := s.cfg.GetBaseURL()
	return updated.ToRsDocument(baseURL), nil
}

// DeleteDocument deletes a document
func (s *service) DeleteDocument(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {

	var doc *Document
	var err error
	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		doc, err = s.repo.FindByID(ctx, id)
		if err != nil {
			return err
		}

		// Check access permission
		if doc.OwnerID != userID {
			return ErrUnauthorizedAccess
		}

		// Soft delete in database
		if err = s.repo.Delete(ctx, id, tx); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Delete from storage (async to not block response)
	go func() {
		if err := s.storage.Delete(context.Background(), doc.ObjectKey); err != nil {
			// Log error but don't fail the request
			fmt.Printf("Failed to delete file from storage: %v\n", err)
		}
	}()

	// Audit log
	s.logFileDelete(ctx, doc, userID)

	return nil
}

// GeneratePresignedUploadURL generates a presigned URL for direct upload to R2
func (s *service) GeneratePresignedUploadURL(ctx context.Context, req *RqGeneratePresignedUploadURL, ownerID uuid.UUID, ownerRole string, file multipart.File, header *multipart.FileHeader) (*RsPresignedUploadURL, error) {
	presignedProvider, ok := s.storage.(upload.PresignedURLProvider)
	if !ok {
		return nil, errors.New("presigned URLs not supported by current storage provider")
	}

	objectKey := s.storage.GenerateObjectKey(ownerID, header.Filename)

	// Default to 15 minutes if not specified; ExpiresIn is optional (*int)
	expiresInSec := 900
	if req.ExpiresIn != nil {
		expiresInSec = *req.ExpiresIn
	}
	expiresIn := time.Duration(expiresInSec) * time.Second
	mimeType := header.Header.Get("Content-Type")
	uploadURL, err := presignedProvider.GeneratePresignedUploadURL(objectKey, mimeType, expiresIn)
	if err != nil {
		return nil, fmt.Errorf("generate presigned upload URL: %w", err)
	}

	_, checksum, err := s.storage.Upload(ctx, file, header, objectKey)
	if err != nil {
		return nil, fmt.Errorf("upload to storage: %w", err)
	}

	// Create pending document record
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
		Status:       StatusPending,
		IsPublic:     false,
		Checksum:     &checksum,
	}

	// Set entity information if provided
	if req.EntityType != nil {
		doc.EntityType = req.EntityType
	}
	if req.EntityID != nil {
		entityUUID, err := uuid.Parse(*req.EntityID)
		if err != nil {
			return nil, fmt.Errorf("invalid entity_id: %w", err)
		}
		doc.EntityID = &entityUUID
	}

	// Set upload expiration
	expiresAt := time.Now().Add(expiresIn)
	doc.UploadExpiresAt = &expiresAt

	var created *Document
	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		// Save to database
		created, err = s.repo.Create(ctx, doc, tx)
		if err != nil {
			return fmt.Errorf("save document record: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &RsPresignedUploadURL{
		UploadURL:  uploadURL,
		ObjectKey:  objectKey,
		ExpiresAt:  expiresAt,
		DocumentID: created.ID,
	}, nil
}

// canAccessDocument checks if user can access a document
func (s *service) canAccessDocument(doc *Document, userID uuid.UUID) bool {
	// Owner can always access
	if doc.OwnerID == userID {
		return true
	}

	// Public documents can be accessed by anyone
	if doc.IsPublic {
		return true
	}

	// TODO: Add more complex permission logic
	// - Check if user is part of the same entity
	// - Check role-based permissions
	// - Check shared access

	return false
}

// Audit logging helpers

func (s *service) logFileUpload(ctx context.Context, doc *Document, userID uuid.UUID) {
	meta := auditctx.GetMetadata(ctx)
	userIDStr := userID.String()
	docIDStr := doc.ID.String()

	s.auditSvc.LogAsync(&audit.LogEntry{
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
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})
}

func (s *service) logFileDownload(ctx context.Context, doc *Document, userID uuid.UUID) {
	meta := auditctx.GetMetadata(ctx)
	userIDStr := userID.String()
	docIDStr := doc.ID.String()

	s.auditSvc.LogAsync(&audit.LogEntry{
		UserID:     &userIDStr,
		Action:     auditctx.ActionFileDownloaded,
		Module:     auditctx.ModuleFile,
		EntityType: lo.ToPtr(auditctx.EntityFile),
		EntityID:   &docIDStr,
		AfterState: map[string]interface{}{
			"filename": doc.OriginalName,
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})
}

func (s *service) logFileUpdate(ctx context.Context, doc *Document, userID uuid.UUID) {
	meta := auditctx.GetMetadata(ctx)
	userIDStr := userID.String()
	docIDStr := doc.ID.String()

	s.auditSvc.LogAsync(&audit.LogEntry{
		UserID:     &userIDStr,
		Action:     auditctx.ActionFileUpdated,
		Module:     auditctx.ModuleFile,
		EntityType: lo.ToPtr(auditctx.EntityFile),
		EntityID:   &docIDStr,
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})
}

func (s *service) logFileDelete(ctx context.Context, doc *Document, userID uuid.UUID) {
	meta := auditctx.GetMetadata(ctx)
	userIDStr := userID.String()
	docIDStr := doc.ID.String()

	s.auditSvc.LogAsync(&audit.LogEntry{
		UserID:     &userIDStr,
		Action:     auditctx.ActionFileDeleted,
		Module:     auditctx.ModuleFile,
		EntityType: lo.ToPtr(auditctx.EntityFile),
		EntityID:   &docIDStr,
		BeforeState: map[string]interface{}{
			"filename": doc.OriginalName,
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})
}
