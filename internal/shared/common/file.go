package common

import (
	"time"

	"github.com/google/uuid"
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

	UploadedAt *time.Time `db:"uploaded_at"`

	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
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

// ToRsDocument converts Document to RsDocument
func (d *Document) ToRsDocument() *RsDocument {
	return &RsDocument{
		ID:           d.ID,
		OriginalName: d.OriginalName,
		FileKey:      d.ObjectKey,
		UploadedAt:   d.UploadedAt,
		CreatedAt:    d.CreatedAt,
	}
}

// ToRsUploadDocument converts Document to RsUploadDocument
func (d *Document) ToRsUploadDocument(baseURL string) *RsDocument {
	return &RsDocument{
		ID:           d.ID,
		OriginalName: d.OriginalName,
		FileKey:      d.ObjectKey,
		UploadedAt:   d.UploadedAt,
		CreatedAt:    d.CreatedAt,
	}
}
