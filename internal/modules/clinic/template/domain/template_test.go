package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template/domain"
)

func TestTemplate_Validate(t *testing.T) {
	tests := []struct {
		name    string
		tmpl    domain.Template
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid template",
			tmpl: domain.Template{
				Name: "Test Template",
				Html: []byte("<html>test</html>"),
				Css:  []byte("body { margin: 0; }"),
			},
			wantErr: false,
		},
		{
			name: "Empty name",
			tmpl: domain.Template{
				Name: "",
				Html: []byte("<html>test</html>"),
				Css:  []byte("body { margin: 0; }"),
			},
			wantErr: true,
			errMsg:  "template name is required",
		},
		{
			name: "Name too long",
			tmpl: domain.Template{
				Name: string(make([]byte, 101)),
				Html: []byte("<html>test</html>"),
				Css:  []byte("body { margin: 0; }"),
			},
			wantErr: true,
			errMsg:  "template name too long",
		},
		{
			name: "Empty HTML",
			tmpl: domain.Template{
				Name: "Test",
				Html: []byte{},
				Css:  []byte("body { margin: 0; }"),
			},
			wantErr: true,
			errMsg:  "template HTML is required",
		},
		{
			name: "Empty CSS",
			tmpl: domain.Template{
				Name: "Test",
				Html: []byte("<html>test</html>"),
				Css:  []byte{},
			},
			wantErr: true,
			errMsg:  "template CSS is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tmpl.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("error message = %q, want %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestTemplate_IsDeleted(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		tmpl    domain.Template
		want    bool
	}{
		{
			name: "Not deleted",
			tmpl: domain.Template{DeletedAt: nil},
			want: false,
		},
		{
			name: "Deleted",
			tmpl: domain.Template{DeletedAt: &now},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tmpl.IsDeleted(); got != tt.want {
				t.Errorf("IsDeleted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTemplate_CanBeUsed(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		tmpl domain.Template
		want bool
	}{
		{
			name: "Active and not deleted",
			tmpl: domain.Template{IsActive: true, DeletedAt: nil},
			want: true,
		},
		{
			name: "Inactive",
			tmpl: domain.Template{IsActive: false, DeletedAt: nil},
			want: false,
		},
		{
			name: "Deleted",
			tmpl: domain.Template{IsActive: true, DeletedAt: &now},
			want: false,
		},
		{
			name: "Inactive and deleted",
			tmpl: domain.Template{IsActive: false, DeletedAt: &now},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tmpl.CanBeUsed(); got != tt.want {
				t.Errorf("CanBeUsed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTemplate_SizeInBytes(t *testing.T) {
	tests := []struct {
		name string
		tmpl domain.Template
		want int
	}{
		{
			name: "Small template",
			tmpl: domain.Template{
				Html: []byte("12345"),
				Css:  []byte("123"),
			},
			want: 8,
		},
		{
			name: "Empty template",
			tmpl: domain.Template{
				Html: []byte{},
				Css:  []byte{},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tmpl.SizeInBytes(); got != tt.want {
				t.Errorf("SizeInBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTemplate_ExceedsSizeLimit(t *testing.T) {
	tests := []struct {
		name string
		tmpl domain.Template
		want bool
	}{
		{
			name: "Within limit",
			tmpl: domain.Template{
				Html: make([]byte, 1024), // 1KB
				Css:  make([]byte, 1024), // 1KB
			},
			want: false,
		},
		{
			name: "Exceeds limit",
			tmpl: domain.Template{
				Html: make([]byte, 6*1024*1024), // 6MB
				Css:  make([]byte, 1024),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tmpl.ExceedsSizeLimit(); got != tt.want {
				t.Errorf("ExceedsSizeLimit() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTemplate_Integration(t *testing.T) {
	// Test a complete template lifecycle
	tmpl := domain.Template{
		ID:          uuid.New(),
		Name:        "Invoice Template",
		Description: stringPtr("Test template"),
		Html:        []byte("<html><body>{{invoice_number}}</body></html>"),
		Css:         []byte("body { font-family: Arial; }"),
		IsDefault:   true,
		IsActive:    true,
		CreatedAt:   time.Now(),
	}

	// Should validate
	if err := tmpl.Validate(); err != nil {
		t.Errorf("valid template should not error: %v", err)
	}

	// Should be usable
	if !tmpl.CanBeUsed() {
		t.Error("active non-deleted template should be usable")
	}

	// Should not exceed size limit
	if tmpl.ExceedsSizeLimit() {
		t.Error("small template should not exceed limit")
	}

	// Should not be deleted
	if tmpl.IsDeleted() {
		t.Error("new template should not be deleted")
	}
}

func stringPtr(s string) *string {
	return &s
}
