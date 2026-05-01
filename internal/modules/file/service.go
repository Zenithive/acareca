package file

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	UploadFile(ctx context.Context, file multipart.File, header *multipart.FileHeader, req *RqUploadFile, ownerID uuid.UUID, ownerRole string) (*RsUploadDocument, error)
	UploadMultipleFiles(ctx context.Context, files []*multipart.FileHeader, req *RqUploadFile, ownerID uuid.UUID, ownerRole string) ([]*RsUploadDocument, error)
	GetDocument(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*RsDocument, error)
	DownloadFile(ctx context.Context, id uuid.UUID, userID uuid.UUID) (io.ReadCloser, *Document, error)
	ListDocuments(ctx context.Context, userID uuid.UUID, filters *RqListDocuments) (*util.RsList, error)
	ListDocumentsByEntity(ctx context.Context, entityType string, entityID uuid.UUID, userID uuid.UUID, filters *RqListDocuments) (*util.RsList, error)
	UpdateDocument(ctx context.Context, id uuid.UUID, req *RqUpdateDocument, userID uuid.UUID) (*RsDocument, error)
	DeleteDocument(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	GenerateShareLink(ctx context.Context, id uuid.UUID, userID uuid.UUID, expiresIn int) (*RsShareLink, error)
	GeneratePresignedUploadURL(ctx context.Context, req *RqGeneratePresignedUploadURL, ownerID uuid.UUID, ownerRole string) (*RsPresignedUploadURL, error)
	ConfirmUpload(ctx context.Context, documentID uuid.UUID, userID uuid.UUID) error
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

// UploadFile uploads a single file
func (s *service) UploadFile(ctx context.Context, file multipart.File, header *multipart.FileHeader, req *RqUploadFile, ownerID uuid.UUID, ownerRole string) (*RsUploadDocument, error) {
	if err := s.validator.Validate(header); err != nil {
		return nil, err
	}

	objectKey := s.storage.GenerateObjectKey(ownerID, header.Filename)

	storedKey, checksum, err := s.storage.Upload(ctx, file, header, objectKey)
	if err != nil {
		return nil, fmt.Errorf("upload to storage: %w", err)
	}

	ext := upload.GetFileExtension(header.Filename)
	mimeType := header.Header.Get("Content-Type")

	doc := &Document{
		OwnerID:      ownerID,
		OwnerRole:    ownerRole,
		ObjectKey:    storedKey,
		Bucket:       s.bucket,
		OriginalName: upload.SanitizeFilename(header.Filename),
		Extension:    &ext,
		MimeType:     mimeType,
		SizeBytes:    header.Size,
		Checksum:     &checksum,
		Status:       StatusUploaded,
		IsPublic:     req.IsPublic != nil && *req.IsPublic,
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

	now := time.Now()
	doc.UploadedAt = &now

	created, err := s.repo.Create(ctx, doc, nil)
	if err != nil {
		s.storage.Delete(ctx, storedKey)
		return nil, fmt.Errorf("save document record: %w", err)
	}

	s.logFileUpload(ctx, created, ownerID)

	baseURL, _ := s.cfg.GetBaseURL()
	return created.ToRsUploadDocument(baseURL), nil
}

// UploadMultipleFiles uploads multiple files
func (s *service) UploadMultipleFiles(ctx context.Context, files []*multipart.FileHeader, req *RqUploadFile, ownerID uuid.UUID, ownerRole string) ([]*RsUploadDocument, error) {
	results := make([]*RsUploadDocument, 0, len(files))
	errors := make([]error, 0)

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			errors = append(errors, fmt.Errorf("open file %s: %w", fileHeader.Filename, err))
			continue
		}
		defer file.Close()

		result, err := s.UploadFile(ctx, file, fileHeader, req, ownerID, ownerRole)
		if err != nil {
			errors = append(errors, fmt.Errorf("upload file %s: %w", fileHeader.Filename, err))
			continue
		}

		results = append(results, result)
	}

	if len(errors) > 0 && len(results) == 0 {
		return nil, fmt.Errorf("all uploads failed: %v", errors)
	}

	return results, nil
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

// DownloadFile downloads a file
func (s *service) DownloadFile(ctx context.Context, id uuid.UUID, userID uuid.UUID) (io.ReadCloser, *Document, error) {
	doc, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	if !s.canAccessDocument(doc, userID) {
		return nil, nil, ErrUnauthorizedAccess
	}

	file, err := s.storage.Download(ctx, doc.ObjectKey)
	if err != nil {
		return nil, nil, fmt.Errorf("download from storage: %w", err)
	}

	s.logFileDownload(ctx, doc, userID)

	return file, doc, nil
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

// ListDocumentsByEntity lists documents for a specific entity
func (s *service) ListDocumentsByEntity(ctx context.Context, entityType string, entityID uuid.UUID, userID uuid.UUID, filters *RqListDocuments) (*util.RsList, error) {
	docs, total, err := s.repo.FindByEntity(ctx, entityType, entityID, filters)
	if err != nil {
		return nil, err
	}

	// Filter documents user can access
	accessibleDocs := make([]Document, 0)
	for _, doc := range docs {
		if s.canAccessDocument(&doc, userID) {
			accessibleDocs = append(accessibleDocs, doc)
		}
	}

	baseURL, _ := s.cfg.GetBaseURL()
	rsDocs := make([]RsDocument, len(accessibleDocs))
	for i, doc := range accessibleDocs {
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

	// Save changes
	updated, err := s.repo.Update(ctx, doc, nil)
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
	doc, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// Check access permission
	if doc.OwnerID != userID {
		return ErrUnauthorizedAccess
	}

	// Soft delete in database
	if err := s.repo.Delete(ctx, id, nil); err != nil {
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

// GenerateShareLink generates a temporary share link
func (s *service) GenerateShareLink(ctx context.Context, id uuid.UUID, userID uuid.UUID, expiresIn int) (*RsShareLink, error) {
	doc, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Check access permission
	if !s.canAccessDocument(doc, userID) {
		return nil, ErrUnauthorizedAccess
	}

	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)

	// Check if storage provider supports presigned URLs
	if presignedProvider, ok := s.storage.(upload.PresignedURLProvider); ok {
		// Generate presigned URL
		url, err := presignedProvider.GeneratePresignedURL(doc.ObjectKey, time.Duration(expiresIn)*time.Second)
		if err != nil {
			return nil, fmt.Errorf("generate presigned URL: %w", err)
		}

		return &RsShareLink{
			URL:       url,
			Token:     "", // No token needed for presigned URLs
			ExpiresAt: expiresAt,
		}, nil
	}

	// Fallback to token-based sharing for local storage
	token := uuid.New().String()
	baseURL, _ := s.cfg.GetBaseURL()
	url := fmt.Sprintf("%s/api/v1/files/%s/download?token=%s", baseURL, id.String(), token)

	return &RsShareLink{
		URL:       url,
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

// GeneratePresignedUploadURL generates a presigned URL for direct upload to R2
func (s *service) GeneratePresignedUploadURL(ctx context.Context, req *RqGeneratePresignedUploadURL, ownerID uuid.UUID, ownerRole string) (*RsPresignedUploadURL, error) {
	// Check if storage provider supports presigned URLs
	presignedProvider, ok := s.storage.(upload.PresignedURLProvider)
	if !ok {
		return nil, errors.New("presigned URLs not supported by current storage provider")
	}

	// Generate object key
	objectKey := s.storage.GenerateObjectKey(ownerID, req.Filename)

	// Generate presigned upload URL
	expiresIn := time.Duration(*req.ExpiresIn) * time.Second
	uploadURL, err := presignedProvider.GeneratePresignedUploadURL(objectKey, req.ContentType, expiresIn)
	if err != nil {
		return nil, fmt.Errorf("generate presigned upload URL: %w", err)
	}

	// Create pending document record
	ext := upload.GetFileExtension(req.Filename)
	doc := &Document{
		OwnerID:      ownerID,
		OwnerRole:    ownerRole,
		ObjectKey:    objectKey,
		Bucket:       s.bucket,
		OriginalName: upload.SanitizeFilename(req.Filename),
		Extension:    &ext,
		MimeType:     req.ContentType,
		SizeBytes:    0, // Will be updated after upload
		Status:       StatusPending,
		IsPublic:     false,
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

	// Save to database
	created, err := s.repo.Create(ctx, doc, nil)
	if err != nil {
		return nil, fmt.Errorf("save document record: %w", err)
	}

	return &RsPresignedUploadURL{
		UploadURL:  uploadURL,
		ObjectKey:  objectKey,
		ExpiresAt:  expiresAt,
		DocumentID: created.ID,
	}, nil
}

// ConfirmUpload confirms that a file was uploaded via presigned URL
func (s *service) ConfirmUpload(ctx context.Context, documentID uuid.UUID, userID uuid.UUID) error {
	doc, err := s.repo.FindByID(ctx, documentID)
	if err != nil {
		return err
	}

	// Check ownership
	if doc.OwnerID != userID {
		return ErrUnauthorizedAccess
	}

	// Check if document is in pending state
	if doc.Status != StatusPending {
		return fmt.Errorf("document is not in pending state")
	}

	// Check if upload has expired
	if doc.UploadExpiresAt != nil && time.Now().After(*doc.UploadExpiresAt) {
		return fmt.Errorf("upload has expired")
	}

	// Verify file exists in storage (for R2)
	if r2Provider, ok := s.storage.(*upload.R2StorageProvider); ok {
		headResult, err := r2Provider.HeadObject(ctx, doc.ObjectKey)
		if err != nil {
			return fmt.Errorf("file not found in storage: %w", err)
		}

		// Update file size from R2
		if headResult.ContentLength != nil {
			doc.SizeBytes = *headResult.ContentLength
		}
	}

	// Update status to uploaded
	if err := s.repo.UpdateStatus(ctx, documentID, StatusUploaded, nil); err != nil {
		return err
	}

	// Audit log
	s.logFileUpload(ctx, doc, userID)

	return nil
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
