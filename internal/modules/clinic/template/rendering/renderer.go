package rendering

import (
	"context"
	"fmt"
)

// IPDFRenderer defines the interface for PDF generation
type IPDFRenderer interface {
	RenderToPDF(ctx context.Context, html string) ([]byte, error)
}

// TemplateContent represents a single template with its order
type TemplateContent struct {
	Order int
	HTML  string
	CSS   string
}

// ValidateTemplateSize checks if the template size is within acceptable limits
func ValidateTemplateSize(html, css string) error {
	const maxSize = 1024 * 1024 // 1 MB limit
	size := len(html) + len(css)
	
	if size > maxSize {
		return fmt.Errorf("template size %d exceeds limit %d", size, maxSize)
	}
	return nil
}
