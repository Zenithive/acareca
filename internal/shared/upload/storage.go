package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/pkg/config"
)

var (
	ErrStorageNotConfigured = errors.New("storage provider not configured")
	ErrFileNotFound         = errors.New("file not found in storage")
	ErrInvalidFileType      = errors.New("invalid file type")
	ErrFileTooLarge         = errors.New("file size exceeds maximum allowed")
)

type StorageProvider interface {
	Upload(ctx context.Context, file multipart.File, header *multipart.FileHeader, objectKey string) (string, string, error)
	Download(ctx context.Context, objectKey string) (io.ReadCloser, error)
	Delete(ctx context.Context, objectKey string) error
	GetURL(objectKey string) string
	GenerateObjectKey(ownerID uuid.UUID, filename string) string
}

type PresignedURLProvider interface {
	StorageProvider
	GeneratePresignedURL(objectKey string, expiresIn time.Duration) (string, error)
	GeneratePresignedUploadURL(objectKey string, contentType string, expiresIn time.Duration) (string, error)
	HeadObject(ctx context.Context, objectKey string) (int64, error)
}

func NewStorageProvider(cfg *config.Config) (StorageProvider, error) {
	switch cfg.FileUploadStorageProvider {
	case "r2":
		return NewR2StorageProvider(cfg)
	default:
		return nil, fmt.Errorf("unsupported storage provider: %s", cfg.FileUploadStorageProvider)
	}
}

type FileValidator struct {
	maxFileSize      int64
	allowedMimeTypes map[string]bool
}

func NewFileValidator(maxFileSize int64, allowedMimeTypes []string) *FileValidator {
	mimeMap := make(map[string]bool)
	for _, mime := range allowedMimeTypes {
		mimeMap[mime] = true
	}

	return &FileValidator{
		maxFileSize:      maxFileSize,
		allowedMimeTypes: mimeMap,
	}
}

func (v *FileValidator) Validate(header *multipart.FileHeader) error {
	if header.Size > v.maxFileSize {
		return fmt.Errorf("%w: file size %d exceeds maximum %d", ErrFileTooLarge, header.Size, v.maxFileSize)
	}

	contentType := header.Header.Get("Content-Type")
	if len(v.allowedMimeTypes) > 0 && !v.allowedMimeTypes[contentType] {
		return fmt.Errorf("%w: %s not allowed", ErrInvalidFileType, contentType)
	}

	return nil
}

func GetFileExtension(filename string) string {
	ext := filepath.Ext(filename)
	if ext != "" {
		return strings.TrimPrefix(ext, ".")
	}
	return ""
}

func SanitizeFilename(filename string) string {
	filename = strings.ReplaceAll(filename, " ", "_")

	dangerous := []string{"..", "/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range dangerous {
		filename = strings.ReplaceAll(filename, char, "")
	}

	return filename
}

func DetectMimeType(file multipart.File) (string, error) {
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read file for mime detection: %w", err)
	}

	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
	}

	contentType := http.DetectContentType(buffer[:n])
	return contentType, nil
}
