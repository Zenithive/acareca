package rendering

import (
	"context"
	"fmt"
)

type IPDFRenderer interface {
	RenderToPDF(ctx context.Context, html string) ([]byte, error)
}

type ITemplateRenderer interface {
	RenderHTML(html, css string, data map[string]interface{}) (string, error)
	RenderMultipleTemplates(templates []TemplateContent, data map[string]interface{}) (string, error)
}

type TemplateContent struct {
	Order int
	HTML  string
	CSS   string
}

type SizeGuard struct {
	currentSize int
	maxSize     int
}

func NewSizeGuard(maxSize int) *SizeGuard {
	return &SizeGuard{
		currentSize: 0,
		maxSize:     maxSize,
	}
}

func (g *SizeGuard) CheckAndAdd(size int) error {
	if g.currentSize+size > g.maxSize {
		return fmt.Errorf("size limit exceeded: current=%d + new=%d > max=%d",
			g.currentSize, size, g.maxSize)
	}
	g.currentSize += size
	return nil
}

func (g *SizeGuard) CurrentSize() int {
	return g.currentSize
}

func (g *SizeGuard) Reset() {
	g.currentSize = 0
}

func ValidateTemplateSize(html, css string) error {
	size := len(html) + len(css)
	// if size > template.MaxTemplateSizeBytes {
	// 	return fmt.Errorf("template size %d exceeds limit %d", size, template.MaxTemplateSizeBytes)
	// }

	if size > 1024*1024 { // 1 MB limit for example
		return fmt.Errorf("template size %d exceeds limit %d", size, 1024*1024)
	}

	return nil
}
