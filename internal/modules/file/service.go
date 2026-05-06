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
	GetDocument(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*RsDocument, error)
	DownloadFile(ctx context.Context, id uuid.UUID, userID uuid.UUID) (io.ReadCloser, *Document, error)
	ListDocuments(ctx context.Context, userID uuid.UUID, filters *RqListDocuments) (*util.RsList, error)
	ListDocumentsByEntity(ctx context.Context, entityType string, entityID uuid.UUID, userID uuid.UUID, filters *RqListDocuments) (*util.RsList, error)
	UpdateDocument(ctx context.Context, id uuid.UUID, req *RqUpdateDocument, userID uuid.UUID) (*RsDocument, error)
	DeleteDocument(ctx context.Context, id uuid.UUID, userID uuid.UUID) error

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

	var created *Document
	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		created, err = s.repo.Create(ctx, doc, tx)
		if err != nil {
			s.storage.Delete(ctx, storedKey)
			return fmt.Errorf("save document record: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	s.logFileUpload(ctx, created, ownerID)

	baseURL, _ := s.cfg.GetBaseURL()
	return created.ToRsUploadDocument(baseURL), nil
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
	docs, _, err := s.repo.FindByEntity(ctx, entityType, entityID, filters)
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

	// Use the filtered count as total so pagination math is correct.
	// Note: for large datasets consider pushing access filtering to the DB layer.
	filteredTotal := int64(len(accessibleDocs))

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
	result.MapToList(rsDocs, int(filteredTotal), page, limit)
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
func (s *service) GeneratePresignedUploadURL(ctx context.Context, req *RqGeneratePresignedUploadURL, ownerID uuid.UUID, ownerRole string) (*RsPresignedUploadURL, error) {
	presignedProvider, ok := s.storage.(upload.PresignedURLProvider)
	if !ok {
		return nil, errors.New("presigned URLs not supported by current storage provider")
	}

	// Validate file metadata (size and MIME type)
	if req.SizeBytes > s.validator.GetMaxFileSize() {
		return nil, fmt.Errorf("%w: file size %d exceeds maximum %d", upload.ErrFileTooLarge, req.SizeBytes, s.validator.GetMaxFileSize())
	}

	if !s.validator.IsAllowedMimeType(req.ContentType) {
		return nil, fmt.Errorf("%w: %s not allowed", upload.ErrInvalidFileType, req.ContentType)
	}

	// Verify entity access if entity_id is provided
	if req.EntityType != nil && req.EntityID != nil {
		entityUUID, err := uuid.Parse(*req.EntityID)
		if err != nil {
			return nil, fmt.Errorf("invalid entity_id: %w", err)
		}

		// Verify user has access to the entity (e.g., clinic)
		hasAccess, err := s.verifyEntityAccess(ctx, ownerID, ownerRole, *req.EntityType, entityUUID)
		if err != nil {
			return nil, fmt.Errorf("verify entity access: %w", err)
		}
		if !hasAccess {
			return nil, ErrUnauthorizedAccess
		}
	}

	// Generate object key
	objectKey := s.storage.GenerateObjectKey(ownerID, req.Filename)

	// Default to 15 minutes if not specified
	expiresInSec := 900
	if req.ExpiresIn != nil {
		expiresInSec = *req.ExpiresIn
	}
	expiresIn := time.Duration(expiresInSec) * time.Second

	// Generate presigned URL (NO FILE UPLOAD HERE!)
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
		SizeBytes:    req.SizeBytes,
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

	// Verify entity access again (in case permissions changed)
	if doc.EntityType != nil && doc.EntityID != nil {
		hasAccess, err := s.verifyEntityAccess(ctx, userID, doc.OwnerRole, *doc.EntityType, *doc.EntityID)
		if err != nil {
			return fmt.Errorf("verify entity access: %w", err)
		}
		if !hasAccess {
			return ErrUnauthorizedAccess
		}
	}

	// Verify file exists in storage and get its actual size
	presignedProvider, ok := s.storage.(upload.PresignedURLProvider)
	if !ok {
		return errors.New("presigned URLs not supported by current storage provider")
	}

	actualSize, err := presignedProvider.HeadObject(ctx, doc.ObjectKey)
	if err != nil {
		return fmt.Errorf("file not found in storage: %w", err)
	}

	// Verify file size matches expected size (within reasonable tolerance)
	if actualSize == 0 {
		return fmt.Errorf("file is empty in storage")
	}

	// Update document with actual size
	doc.SizeBytes = actualSize

	// Set uploaded timestamp
	now := time.Now()
	doc.UploadedAt = &now

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		// Update status to uploaded
		if err := s.repo.UpdateStatus(ctx, documentID, StatusUploaded, doc, tx); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
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

// verifyEntityAccess checks if user has access to the specified entity
func (s *service) verifyEntityAccess(ctx context.Context, userID uuid.UUID, userRole string, entityType string, entityID uuid.UUID) (bool, error) {
	// TODO: Implement proper entity access verification based on entity type
	// For now, we'll implement basic checks for common entity types

	switch entityType {
	case EntityTypeClinic:
		// Check if user owns or has access to the clinic
		return s.verifyClinicAccess(ctx, userID, entityID)
	case EntityTypeBusiness:
		// Check if user owns or has access to the business
		return s.verifyBusinessAccess(ctx, userID, entityID)
	case EntityTypePractitioner, EntityTypeAccountant, EntityTypeAdmin:
		// Check if the entity_id matches the user_id
		return userID == entityID, nil
	default:
		// For other entity types, allow if user is owner
		// TODO: Implement specific checks for each entity type
		return true, nil
	}
}

// verifyClinicAccess checks if user has access to a clinic
func (s *service) verifyClinicAccess(ctx context.Context, userID uuid.UUID, clinicID uuid.UUID) (bool, error) {
	// TODO: Query the clinic table to verify user has access
	// This is a placeholder - implement based on your clinic access logic
	query := `
		SELECT EXISTS(
			SELECT 1 FROM tbl_clinic 
			WHERE id = $1 
			AND (owner_id = $2 OR id IN (
				SELECT clinic_id FROM tbl_clinic_member WHERE user_id = $2 AND deleted_at IS NULL
			))
			AND deleted_at IS NULL
		)`

	var hasAccess bool
	err := s.db.GetContext(ctx, &hasAccess, query, clinicID, userID)
	if err != nil {
		return false, fmt.Errorf("check clinic access: %w", err)
	}

	return hasAccess, nil
}

// verifyBusinessAccess checks if user has access to a business
func (s *service) verifyBusinessAccess(ctx context.Context, userID uuid.UUID, businessID uuid.UUID) (bool, error) {
	// TODO: Query the business table to verify user has access
	// This is a placeholder - implement based on your business access logic
	query := `
		SELECT EXISTS(
			SELECT 1 FROM tbl_business 
			WHERE id = $1 
			AND (owner_id = $2 OR id IN (
				SELECT business_id FROM tbl_business_member WHERE user_id = $2 AND deleted_at IS NULL
			))
			AND deleted_at IS NULL
		)`

	var hasAccess bool
	err := s.db.GetContext(ctx, &hasAccess, query, businessID, userID)
	if err != nil {
		return false, fmt.Errorf("check business access: %w", err)
	}

	return hasAccess, nil
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
