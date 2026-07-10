package template

import "errors"

// Template errors
var (
	ErrNotFound             = errors.New("template not found")
	ErrInvalidTemplate      = errors.New("invalid template data")
	ErrTemplateRequired     = errors.New("at least one template ID is required")
	ErrTooManyTemplates     = errors.New("too many template IDs provided")
	ErrTemplateSizeExceeded = errors.New("template size exceeds maximum limit")
)

// Setting errors
var (
	ErrSettingNotFound = errors.New("setting not found")
	ErrInvalidSetting  = errors.New("invalid setting data")
)

// Invoice errors
var (
	ErrInvoiceNotFound = errors.New("invoice not found")
)

// Security errors
var (
	ErrUnauthorized         = errors.New("unauthorized access")
	ErrInvalidEncryptionKey = errors.New("encryption key must be exactly 32 characters")
)

// Size limit errors
var (
	ErrTemplateTooLarge  = errors.New("individual template exceeds size limit")
	ErrTotalSizeTooLarge = errors.New("total template size exceeds limit")
)

// Implementation errors
var (
	ErrNotImplemented = errors.New("not implemented - use new architecture")
)
