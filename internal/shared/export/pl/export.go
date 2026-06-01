package pl

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/iamarpitzala/acareca/internal/shared/export"
	lo "github.com/samber/lo"
	"github.com/xuri/excelize/v2"
)

// RsReportAccount represents an account in a P&L report section
type RsReportAccount struct {
	CoaID      string  `json:"coa_id"`
	CoaName    string  `json:"coa_name"`
	TotalValue float64 `json:"total_value"`
}

// RsReportGroup represents a group of accounts (Income, Expenses, etc.)
type RsReportGroup struct {
	GroupTotal float64           `json:"group_total"`
	Accounts   []RsReportAccount `json:"accounts"`
}

// RsReportMetadata holds metadata about the P&L report
type RsReportMetadata struct {
	DateFrom         string  `json:"date_from"`
	DateUntil        string  `json:"date_until"`
	OverallNetProfit float64 `json:"overall_net_profit"`
}

// RsReport is the complete P&L report
type RsReport struct {
	ReportMetadata RsReportMetadata `json:"report_metadata"`
	Income         RsReportGroup    `json:"income"`
	CostOfSales    RsReportGroup    `json:"cost_of_sales"`
	GrossProfit    float64          `json:"gross_profit"`
	OtherCosts     RsReportGroup    `json:"other_costs"`
	NetProfit      float64          `json:"net_profit"`
}

// Round2 rounds a float value to 2 decimal places
func Round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// GenerateExcelReport generates an Excel file for P&L report
func GenerateExcelReport(data *RsReport, config export.ExportConfig) (*excelize.File, error) {
	f := excelize.NewFile()
	sheet := "Profit and Loss"
	f.NewSheet(sheet)
	f.DeleteSheet("Sheet1")

	styles := applyReportStyles(f)

	// Helper function for rich text metadata
	setMetaRow := func(cell string, label string, value string) {
		f.SetCellRichText(sheet, cell, []excelize.RichTextRun{
			{Text: label, Font: &excelize.Font{Bold: true, Family: "Calibri", Size: 10}},
			{Text: " " + value, Font: &excelize.Font{Bold: false, Family: "Calibri", Size: 10}},
		})
	}

	// Title
	f.SetCellValue(sheet, "A1", "Profit and Loss Report")
	f.MergeCell(sheet, "A1", "B1")
	f.SetCellStyle(sheet, "A1", "B1", styles.HeaderBlue)

	currentRow := 2

	// Metadata rows
	setMetaRow(fmt.Sprintf("A%d", currentRow), "Exported by:", config.EntityName)
	currentRow++

	if config.EntityABN != "" {
		setMetaRow(fmt.Sprintf("A%d", currentRow), "ABN:", config.EntityABN)
		currentRow++
	}

	if data.ReportMetadata.DateFrom != "" && data.ReportMetadata.DateUntil != "" {
		setMetaRow(fmt.Sprintf("A%d", currentRow), "Period:",
			fmt.Sprintf("%s to %s", FormatDateStr(data.ReportMetadata.DateFrom), FormatDateStr(data.ReportMetadata.DateUntil)))
		currentRow++
	}

	currentTimeStr := time.Now().Format("02/01/2006, 3:04:05 pm")
	setMetaRow(fmt.Sprintf("A%d", currentRow), "Generated:", currentTimeStr)
	currentRow++
	currentRow++

	var totalIncomeCell, totalCOSCell, totalOtherCostsCell string

	// Helper function to render a report section
	renderGroup := func(title string, group RsReportGroup) string {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), title)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.SectionTitle)

		if len(group.Accounts) > 0 {
			tableRange := fmt.Sprintf("A%d:A%d", currentRow, currentRow+len(group.Accounts))
			tableName := strings.ReplaceAll(title, " ", "_") + fmt.Sprintf("_%d", currentRow)
			showHeaders := true
			f.AddTable(sheet, &excelize.Table{
				Range:         tableRange,
				Name:          tableName,
				StyleName:     "",
				ShowHeaderRow: &showHeaders,
			})
		}

		currentRow++
		dataStartRow := currentRow
		for _, acc := range group.Accounts {
			f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), acc.CoaName)
			f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.DataLeft)

			f.SetCellValue(sheet, fmt.Sprintf("B%d", currentRow), acc.TotalValue)
			f.SetCellStyle(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("B%d", currentRow), styles.DataGrid)
			currentRow++
		}
		dataEndRow := currentRow - 1

		totalCell := fmt.Sprintf("B%d", currentRow)
		f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "TOTAL "+title)

		if len(group.Accounts) > 0 {
			formula := fmt.Sprintf("SUBTOTAL(109, B%d:B%d)", dataStartRow, dataEndRow)
			f.SetCellFormula(sheet, totalCell, formula)
		} else {
			f.SetCellValue(sheet, totalCell, 0)
		}

		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("B%d", currentRow), styles.GroupTotal)
		currentRow += 2

		return totalCell
	}

	totalIncomeCell = renderGroup("INCOME", data.Income)
	totalCOSCell = renderGroup("COST OF SALES", data.CostOfSales)

	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "GROSS PROFIT")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.Profit)

	f.SetCellFormula(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("%s-%s", totalIncomeCell, totalCOSCell))
	f.SetCellStyle(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("B%d", currentRow), styles.ProfitGreen)
	grossProfitCell := fmt.Sprintf("B%d", currentRow)
	currentRow += 2

	totalOtherCostsCell = renderGroup("OTHER COSTS", data.OtherCosts)

	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "NET PROFIT")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.Profit)

	f.SetCellFormula(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("%s-%s", grossProfitCell, totalOtherCostsCell))
	f.SetCellStyle(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("B%d", currentRow), styles.ProfitGreen)

	f.SetColWidth(sheet, "A", "A", 45)
	f.SetColWidth(sheet, "B", "B", 20)
	f.UpdateLinkedValue()

	return f, nil
}

