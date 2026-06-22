package chromepdf

import (
	"bytes"
	"fmt"
	"html/template"
)

// Render resolves all {{.variable}} and {{if}} / {{range}} blocks in the
// template HTML/CSS using Go's html/template engine, then injects the
// CSS into the HTML as a <style> block.
func Render(html, css string, data map[string]any) (string, error) {
	// Create function map with custom helpers
	funcMap := template.FuncMap{
		"format_currency": func(amount float64) string {
			return fmt.Sprintf("$%.2f", amount)
		},
	}

	// Render CSS
	cssTmpl, err := template.New("css").Funcs(funcMap).Parse(css)
	if err != nil {
		return "", fmt.Errorf("chromepdf: css template parse failed: %w", err)
	}
	var cssBuffer bytes.Buffer
	if err := cssTmpl.Execute(&cssBuffer, data); err != nil {
		return "", fmt.Errorf("chromepdf: css render failed: %w", err)
	}
	renderedCSS := cssBuffer.String()

	// Render HTML
	htmlTmpl, err := template.New("html").Funcs(funcMap).Parse(html)
	if err != nil {
		return "", fmt.Errorf("chromepdf: html template parse failed: %w", err)
	}
	var htmlBuffer bytes.Buffer
	if err := htmlTmpl.Execute(&htmlBuffer, data); err != nil {
		return "", fmt.Errorf("chromepdf: html render failed: %w", err)
	}
	renderedHTML := htmlBuffer.String()

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
