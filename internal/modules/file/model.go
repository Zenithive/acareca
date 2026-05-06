package file

import (
	"time"

	"github.com/google/uuid"
)

// Document represents a file in the system
type Document struct {
	ID        uuid.UUID `db:"id" json:"id"`
	OwnerID   uuid.UUID `db:"owner_id" json:"owner_id"`
	OwnerRole string    `db:"owner_role" json:"owner_role"`
	ObjectKey string    `db:"object_key" json:"object_key"`
	Bucket    string    `db:"bucket" json:"bucket"`

	OriginalName string  `db:"original_name" json:"original_name"`
	Extension    *string `db:"extension" json:"extension,omitempty"`
	MimeType     string  `db:"mime_type" json:"mime_type"`
	SizeBytes    int64   `db:"size_bytes" json:"size_bytes"`

	Checksum *string `db:"checksum" json:"checksum,omitempty"`
	Status   string  `db:"status" json:"status"`
	IsPublic bool    `db:"is_public" json:"is_public"`

	EntityType *string    `db:"entity_type" json:"entity_type,omitempty"`
	EntityID   *uuid.UUID `db:"entity_id" json:"entity_id,omitempty"`

	UploadExpiresAt *time.Time `db:"upload_expires_at" json:"upload_expires_at,omitempty"`
	UploadedAt      *time.Time `db:"uploaded_at" json:"uploaded_at,omitempty"`

	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// Document status constants
const (
	StatusPending  = "pending"
	StatusUploaded = "uploaded"
	StatusFailed   = "failed"
	StatusDeleted  = "deleted"
)

// Entity types
const (
	EntityTypePractitioner = "practitioner"
	EntityTypeAccountant   = "accountant"
	EntityTypeAdmin        = "admin"
	EntityTypeClinic       = "clinic"
	EntityTypeTransaction  = "transaction"
	EntityTypeInvoice      = "invoice"
	EntityTypeReport       = "report"
	EntityTypeUser         = "user"
	EntityTypeBusiness     = "business"
	EntityTypeForm         = "form"
	EntityTypeFormEntry    = "form_entry"
)

// Request models

// RqUploadFile represents a file upload request
type RqUploadFile struct {
	EntityType  *string `form:"entity_type" json:"entity_type" validate:"omitempty,oneof=practitioner accountant admin clinic transaction invoice report user business form form_entry"`
	EntityID    *string `form:"entity_id" json:"entity_id" validate:"omitempty,uuid"`
	IsPublic    *bool   `form:"is_public" json:"is_public"`
	Description *string `form:"description" json:"description"`
}

// RqUpdateDocument represents a document metadata update request
type RqUpdateDocument struct {
	OriginalName *string `json:"original_name" validate:"omitempty"`
	EntityType   *string `json:"entity_type" validate:"omitempty,oneof=practitioner accountant admin clinic transaction invoice report user business form form_entry"`
	EntityID     *string `json:"entity_id" validate:"omitempty,uuid"`
	IsPublic     *bool   `json:"is_public"`
}

// RqListDocuments represents a document list request
type RqListDocuments struct {
	EntityType *string `form:"entity_type" json:"entity_type" validate:"omitempty"`
	EntityID   *string `form:"entity_id" json:"entity_id" validate:"omitempty,uuid"`
	Status     *string `form:"status" json:"status" validate:"omitempty,oneof=pending uploaded failed deleted"`
	Page       int     `form:"page" json:"page" validate:"omitempty,min=1"`
	Limit      int     `form:"limit" json:"limit" validate:"omitempty,min=1,max=100"`
	Sort       string  `form:"sort" json:"sort" validate:"omitempty,oneof=created_at updated_at size_bytes original_name"`
	Order      string  `form:"order" json:"order" validate:"omitempty,oneof=asc desc"`
}

// RqGenerateShareLink represents a request to generate a temporary share link
type RqGenerateShareLink struct {
	ExpiresIn int `json:"expires_in" validate:"required,min=60,max=604800"` // 1 minute to 7 days
}

type RqGeneratePresignedUploadURL struct {
	ExpiresIn  *int    `form:"expires_in" validate:"omitempty,min=60,max=3600"`
	EntityType *string `form:"entity_type" validate:"omitempty,oneof=practitioner accountant admin clinic transaction invoice report user business form form_entry"`
	EntityID   *string `form:"entity_id" validate:"omitempty,uuid"`
}

// Response models
// RsDocument represents a document response
type RsDocument struct {
	ID           uuid.UUID  `json:"id"`
	OriginalName string     `json:"original_name"`
	Extension    *string    `json:"extension,omitempty"`
	MimeType     string     `json:"mime_type"`
	SizeBytes    int64      `json:"size_bytes"`
	Status       string     `json:"status"`
	IsPublic     bool       `json:"is_public"`
	EntityType   *string    `json:"entity_type,omitempty"`
	EntityID     *uuid.UUID `json:"entity_id,omitempty"`
	DownloadURL  string     `json:"download_url"`
	UploadedAt   *time.Time `json:"uploaded_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// RsUploadDocument represents an upload response
type RsUploadDocument struct {
	ID           uuid.UUID `json:"id"`
	OriginalName string    `json:"original_name"`
	SizeBytes    int64     `json:"size_bytes"`
	MimeType     string    `json:"mime_type"`
	DownloadURL  string    `json:"download_url"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// RsListDocuments represents a paginated list of documents
type RsListDocuments struct {
	Documents  []RsDocument     `json:"documents"`
	Pagination RsPaginationMeta `json:"pagination"`
}

// RsPaginationMeta represents pagination metadata
type RsPaginationMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// RsShareLink represents a temporary share link response
type RsShareLink struct {
	URL       string    `json:"url"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// RsPresignedUploadURL represents a presigned upload URL response
type RsPresignedUploadURL struct {
	UploadURL  string    `json:"upload_url"`
	ObjectKey  string    `json:"object_key"`
	ExpiresAt  time.Time `json:"expires_at"`
	DocumentID uuid.UUID `json:"document_id"`
}

// ToRsDocument converts Document to RsDocument
func (d *Document) ToRsDocument(baseURL string) *RsDocument {
	return &RsDocument{
		ID:           d.ID,
		OriginalName: d.OriginalName,
		Extension:    d.Extension,
		MimeType:     d.MimeType,
		SizeBytes:    d.SizeBytes,
		Status:       d.Status,
		IsPublic:     d.IsPublic,
		EntityType:   d.EntityType,
		EntityID:     d.EntityID,
		DownloadURL:  baseURL + "/api/v1/files/" + d.ID.String() + "/download",
		UploadedAt:   d.UploadedAt,
		CreatedAt:    d.CreatedAt,
	}
}

// ToRsUploadDocument converts Document to RsUploadDocument
func (d *Document) ToRsUploadDocument(baseURL string) *RsUploadDocument {
	return &RsUploadDocument{
		ID:           d.ID,
		OriginalName: d.OriginalName,
		SizeBytes:    d.SizeBytes,
		MimeType:     d.MimeType,
		DownloadURL:  baseURL + "/api/v1/files/" + d.ID.String() + "/download",
		Status:       d.Status,
		CreatedAt:    d.CreatedAt,
	}
}

// FileUploadConfig represents file upload configuration
type FileUploadConfig struct {
	MaxFileSize      int64    // Maximum file size in bytes
	AllowedMimeTypes []string // Allowed MIME types
	StorageProvider  string   // Storage provider: local, s3, r2
	LocalPath        string   // Local storage path
	BaseURL          string   // Base URL for file access
}
