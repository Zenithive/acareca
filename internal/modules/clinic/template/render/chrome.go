package render

import (
	"context"
	"fmt"

	"github.com/iamarpitzala/acareca/pkg/chromepdf"
)

// ChromeRenderer implements IPDFRenderer using Chrome/Chromium for PDF generation
type ChromeRenderer struct{}

// NewChromeRenderer creates a new ChromeRenderer instance
func NewChromeRenderer() *ChromeRenderer {
	return &ChromeRenderer{}
}

// RenderToPDF converts HTML to PDF using Chrome headless
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
