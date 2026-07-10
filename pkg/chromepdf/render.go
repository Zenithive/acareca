package chromepdf

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/aymerick/raymond"
)

// coalesceRe matches {{coalesce arg1 arg2 ...}} with any number of args.
// Args can be dot-path identifiers (e.g. template_settings.primary_color)
// or double-quoted string literals (e.g. "#1f4e5f").
var coalesceRe = regexp.MustCompile(`\{\{coalesce\s+((?:[^\s\}][^\}]*?)?)\}\}`)

// coalesceArgRe splits individual arguments — either "quoted string" or bare.path.identifier
var coalesceArgRe = regexp.MustCompile(`"([^"]*)"|([^\s"]+)`)

// resolveCoalesce replaces every {{coalesce ...}} expression in src with its
// resolved value looked up from data. This runs before raymond so that raymond
// never sees a multi-argument coalesce call (raymond v2 does not support
// variadic helper functions).
func resolveCoalesce(src string, data map[string]any) string {
	return coalesceRe.ReplaceAllStringFunc(src, func(match string) string {
		inner := coalesceRe.FindStringSubmatch(match)
		if len(inner) < 2 {
			return match
		}

		for _, m := range coalesceArgRe.FindAllStringSubmatch(inner[1], -1) {
			// m[1] is a quoted string literal, m[2] is a bare identifier
			if m[1] != "" {
				// string literal — use it directly as the fallback
				return m[1]
			}
			// dot-path lookup against data map
			val := lookupPath(data, m[2])
			if val != "" {
				return val
			}
		}
		return ""
	})
}

// lookupPath resolves a dot-separated key path (e.g. "template_settings.primary_color")
// against a nested map[string]any, returning the string value or "".
func lookupPath(data map[string]any, path string) string {
	parts := strings.Split(path, ".")
	var cur any = data
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[p]
	}
	if cur == nil {
		return ""
	}
	switch v := cur.(type) {
	case string:
		return v
	case *string:
		if v == nil {
			return ""
		}
		return *v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// Render resolves all {{variable}} and {{#if}} / {{#each}} blocks in the
// template HTML/CSS using the raymond Handlebars engine, then injects the
// CSS into the HTML as a <style> block.
func Render(html, css string, data map[string]any) (string, error) {
	// Pre-resolve {{coalesce ...}} before raymond sees them — raymond v2 does
	// not support variadic helper functions and will error on multi-arg calls.
	css = resolveCoalesce(css, data)
	html = resolveCoalesce(html, data)

	renderedCSS, err := raymond.Render(css, data)
	if err != nil {
		return "", fmt.Errorf("chromepdf: css render failed: %w", err)
	}

	renderedHTML, err := raymond.Render(html, data)
	if err != nil {
		return "", fmt.Errorf("chromepdf: html render failed: %w", err)
	}

	// Dynamic external resource pre-connect setup
	var fontLinks strings.Builder
	if ts, ok := data["template_settings"].(map[string]any); ok {
		fontLinks.WriteString(`<link rel="preconnect" href="https://fonts.googleapis.com">`)
		fontLinks.WriteString(`<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>`)

		if headerFont, _ := ts["header_font_family_css"].(string); headerFont != "" {
			fontLinks.WriteString(fmt.Sprintf(`<link href="https://fonts.googleapis.com/css2?family=%s:wght@400;700&display=swap" rel="stylesheet">`, headerFont))
		}
		if bodyFont, _ := ts["body_font_family_css"].(string); bodyFont != "" {
			fontLinks.WriteString(fmt.Sprintf(`<link href="https://fonts.googleapis.com/css2?family=%s:wght@400;700&display=swap" rel="stylesheet">`, bodyFont))
		}
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

func toFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}

func init() {
	// coalesce is now handled by the pre-processor above; this single-arg
	// registration is kept so any stray {{coalesce x}} in old stored templates
	// still resolves cleanly without erroring.
	raymond.RegisterHelper("coalesce", func(val interface{}) raymond.SafeString {
		if val == nil {
			return raymond.SafeString("")
		}
		switch v := val.(type) {
		case string:
			return raymond.SafeString(v)
		case *string:
			if v == nil {
				return raymond.SafeString("")
			}
			return raymond.SafeString(*v)
		default:
			return raymond.SafeString(fmt.Sprintf("%v", v))
		}
	})

	raymond.RegisterHelper("format_currency", func(amount float64) raymond.SafeString {
		return raymond.SafeString(fmt.Sprintf("$%.2f", math.Abs(amount)))
	})

	raymond.RegisterHelper("format_table_amount", func(row any) raymond.SafeString {
		m, ok := row.(map[string]any)
		if !ok {
			if m2, ok2 := row.(map[string]interface{}); ok2 {
				m = m2
			} else {
				return raymond.SafeString("")
			}
		}
		amount := toFloat64(m["amount"])
		formatted := fmt.Sprintf("$%.2f", math.Abs(amount))
		if neg, _ := m["is_negative"].(bool); neg {
			return raymond.SafeString(fmt.Sprintf("(%s)", formatted))
		}
		return raymond.SafeString(formatted)
	})
}
