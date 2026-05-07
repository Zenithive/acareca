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

	baseURL, err := s.cfg.GetBaseURL()
	if err != nil {
		return nil, fmt.Errorf("get base URL: %w", err)
	}
	return doc.ToRsDocument(baseURL), nil
}

// ListDocuments lists documents for a user
func (s *service) ListDocuments(ctx context.Context, userID uuid.UUID, filters *RqListDocuments) (*util.RsList, error) {

	docs, total, err := s.repo.FindByOwner(ctx, userID, filters)
	if err != nil {
		return nil, err
	}

	baseURL, err := s.cfg.GetBaseURL()
	if err != nil {
		return nil, fmt.Errorf("get base URL: %w", err)
	}

	rsDocs := make([]RsDocument, len(docs))
	for i, doc := range docs {
		rsDocs[i] = *doc.ToRsDocument(baseURL)
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

	baseURL, err := s.cfg.GetBaseURL()
	if err != nil {
		return nil, fmt.Errorf("get base URL: %w", err)
	}
	return updated.ToRsDocument(baseURL), nil
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
	return nil
}

// GeneratePresignedUploadURL generates a presigned URL for direct client upload to R2.
func (s *service) GeneratePresignedUploadURL(ctx context.Context, req *RqGeneratePresignedUploadURL, ownerID uuid.UUID, ownerRole string, file multipart.File, header *multipart.FileHeader) (*RsPresignedUploadURL, error) {
	presignedProvider, ok := s.storage.(upload.PresignedURLProvider)
	if !ok {
		return nil, errors.New("presigned URLs not supported by current storage provider")
	}

	objectKey := s.storage.GenerateObjectKey(ownerID, header.Filename)

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
	}

	expiresAt := time.Now().Add(expiresIn)
	doc.UploadExpiresAt = &expiresAt

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

	_, err = s.cfg.GetBaseURL()
	if err != nil {
		return nil, fmt.Errorf("get base URL: %w", err)
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

func (s *service) logFileUpdate(ctx context.Context, doc *Document, userID uuid.UUID, beforeState map[string]interface{}) {
	meta := auditctx.GetMetadata(ctx)
	userIDStr := userID.String()
	docIDStr := doc.ID.String()

	s.auditSvc.LogAsync(&audit.LogEntry{
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
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
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
			"filename":   doc.OriginalName,
			"object_key": doc.ObjectKey,
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})
}
