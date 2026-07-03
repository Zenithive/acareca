package rendering

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aymerick/raymond"
)

type TemplateBuilder struct {
	sizeGuard *SizeGuard
}

func NewTemplateBuilder() *TemplateBuilder {
	return &TemplateBuilder{
		// sizeGuard: NewSizeGuard(template.MaxTotalSizeBytes),
		sizeGuard: NewSizeGuard(1024 * 1024), // 1 MB limit for the entire document
	}
}

func NewTemplateBuilderWithLimit(maxSize int) *TemplateBuilder {
	return &TemplateBuilder{
		sizeGuard: NewSizeGuard(maxSize),
	}
}

func (b *TemplateBuilder) BuildDocument(templates []TemplateContent, data map[string]interface{}) (string, error) {
	if len(templates) == 0 {
		return "", fmt.Errorf("no templates provided")
	}

	// Sort by order
	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Order < templates[j].Order
	})

	var htmlBuilder, cssBuilder strings.Builder

	for _, tmpl := range templates {
		// Check size before rendering
		estimatedSize := len(tmpl.HTML) + len(tmpl.CSS)
		if err := b.sizeGuard.CheckAndAdd(estimatedSize); err != nil {
			return "", fmt.Errorf("template size limit exceeded: %w", err)
		}

		// Render HTML
		renderedHTML, err := raymond.Render(tmpl.HTML, data)
		if err != nil {
			return "", fmt.Errorf("failed to render HTML: %w", err)
		}
		htmlBuilder.WriteString(renderedHTML)
		htmlBuilder.WriteString("\n")

		// Render CSS
		renderedCSS, err := raymond.Render(tmpl.CSS, data)
		if err != nil {
			return "", fmt.Errorf("failed to render CSS: %w", err)
		}
		cssBuilder.WriteString(renderedCSS)
		cssBuilder.WriteString("\n")
	}

	// Build final document
	document := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<style>
%s
</style>
</head>
<body>
%s
</body>
</html>`, cssBuilder.String(), htmlBuilder.String())

	return document, nil
}

func (b *TemplateBuilder) BuildSimpleDocument(html, css string) (string, error) {
	size := len(html) + len(css)
	if err := b.sizeGuard.CheckAndAdd(size); err != nil {
		return "", fmt.Errorf("document size limit exceeded: %w", err)
	}

	document := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<style>
%s
</style>
</head>
<body>
%s
</body>
</html>`, css, html)

	return document, nil
}

func (b *TemplateBuilder) Reset() {
	b.sizeGuard.Reset()
}

func (b *TemplateBuilder) CurrentSize() int {
	return b.sizeGuard.CurrentSize()
}
