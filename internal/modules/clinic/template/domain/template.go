package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Template represents the domain entity with business rules
type Template struct {
	ID          uuid.UUID
	Name        string
	Description *string
	Html        []byte
	Css         []byte
	IsDefault   bool
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   *time.Time
	DeletedAt   *time.Time
}

// Validate checks if template meets business rules
func (t *Template) Validate() error {
	if t.Name == "" {
		return errors.New("template name is required")
	}
	if len(t.Name) > 100 {
		return errors.New("template name too long (max 100 characters)")
	}
	if len(t.Html) == 0 {
		return errors.New("template HTML is required")
	}
	if len(t.Css) == 0 {
		return errors.New("template CSS is required")
	}
	return nil
}

// IsDeleted checks if template is soft-deleted
func (t *Template) IsDeleted() bool {
	return t.DeletedAt != nil
}

// CanBeUsed checks if template can be used for generation
func (t *Template) CanBeUsed() bool {
	return t.IsActive && !t.IsDeleted()
}

// SizeInBytes returns total size of HTML + CSS
func (t *Template) SizeInBytes() int {
	return len(t.Html) + len(t.Css)
}

// ExceedsSizeLimit checks if template exceeds individual size limit
func (t *Template) ExceedsSizeLimit() bool {
	return t.SizeInBytes() > MaxTemplateSizeBytes
}
