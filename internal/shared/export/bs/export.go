package bs

import (
	"fmt"
	"strings"
	"time"

	"github.com/iamarpitzala/acareca/internal/shared/export"
	"github.com/xuri/excelize/v2"
)

// RsAccount represents an account in the balance sheet
type RsAccount struct {
	CoaId   interface{} `json:"coa_id"`
	Code    int16       `json:"code"`
	Name    string      `json:"name"`
	Balance float64     `json:"balance"`
}

// RsBalanceSheet represents the complete balance sheet
type RsBalanceSheet struct {
	EndDate                   string      `json:"end_date"`
	Assets                    []RsAccount `json:"assets"`
	TotalAssets               float64     `json:"total_assets"`
	Liabilities               []RsAccount `json:"liabilities"`
	TotalLiabilities          float64     `json:"total_liabilities"`
	Equity                    []RsAccount `json:"equity"`
	CurrentYearProfit         float64     `json:"current_year_profit"`
	TotalEquity               float64     `json:"total_equity"`
	TotalLiabilitiesAndEquity float64     `json:"total_liabilities_and_equity"`
}

// GenerateExcelReport generates an Excel file for balance sheet report
func GenerateExcelReport(data *RsBalanceSheet, config export.ExportConfig) (*excelize.File, error) {
	f := excelize.NewFile()
	sheet := "Balance Sheet"
	f.NewSheet(sheet)
	f.DeleteSheet("Sheet1")

	// Apply common styles
	styles := export.ApplyCommonStyles(f)

	// Create helper function for rich text metadata
	setRichMeta := func(cell string, label string, value string) {
		f.SetCellRichText(sheet, cell, []excelize.RichTextRun{
			{Text: label, Font: &excelize.Font{Bold: true, Family: "Calibri", Size: 10}},
			{Text: " " + value, Font: &excelize.Font{Bold: false, Family: "Calibri", Size: 10}},
		})
	}

	// Set title
	f.SetCellValue(sheet, "A1", "Balance Sheet")
	f.MergeCell(sheet, "A1", "B1")
	f.SetCellStyle(sheet, "A1", "B1", styles.HeaderBlue)

	// Set metadata rows
	f.MergeCell(sheet, "A2", "B2")
	setRichMeta("A2", "Exported by:", config.EntityName)

	f.MergeCell(sheet, "A3", "B3")
	if config.EntityABN != "" {
		setRichMeta("A3", "ABN:", config.EntityABN)
	}

	var dateText string
	if data.EndDate != "" {
		dateText = fmt.Sprintf("As of %s", data.EndDate)
	}

	f.MergeCell(sheet, "A4", "B4")
	setRichMeta("A4", "Period:", dateText)

	currentTimeStr := time.Now().Format("02/01/2006, 3:04:05 pm")
	f.MergeCell(sheet, "A5", "B5")
	setRichMeta("A5", "Generated:", currentTimeStr)
	f.MergeCell(sheet, "A6", "B6")

	currentRow := 7

	// Render a balance sheet section (Assets, Liabilities, or Equity)
	renderBSSection := func(title string, accounts []RsAccount, total float64) string {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), title)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.SectionTitle)

		if len(accounts) > 0 {
			tableRange := fmt.Sprintf("A%d:A%d", currentRow, currentRow+len(accounts))
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
		for _, acc := range accounts {
			f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), acc.Name)
			f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.DataLeft)

			f.SetCellValue(sheet, fmt.Sprintf("B%d", currentRow), acc.Balance)
			f.SetCellStyle(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("B%d", currentRow), styles.DataGrid)
			currentRow++
		}
		dataEndRow := currentRow - 1

		totalCell := fmt.Sprintf("B%d", currentRow)
		f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "TOTAL "+title)

		if len(accounts) > 0 {
			formula := fmt.Sprintf("SUBTOTAL(109, B%d:B%d)", dataStartRow, dataEndRow)
			f.SetCellFormula(sheet, totalCell, formula)
		} else {
			f.SetCellValue(sheet, totalCell, 0)
		}

		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("B%d", currentRow), styles.GroupTotal)
		currentRow += 2
		return totalCell
	}

	renderBSSection("ASSETS", data.Assets, data.TotalAssets)
	renderBSSection("LIABILITIES", data.Liabilities, data.TotalLiabilities)
	renderBSSection("EQUITY", data.Equity, data.TotalEquity)

	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "Current Year Profit")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.Profit)
	f.SetCellValue(sheet, fmt.Sprintf("B%d", currentRow), data.CurrentYearProfit)
	f.SetCellStyle(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("B%d", currentRow), styles.ProfitGreen)
	currentRow += 2

	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "TOTAL LIABILITIES & EQUITY")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.Profit)
	f.SetCellValue(sheet, fmt.Sprintf("B%d", currentRow), data.TotalLiabilitiesAndEquity)
	f.SetCellStyle(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("B%d", currentRow), styles.ProfitGreen)

	f.SetColWidth(sheet, "A", "A", 45)
	f.SetColWidth(sheet, "B", "B", 20)

	return f, nil
}
