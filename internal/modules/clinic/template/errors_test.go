package template_test

import (
	"errors"
	"testing"

	"github.com/iamarpitzala/acareca/internal/modules/clinic/template"
)

func TestErrorDefinitions(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"NotFound", template.ErrNotFound},
		{"InvalidTemplate", template.ErrInvalidTemplate},
		{"TemplateRequired", template.ErrTemplateRequired},
		{"TooManyTemplates", template.ErrTooManyTemplates},
		{"TemplateSizeExceeded", template.ErrTemplateSizeExceeded},
		{"SettingNotFound", template.ErrSettingNotFound},
		{"InvalidSetting", template.ErrInvalidSetting},
		{"InvoiceNotFound", template.ErrInvoiceNotFound},
		{"Unauthorized", template.ErrUnauthorized},
		{"InvalidEncryptionKey", template.ErrInvalidEncryptionKey},
		{"TemplateTooLarge", template.ErrTemplateTooLarge},
		{"TotalSizeTooLarge", template.ErrTotalSizeTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Error("error should not be nil")
			}
			if tt.err.Error() == "" {
				t.Error("error message should not be empty")
			}
			// Test errors.Is works
			if !errors.Is(tt.err, tt.err) {
				t.Error("errors.Is should work for same error")
			}
		})
	}
}

func TestErrorMessages(t *testing.T) {
	tests := []struct {
		err     error
		wantMsg string
	}{
		{template.ErrNotFound, "template not found"},
		{template.ErrUnauthorized, "unauthorized access"},
		{template.ErrInvalidEncryptionKey, "encryption key must be exactly 32 characters"},
	}

	for _, tt := range tests {
		t.Run(tt.wantMsg, func(t *testing.T) {
			if tt.err.Error() != tt.wantMsg {
				t.Errorf("error message = %q, want %q", tt.err.Error(), tt.wantMsg)
			}
		})
	}
}
