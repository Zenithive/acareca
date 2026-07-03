package file

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

// Document status constants
const (
	StatusUploaded = "uploaded"
	StatusFailed   = "failed"
)

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
type RqListDocument struct {
	Status *string `form:"status" json:"status" validate:"omitempty,oneof=uploaded failed"`
	filter common.Filter
}

// RqGenerateShareLink represents a request to generate a temporary share link
type RqGenerateShareLink struct {
	ExpiresIn int `json:"expires_in" validate:"required,min=60,max=604800"` // 1 minute to 7 days
}

type RqGeneratePresignedUploadURL struct {
	ExpiresIn *int `form:"expires_in" validate:"omitempty,min=60,max=3600"`
}

// RsListDocuments represents a paginated list of documents
type RsListDocuments struct {
	Documents []common.RsDocument `json:"documents"`
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
	UploadURL   string    `json:"upload_url"`
	ObjectKey   string    `json:"object_key"`
	ExpiresAt   time.Time `json:"expires_at"`
	DocumentID  uuid.UUID `json:"document_id"`
	ContentType string    `json:"content_type"` // verified MIME type — client must send this as Content-Type on the PUT
}
