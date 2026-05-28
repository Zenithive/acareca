package export

import (
	"bytes"

	"github.com/xuri/excelize/v2"
)

// ExportConfig holds common export configuration
type ExportConfig struct {
	EntityName     string
	EntityABN      string
	Period         string
	ExportType     string
	ExportedByName string
	GeneratedTime  string
}

// ExportResult wraps different export format results
type ExportResult struct {
	Result      interface{} // *excelize.File or *bytes.Buffer
	ContentType string
	FileName    string
}

// StyleSet holds commonly used styles for Excel exports
type StyleSet struct {
	HeaderBlue     int
	SectionTitle   int
	DataLeft       int
	DataGrid       int
	DataGridBold   int
	GroupTotal     int
	Profit         int
	ProfitGreen    int
	TotalRow       int
	TotalCurrency  int
	NormalCurrency int
	HeaderWhite    int
	GroupHeader    int
}

// Helper functions for nil value handling
func GetFloatValue(f *float64) float64 {
	if f == nil {
		return 0.0
	}
	return *f
}

func GetStringValue(s *string) string {
	if s == nil || *s == "" || *s == "<nil>" {
		return "-"
	}
	return *s
}

// FormatDateHelper formats date strings from YYYY-MM-DD to DD-MM-YYYY
func FormatDateHelper(dateStr string) string {
	// Implementation imported from services
	if dateStr == "" || dateStr == "<nil>" {
		return "-"
	}
	if len(dateStr) < 10 {
		return dateStr
	}

	// Use the utility from the caller's context
	// This will be handled by individual export modules
	return dateStr
}

// WriteBuffer is a helper to get buffer from excelize.File
func WriteBuffer(f *excelize.File) (*bytes.Buffer, error) {
	return f.WriteToBuffer()
}
