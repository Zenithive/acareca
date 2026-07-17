package test_scripts

import (
	"strings"
	"testing"

	"github.com/iamarpitzala/acareca/pkg/chromepdf"
)

func TestCoalescePreProcessor(t *testing.T) {
	tests := []struct {
		name     string
		tmpl     string
		data     map[string]any
		contains string
	}{
		{
			name:     "returns first non-empty variable",
			tmpl:     `{{coalesce a b}}`,
			data:     map[string]any{"a": "hello", "b": "world"},
			contains: "hello",
		},
		{
			name:     "falls back to second variable",
			tmpl:     `{{coalesce a b}}`,
			data:     map[string]any{"a": "", "b": "world"},
			contains: "world",
		},
		{
			name:     "falls back to string literal",
			tmpl:     `{{coalesce a "#1f4e5f"}}`,
			data:     map[string]any{"a": ""},
			contains: "#1f4e5f",
		},
		{
			name:     "returns variable over string literal",
			tmpl:     `{{coalesce a "#1f4e5f"}}`,
			data:     map[string]any{"a": "#ff0000"},
			contains: "#ff0000",
		},
		{
			name:     "three args with literal fallback",
			tmpl:     `{{coalesce a b "Arial"}}`,
			data:     map[string]any{"a": "", "b": ""},
			contains: "Arial",
		},
		{
			name:     "CSS pattern: primary color fallback",
			tmpl:     `--primary-color: {{coalesce template_settings.primary_color "#1f4e5f"}};`,
			data:     map[string]any{"template_settings": map[string]any{"primary_color": ""}},
			contains: "#1f4e5f",
		},
		{
			name:     "CSS pattern: primary color set",
			tmpl:     `--primary-color: {{coalesce template_settings.primary_color "#1f4e5f"}};`,
			data:     map[string]any{"template_settings": map[string]any{"primary_color": "#abc123"}},
			contains: "#abc123",
		},
		{
			name:     "watermark text fallback",
			tmpl:     `{{coalesce template_settings.watermark_text "PAID"}}`,
			data:     map[string]any{"template_settings": map[string]any{"watermark_text": ""}},
			contains: "PAID",
		},
		{
			name:     "three-arg font family fallback",
			tmpl:     `'{{coalesce template_settings.header_font_family_css template_settings.header_font_family "Arial"}}'`,
			data:     map[string]any{"template_settings": map[string]any{"header_font_family_css": "", "header_font_family": ""}},
			contains: "Arial",
		},
		{
			name:     "three-arg font family middle value wins",
			tmpl:     `'{{coalesce template_settings.header_font_family_css template_settings.header_font_family "Arial"}}'`,
			data:     map[string]any{"template_settings": map[string]any{"header_font_family_css": "", "header_font_family": "Roboto"}},
			contains: "Roboto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := chromepdf.Render(tt.tmpl, "", tt.data)
			if err != nil {
				t.Fatalf("Render error: %v", err)
			}
			if !strings.Contains(result, tt.contains) {
				t.Errorf("expected output to contain %q\ngot: %s", tt.contains, result)
			}
		})
	}
}
