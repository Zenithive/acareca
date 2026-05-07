package file

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

// Document represents a file in the system
type Document struct {
	ID        uuid.UUID `db:"id"`
	OwnerID   uuid.UUID `db:"owner_id"`
	OwnerRole string    `db:"owner_role"`
	ObjectKey string    `db:"object_key"`
	Bucket    string    `db:"bucket"`

	OriginalName string  `db:"original_name"`
	Extension    *string `db:"extension"`
	MimeType     string  `db:"mime_type"`
	SizeBytes    int64   `db:"size_bytes"`

	Checksum *string `db:"checksum"`
	Status   string  `db:"status"`
	IsPublic bool    `db:"is_public"`

	UploadExpiresAt *time.Time `db:"upload_expires_at"`
	UploadedAt      *time.Time `db:"uploaded_at"`

	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

// Document status constants
const (
	StatusPending  = "pending"
	StatusUploaded = "uploaded"
	StatusFailed   = "failed"
	StatusDeleted  = "deleted"
)

// Request models

// RqUploadFile represents a file upload request
type RqUploadFile struct {
	IsPublic    *bool   `form:"is_public" json:"is_public"`
	Description *string `form:"description" json:"description"`
}

// RqUpdateDocument represents a document metadata update request
type RqUpdateDocument struct {
	OriginalName *string `json:"original_name" validate:"omitempty"`
	IsPublic     *bool   `json:"is_public"`
}

// RqListDocuments represents a document list request
type RqListDocuments struct {
	Status *string `form:"status" json:"status" validate:"omitempty,oneof=pending uploaded failed deleted"`
	filter common.Filter
}

// RqGenerateShareLink represents a request to generate a temporary share link
type RqGenerateShareLink struct {
	ExpiresIn int `json:"expires_in" validate:"required,min=60,max=604800"` // 1 minute to 7 days
}

type RqGeneratePresignedUploadURL struct {
	ExpiresIn *int `form:"expires_in" validate:"omitempty,min=60,max=3600"`
}

// Response models
// RsDocument represents a document response
type RsDocument struct {
	ID           uuid.UUID  `json:"id"`
	OriginalName string     `json:"original_name"`
	FileKey      string     `json:"file_key"`
	UploadedAt   *time.Time `json:"uploaded_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// RsUploadDocument represents an upload response
type RsUploadDocument struct {
	ID           uuid.UUID `json:"id"`
	OriginalName string    `json:"original_name"`
	FileKey      string    `json:"file_key"`
	CreatedAt    time.Time `json:"created_at"`
}

// RsListDocuments represents a paginated list of documents
type RsListDocuments struct {
	Documents []RsDocument `json:"documents"`
	// Pagination RsPaginationMeta `json:"pagination"`
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
		FileKey:      d.ObjectKey,
		UploadedAt:   d.UploadedAt,
		CreatedAt:    d.CreatedAt,
	}
}

// ToRsUploadDocument converts Document to RsUploadDocument
func (d *Document) ToRsUploadDocument(baseURL string) *RsUploadDocument {
	return &RsUploadDocument{
		ID:           d.ID,
		OriginalName: d.OriginalName,
		FileKey:      d.ObjectKey,
		CreatedAt:    d.CreatedAt,
	}
}
