package chromepdf

import (
	"fmt"
	"math"

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
	raymond.RegisterHelper("coalesce", func(args ...interface{}) raymond.SafeString {
		// Last argument is always the Handlebars options object, skip it
		values := args[:len(args)-1]
		for _, v := range values {
			if v != nil {
				switch val := v.(type) {
				case string:
					if val != "" {
						return raymond.SafeString(val)
					}
				case *string:
					if val != nil && *val != "" {
						return raymond.SafeString(*val)
					}
				default:
					// For non-string types, return if not nil
					return raymond.SafeString(fmt.Sprintf("%v", val))
				}
			}
		}
		return raymond.SafeString("")
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
