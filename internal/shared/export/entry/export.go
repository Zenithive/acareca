package entry

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"github.com/iamarpitzala/acareca/internal/shared/export"
	lo "github.com/samber/lo"
	"github.com/xuri/excelize/v2"
)

// CoaGroup represents a Chart of Accounts grouping
type CoaGroup struct {
	CoaID            string       `json:"coa_id"`
	CoaName          string       `json:"coa_name"`
	TotalNetAmount   float64      `json:"total_net_amount"`
	TotalGrossAmount float64      `json:"total_gross_amount"`
	Details          []*CoaDetail `json:"details"`
}

// CoaDetail represents a transaction detail within a COA group
type CoaDetail struct {
	FormFieldName string    `json:"form_field_name"`
	TaxTypeName   *string   `json:"tax_type_name"`
	FormName      string    `json:"form_name"`
	ClinicName    string    `json:"clinic_name"`
	NetAmount     *float64  `json:"net_amount"`
	GstAmount     *float64  `json:"gst_amount"`
	GrossAmount   *float64  `json:"gross_amount"`
	CreatedAt     time.Time `json:"created_at"`
	IsExpense     bool      `json:"is_expense"`
}

// GenerateExcelReport generates an Excel file for transaction report
func GenerateExcelReport(groups []*CoaGroup, config export.ExportConfig, formatDateFn func(string) string) (*bytes.Buffer, error) {
	xl := excelize.NewFile()
	defer xl.Close()
	sheet := "Transactions"
	xl.SetSheetName("Sheet1", sheet)

	// Define the width of the report (Columns A through I)
	lastCol := "I"

	// Create custom style for transaction report headers
	styleHeaderBlue, _ := xl.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14, Color: "FFFFFF"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#4EA7B3"}, Pattern: 1},
	})

	headerStyle, _ := xl.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#4EA7B3"}, Pattern: 1},
	})
	groupHeaderStyle, _ := xl.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#DAEEF3"}, Pattern: 1},
	})

	normalCurrencyStyle, _ := xl.NewStyle(&excelize.Style{
		CustomNumFmt: lo.ToPtr("$#,##0.00"),
	})

	// Bold style for the bottom total row
	totalRowStyle, _ := xl.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E1E1E1"}, Pattern: 1},
	})
	totalCurrencyStyle, _ := xl.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"#F2F2F2"}, Pattern: 1},
		CustomNumFmt: lo.ToPtr("$#,##0.00"),
	})

	// Helpers to handle Pointers and Nils
	getFloat := func(f *float64) float64 {
		return export.GetFloatValue(f)
	}
	getString := func(s *string) string {
		return export.GetStringValue(s)
	}

	// --- 1. RENDER METADATA ---
	// Row 1: Title
	xl.MergeCell(sheet, "A1", lastCol+"1")
	xl.SetCellValue(sheet, "A1", "Transaction Report")
	xl.SetCellStyle(sheet, "A1", "A1", styleHeaderBlue)

	setRichMeta := func(row int, label, value string) {
		cell := fmt.Sprintf("A%d", row)
		xl.MergeCell(sheet, cell, lastCol+strconv.Itoa(row))
		xl.SetCellRichText(sheet, cell, []excelize.RichTextRun{
			{Text: label, Font: &excelize.Font{Bold: true, Family: "Calibri", Size: 10}},
			{Text: " " + value, Font: &excelize.Font{Bold: false, Family: "Calibri", Size: 10}},
		})
	}

	metaRow := 2

	// Exported By (Always show)
	setRichMeta(metaRow, "Exported by:", config.EntityName)
	metaRow++

	// ABN (Skip if empty)
	if config.EntityABN != "" {
		setRichMeta(metaRow, "ABN:", config.EntityABN)
		metaRow++
	}

	// Period (Skip if nil/empty)
	if config.Period != "" {
		setRichMeta(metaRow, "Period:", config.Period)
		metaRow++
	}

	// Generated time
	if config.GeneratedTime == "" {
		config.GeneratedTime = time.Now().Format("02/01/2006, 3:04:05 pm")
	}
	setRichMeta(metaRow, "Generated:", config.GeneratedTime)
	metaRow++

	// 2. Set Headers
	headerRow := metaRow + 1
	headers := []string{"Date", "Account / Field", "Tax Type", "Form", "Clinic", "Net Amount", "GST Amount", "Gross Amount", "Type"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, headerRow)
		xl.SetCellValue(sheet, cell, h)
	}
	xl.SetCellStyle(sheet, fmt.Sprintf("A%d", headerRow), fmt.Sprintf("I%d", headerRow), headerStyle)

	currRow := headerRow + 1
	for _, g := range groups {
		// --- 3. GROUP HEADER ---
		xl.SetCellValue(sheet, fmt.Sprintf("A%d", currRow), g.CoaName)
		xl.MergeCell(sheet, fmt.Sprintf("A%d", currRow), fmt.Sprintf("I%d", currRow))
		xl.SetCellStyle(sheet, fmt.Sprintf("A%d", currRow), fmt.Sprintf("I%d", currRow), groupHeaderStyle)
		currRow++

		// --- 4. INDIVIDUAL TRANSACTIONS ---
		for _, d := range g.Details {
			xl.SetCellValue(sheet, fmt.Sprintf("A%d", currRow), formatDateFn(d.CreatedAt.Format("2006-01-02")))
			xl.SetCellValue(sheet, fmt.Sprintf("B%d", currRow), "  "+d.FormFieldName)
			xl.SetCellValue(sheet, fmt.Sprintf("C%d", currRow), getString(d.TaxTypeName))
			xl.SetCellValue(sheet, fmt.Sprintf("D%d", currRow), d.FormName)
			xl.SetCellValue(sheet, fmt.Sprintf("E%d", currRow), d.ClinicName)

			xl.SetCellValue(sheet, fmt.Sprintf("F%d", currRow), getFloat(d.NetAmount))
			xl.SetCellValue(sheet, fmt.Sprintf("G%d", currRow), getFloat(d.GstAmount))
			xl.SetCellValue(sheet, fmt.Sprintf("H%d", currRow), getFloat(d.GrossAmount))

			// Apply currency formatting to F, G, H columns
			xl.SetCellStyle(sheet, fmt.Sprintf("F%d", currRow), fmt.Sprintf("H%d", currRow), normalCurrencyStyle)

			entryType := "Entry"
			if d.IsExpense {
				entryType = "Expense"
			}
			xl.SetCellValue(sheet, fmt.Sprintf("I%d", currRow), entryType)
			currRow++
		}

		// --- 5. TOTAL ROW ---
		xl.SetCellValue(sheet, fmt.Sprintf("A%d", currRow), "Total "+g.CoaName)
		xl.SetCellValue(sheet, fmt.Sprintf("F%d", currRow), g.TotalNetAmount)
		xl.SetCellValue(sheet, fmt.Sprintf("H%d", currRow), g.TotalGrossAmount)

		xl.SetCellStyle(sheet, fmt.Sprintf("A%d", currRow), fmt.Sprintf("I%d", currRow), totalRowStyle)
		xl.SetCellStyle(sheet, fmt.Sprintf("F%d", currRow), fmt.Sprintf("F%d", currRow), totalCurrencyStyle)
		xl.SetCellStyle(sheet, fmt.Sprintf("H%d", currRow), fmt.Sprintf("H%d", currRow), totalCurrencyStyle)

		currRow += 2 // Gap between groups
	}

	// Add AutoFilter to the header row (A to I)
	if err := xl.AutoFilter(sheet, fmt.Sprintf("A%d:I%d", headerRow, headerRow), nil); err != nil {
		return nil, err
	}

	// Column Widths
	xl.SetColWidth(sheet, "A", "A", 15) // Date
	xl.SetColWidth(sheet, "B", "B", 35) // Account
	xl.SetColWidth(sheet, "C", "E", 20) // Tax, Form, Clinic
	xl.SetColWidth(sheet, "F", "H", 15) // Amounts
	xl.SetColWidth(sheet, "I", "I", 12) // Type

	return xl.WriteToBuffer()
}
