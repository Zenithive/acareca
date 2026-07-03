package rendering

import (
	"context"
	"fmt"

	"github.com/iamarpitzala/acareca/pkg/chromepdf"
)

type ChromeRenderer struct{}

func NewChromeRenderer() *ChromeRenderer {
	return &ChromeRenderer{}
}

func (r *ChromeRenderer) RenderToPDF(ctx context.Context, html string) ([]byte, error) {
	if html == "" {
		return nil, fmt.Errorf("html content is empty")
	}

	pdf, err := chromepdf.Generate(ctx, html)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}

	return pdf, nil
}

// RenderWithOptions allows custom PDF generation options (future extension)
func (r *ChromeRenderer) RenderWithOptions(ctx context.Context, html string, opts map[string]interface{}) ([]byte, error) {
	// For now, just call the standard render
	// Can be extended in the future to support options like:
	// - Page size
	// - Margins
	// - Orientation
	// - etc.
	return r.RenderToPDF(ctx, html)
}
