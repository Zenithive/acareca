package rendering

import (
	"context"
	"fmt"

	"github.com/iamarpitzala/acareca/internal/modules/clinic/template/domain"
)

// IPDFRenderer defines PDF rendering interface
type IPDFRenderer interface {
	RenderToPDF(ctx context.Context, html string) ([]byte, error)
}

// ITemplateRenderer combines templates and data for rendering
type ITemplateRenderer interface {
	RenderHTML(html, css string, data map[string]interface{}) (string, error)
	RenderMultipleTemplates(templates []TemplateContent, data map[string]interface{}) (string, error)
}

// TemplateContent represents a single template's content
type TemplateContent struct {
	Order int
	HTML  string
	CSS   string
}

// SizeGuard enforces size limits during template building
type SizeGuard struct {
	currentSize int
	maxSize     int
}

// NewSizeGuard creates a new size guard with specified max size
func NewSizeGuard(maxSize int) *SizeGuard {
	return &SizeGuard{
		currentSize: 0,
		maxSize:     maxSize,
	}
}

// CheckAndAdd validates and adds size to current total
func (g *SizeGuard) CheckAndAdd(size int) error {
	if g.currentSize+size > g.maxSize {
		return fmt.Errorf("size limit exceeded: current=%d + new=%d > max=%d",
			g.currentSize, size, g.maxSize)
	}
	g.currentSize += size
	return nil
}

// CurrentSize returns the accumulated size
func (g *SizeGuard) CurrentSize() int {
	return g.currentSize
}

// Reset resets the size guard
func (g *SizeGuard) Reset() {
	g.currentSize = 0
}

// ValidateTemplateSize checks if a single template exceeds individual limit
func ValidateTemplateSize(html, css string) error {
	size := len(html) + len(css)
	if size > domain.MaxTemplateSizeBytes {
		return fmt.Errorf("template size %d exceeds limit %d", size, domain.MaxTemplateSizeBytes)
	}
	return nil
}
