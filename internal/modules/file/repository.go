package file

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var (
	ErrDocumentNotFound      = errors.New("document not found")
	ErrDocumentAlreadyExists = errors.New("document already exists")
	ErrUnauthorizedAccess    = errors.New("unauthorized access to document")
)

type Repository interface {
	Create(ctx context.Context, doc *Document, tx *sqlx.Tx) (*Document, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Document, error)
	FindByObjectKey(ctx context.Context, objectKey string) (*Document, error)
	FindByOwner(ctx context.Context, ownerID uuid.UUID, filters *RqListDocuments) ([]Document, int64, error)
	Update(ctx context.Context, doc *Document, tx *sqlx.Tx) (*Document, error)
	Delete(ctx context.Context, id uuid.UUID, tx *sqlx.Tx) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status string, doc *Document, tx *sqlx.Tx) error
	FindPendingUploads(ctx context.Context, limit int) ([]Document, error)
	FindExpiredPendingUploads(ctx context.Context, limit int) ([]Document, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

// Create creates a new document record
func (r *repository) Create(ctx context.Context, doc *Document, tx *sqlx.Tx) (*Document, error) {
	query := `
		INSERT INTO tbl_document (
			owner_id, owner_role, object_key, bucket,
			original_name, extension, mime_type, size_bytes,
			checksum, status, is_public,
			upload_expires_at, uploaded_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10, $11,
			$14, $15
		) RETURNING id, created_at, updated_at`

	var id uuid.UUID
	var createdAt, updatedAt time.Time

	err := tx.QueryRowxContext(ctx, query,
		doc.OwnerID, doc.OwnerRole, doc.ObjectKey, doc.Bucket,
		doc.OriginalName, doc.Extension, doc.MimeType, doc.SizeBytes,
		doc.Checksum, doc.Status, doc.IsPublic,
		doc.UploadExpiresAt, doc.UploadedAt,
	).Scan(&id, &createdAt, &updatedAt)

	if err != nil {
		return nil, fmt.Errorf("create document: %w", err)
	}

	doc.ID = id
	doc.CreatedAt = createdAt
	doc.UpdatedAt = updatedAt

	return doc, nil
}

// FindByID finds a document by ID
func (r *repository) FindByID(ctx context.Context, id uuid.UUID) (*Document, error) {
	query := `
		SELECT 
			id, owner_id, owner_role, object_key, bucket,
			original_name, extension, mime_type, size_bytes,
			checksum, status, is_public,
			upload_expires_at, uploaded_at,
			created_at, updated_at, deleted_at
		FROM tbl_document
		WHERE id = $1 AND deleted_at IS NULL`

	var doc Document
	err := r.db.GetContext(ctx, &doc, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("find document by id: %w", err)
	}

	return &doc, nil
}

// FindByObjectKey finds a document by object key
func (r *repository) FindByObjectKey(ctx context.Context, objectKey string) (*Document, error) {
	query := `
		SELECT 
			id, owner_id, owner_role, object_key, bucket,
			original_name, extension, mime_type, size_bytes,
			checksum, status, is_public,
			upload_expires_at, uploaded_at,
			created_at, updated_at, deleted_at
		FROM tbl_document
		WHERE object_key = $1 AND deleted_at IS NULL`

	var doc Document
	err := r.db.GetContext(ctx, &doc, query, objectKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("find document by object key: %w", err)
	}

	return &doc, nil
}

// FindByOwner finds documents by owner with pagination and filters
func (r *repository) FindByOwner(ctx context.Context, ownerID uuid.UUID, filters *RqListDocuments) ([]Document, int64, error) {
	// Build query with filters
	query := `
		SELECT 
			id, owner_id, owner_role, object_key, bucket,
			original_name, extension, mime_type, size_bytes,
			checksum, status, is_public,
			upload_expires_at, uploaded_at,
			created_at, updated_at, deleted_at
		FROM tbl_document
		WHERE owner_id = $1 AND deleted_at IS NULL`

	countQuery := `SELECT COUNT(*) FROM tbl_document WHERE owner_id = $1 AND deleted_at IS NULL`

	args := []interface{}{ownerID}
	argCount := 1

	// Apply filters
	if filters.Status != nil {
		argCount++
		query += fmt.Sprintf(" AND status = $%d", argCount)
		countQuery += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, *filters.Status)
	}

	// Get total count
	var total int64
	err := r.db.GetContext(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("count documents: %w", err)
	}

	// Execute query
	var docs []Document
	err = r.db.SelectContext(ctx, &docs, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("find documents by owner: %w", err)
	}

	return docs, total, nil
}