// applyReportStyles creates and returns styled elements for P&L reports
func applyReportStyles(f *excelize.File) StyleSet {
	hBlue, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Family: "Calibri", Size: 14},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#DAEEF3"}, Pattern: 1},
	})
	sTitle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Family: "Calibri", Size: 12},
	})
	dLeft, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Family: "Calibri", Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "left"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})
	dGrid, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Family: "Calibri", Size: 10},
		CustomNumFmt: lo.ToPtr("$#,##0.00;$#,##0.00;$0.00"),
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})
	gTotal, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Family: "Calibri"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"#DAEEF3"}, Pattern: 1},
		CustomNumFmt: lo.ToPtr("$#,##0.00;$#,##0.00;$0.00"),
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})
	profit, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Family: "Calibri"},
		CustomNumFmt: lo.ToPtr("$#,##0.00;$#,##0.00;$0.00"),
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"#c4f0ce"}, Pattern: 1},
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 2}, {Type: "top", Color: "000000", Style: 2},
			{Type: "bottom", Color: "000000", Style: 2}, {Type: "right", Color: "000000", Style: 2},
		},
	})
	profitGreen, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Family: "Calibri", Color: "28a745"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"#c4f0ce"}, Pattern: 1},
		CustomNumFmt: lo.ToPtr("$#,##0.00;$#,##0.00;$0.00"),
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 2}, {Type: "top", Color: "000000", Style: 2},
			{Type: "bottom", Color: "000000", Style: 2}, {Type: "right", Color: "000000", Style: 2},
		},
	})

	return StyleSet{
		HeaderBlue:   hBlue,
		SectionTitle: sTitle,
		DataLeft:     dLeft,
		DataGrid:     dGrid,
		GroupTotal:   gTotal,
		Profit:       profit,
		ProfitGreen:  profitGreen,
	}
}

// StyleSet holds styles for P&L reports
type StyleSet struct {
	HeaderBlue   int
	SectionTitle int
	DataLeft     int
	DataGrid     int
	GroupTotal   int
	Profit       int
	ProfitGreen  int
}

// FormatDateStr formats a date string from YYYY-MM-DD to DD-MM-YYYY
func FormatDateStr(dateStr string) string {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return dateStr
	}
	return t.Format("02-01-2006")
}
