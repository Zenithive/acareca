package chromepdf

import (
	"fmt"

	"github.com/aymerick/raymond"
)

// Render resolves all {{variable}} and {{#if}} / {{#each}} blocks in the
// template HTML/CSS using the raymond Handlebars engine, then injects the
// CSS into the HTML as a <style> block.
func Render(html, css string, data map[string]any) (string, error) {
	renderedCSS, err := raymond.Render(css, data)
	if err != nil {
		return "", fmt.Errorf("chromepdf: css render failed: %w", err)
	}

	renderedHTML, err := raymond.Render(html, data)
	if err != nil {
		return "", fmt.Errorf("chromepdf: html render failed: %w", err)
	}

	// Wrap in a full HTML document so Chrome has a proper DOM
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<style>%s</style>
</head>
<body>%s</body>
</html>`, renderedCSS, renderedHTML), nil
}

// formatCurrency helper — registered once, used by both templates
func init() {
	raymond.RegisterHelper("format_currency", func(amount float64) string {
		return fmt.Sprintf("$%.2f", amount)
	})
}