// Update updates a document
func (r *repository) Update(ctx context.Context, doc *Document, tx *sqlx.Tx) (*Document, error) {
	query := `
		UPDATE tbl_document
		SET 
			original_name = $1,
			extension = $2,
			is_public = $3,
			updated_at = NOW()
		WHERE id = $4 AND deleted_at IS NULL
		RETURNING updated_at`

	var updatedAt time.Time
	err := tx.QueryRowxContext(ctx, query,
		doc.OriginalName,
		doc.Extension,
		doc.IsPublic,
		doc.ID,
	).Scan(&updatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("update document: %w", err)
	}

	doc.UpdatedAt = updatedAt
	return doc, nil
}

// UpdateStatus updates document status
func (r *repository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, doc *Document, tx *sqlx.Tx) error {
	uploadedAt := doc.UploadedAt
	if status == StatusUploaded {
		now := time.Now()
		uploadedAt = &now
	}

	query := `
	UPDATE tbl_document
	SET 
		status = $1,
		uploaded_at = $2,
		updated_at = NOW()
	WHERE id = $3 AND deleted_at IS NULL`

	var result sql.Result
	var err error

	result, err = r.db.ExecContext(ctx, query, status, uploadedAt, id)

	if err != nil {
		return fmt.Errorf("update document status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrDocumentNotFound
	}

	return nil
}

// Delete soft deletes a document
func (r *repository) Delete(ctx context.Context, id uuid.UUID, tx *sqlx.Tx) error {
	query := `
		UPDATE tbl_document
		SET 
			deleted_at = NOW(),
			status = 'deleted',
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`

	result, err := tx.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete document: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrDocumentNotFound
	}

	return nil
}

// FindPendingUploads finds documents with pending status
func (r *repository) FindPendingUploads(ctx context.Context, limit int) ([]Document, error) {
	query := `
		SELECT 
			id, owner_id, owner_role, object_key, bucket,
			original_name, extension, mime_type, size_bytes,
			checksum, status, is_public,
			upload_expires_at, uploaded_at,
			created_at, updated_at, deleted_at
		FROM tbl_document
		WHERE status = $1 
			AND deleted_at IS NULL
			AND (upload_expires_at IS NULL OR upload_expires_at > NOW())
		ORDER BY created_at ASC
		LIMIT $2`

	var docs []Document
	err := r.db.SelectContext(ctx, &docs, query, StatusPending, limit)
	if err != nil {
		return nil, fmt.Errorf("find pending uploads: %w", err)
	}

	return docs, nil
}

// FindExpiredPendingUploads finds documents with expired pending status
func (r *repository) FindExpiredPendingUploads(ctx context.Context, limit int) ([]Document, error) {
	query := `
		SELECT 
			id, owner_id, owner_role, object_key, bucket,
			original_name, extension, mime_type, size_bytes,
			checksum, status, is_public,
			upload_expires_at, uploaded_at,
			created_at, updated_at, deleted_at
		FROM tbl_document
		WHERE status = $1 
			AND deleted_at IS NULL
			AND upload_expires_at IS NOT NULL 
			AND upload_expires_at <= NOW()
		ORDER BY created_at ASC
		LIMIT $2`

	var docs []Document
	err := r.db.SelectContext(ctx, &docs, query, StatusPending, limit)
	if err != nil {
		return nil, fmt.Errorf("find expired pending uploads: %w", err)
	}

	return docs, nil
}
