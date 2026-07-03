package domain_test

import (
	"testing"

	"github.com/iamarpitzala/acareca/internal/modules/clinic/template/domain"
)

func TestGetMethod(t *testing.T) {
	tests := []struct {
		name       string
		methodName string
		wantOk     bool
		wantCount  int
	}{
		{
			name:       "Valid SFA Clinic Collects",
			methodName: "SFA_CLINIC_COLLECTS",
			wantOk:     true,
			wantCount:  3,
		},
		{
			name:       "Valid SFA Dentist Collects",
			methodName: "SFA_DENTIST_COLLECTS",
			wantOk:     true,
			wantCount:  2,
		},
		{
			name:       "Valid Independent Contractor",
			methodName: "INDEPENDENT_CONTRACTOR",
			wantOk:     true,
			wantCount:  3,
		},
		{
			name:       "Invalid Method",
			methodName: "INVALID_METHOD",
			wantOk:     false,
			wantCount:  0,
		},
		{
			name:       "Empty Method",
			methodName: "",
			wantOk:     false,
			wantCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method, ok := domain.GetMethod(tt.methodName)
			if ok != tt.wantOk {
				t.Errorf("GetMethod() ok = %v, want %v", ok, tt.wantOk)
			}
			if ok && len(method.TemplateNames) != tt.wantCount {
				t.Errorf("template count = %v, want %v",
					len(method.TemplateNames), tt.wantCount)
			}
		})
	}
}

func TestGetPageOrder(t *testing.T) {
	tests := []struct {
		name       string
		methodName string
		wantLen    int
	}{
		{"SFA Clinic", "SFA_CLINIC_COLLECTS", 3},
		{"SFA Dentist", "SFA_DENTIST_COLLECTS", 2},
		{"Independent", "INDEPENDENT_CONTRACTOR", 3},
		{"Invalid returns default", "INVALID", 3}, // Falls back to default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pageOrder := domain.GetPageOrder(tt.methodName)
			if len(pageOrder) != tt.wantLen {
				t.Errorf("GetPageOrder() len = %v, want %v",
					len(pageOrder), tt.wantLen)
			}
		})
	}
}

func TestGetTemplateNames(t *testing.T) {
	tests := []struct {
		name       string
		methodName string
		wantNil    bool
		wantLen    int
	}{
		{"Valid method", "SFA_CLINIC_COLLECTS", false, 3},
		{"Invalid method returns nil", "INVALID", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			names := domain.GetTemplateNames(tt.methodName)
			if tt.wantNil && names != nil {
				t.Error("expected nil, got slice")
			}
			if !tt.wantNil && len(names) != tt.wantLen {
				t.Errorf("len = %v, want %v", len(names), tt.wantLen)
			}
		})
	}
}

func TestAllMethods(t *testing.T) {
	methods := domain.AllMethods()
	if len(methods) != 3 {
		t.Errorf("expected 3 methods, got %d", len(methods))
	}

	// Verify all expected methods are present
	expectedMethods := map[string]bool{
		"SFA_CLINIC_COLLECTS":    false,
		"SFA_DENTIST_COLLECTS":   false,
		"INDEPENDENT_CONTRACTOR": false,
	}

	for _, method := range methods {
		if _, ok := expectedMethods[method]; ok {
			expectedMethods[method] = true
		}
	}

	for method, found := range expectedMethods {
		if !found {
			t.Errorf("expected method %s not found", method)
		}
	}
}

func TestMethodPageOrderCorrectness(t *testing.T) {
	// Verify SFA_CLINIC_COLLECTS page order
	method, _ := domain.GetMethod("SFA_CLINIC_COLLECTS")
	if method.PageOrder["Calculation Statement"] != 1 {
		t.Error("Calculation Statement should be page 1")
	}
	if method.PageOrder["Tax Invoice"] != 2 {
		t.Error("Tax Invoice should be page 2")
	}
	if method.PageOrder["Remittance Advice"] != 3 {
		t.Error("Remittance Advice should be page 3")
	}
}

func TestSizeLimitsConstants(t *testing.T) {
	if domain.MaxTemplateSizeBytes != 5*1024*1024 {
		t.Errorf("MaxTemplateSizeBytes = %d, want %d",
			domain.MaxTemplateSizeBytes, 5*1024*1024)
	}
	if domain.MaxTotalSizeBytes != 10*1024*1024 {
		t.Errorf("MaxTotalSizeBytes = %d, want %d",
			domain.MaxTotalSizeBytes, 10*1024*1024)
	}
	if domain.MaxTemplateCount != 10 {
		t.Errorf("MaxTemplateCount = %d, want %d",
			domain.MaxTemplateCount, 10)
	}
}
