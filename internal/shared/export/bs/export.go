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

// helper function to turn an end-date into a "Financial Year YYYY-YYYY" string
func getFinancialYearHeader(dateStr string) string {
	var t time.Time
	var err error
	t, err = time.Parse("2006-01-02", dateStr)
	if err != nil {
		t, err = time.Parse("02-01-2006", dateStr)
	}
	if err != nil {
		return dateStr
	}

	var startYear, endYear int
	if t.Month() >= 7 {
		startYear = t.Year()
		endYear = t.Year() + 1
	} else {
		startYear = t.Year() - 1
		endYear = t.Year()
	}

	return fmt.Sprintf("Financial Year %d-%d", startYear, endYear)
}

// GenerateExcelReport generates an Excel file for balance sheet report
func GenerateExcelReport(data []*RsBalanceSheet, config export.ExportConfig) (*excelize.File, error) {
	f := excelize.NewFile()
	sheet := "Balance Sheet"
	f.NewSheet(sheet)
	f.DeleteSheet("Sheet1")

	styles := export.ApplyCommonStyles(f)

	centerHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Family: "Calibri", Size: 11, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1F4E78"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	if len(data) == 0 {
		return f, fmt.Errorf("no report datasets provided")
	}
	baseline := data[0]
	numYears := len(data)

	getColLetter := func(colIndex int) string {
		letter, _ := excelize.ColumnNumberToName(colIndex)
		return letter
	}

	lastColLetter := getColLetter(numYears + 1)

	setRichMeta := func(cell string, label string, value string) {
		f.SetCellRichText(sheet, cell, []excelize.RichTextRun{
			{Text: label, Font: &excelize.Font{Bold: true, Family: "Calibri", Size: 10}},
			{Text: " " + value, Font: &excelize.Font{Bold: false, Family: "Calibri", Size: 10}},
		})
	}

	// Set title
	f.SetCellValue(sheet, "A1", "Balance Sheet")
	f.MergeCell(sheet, "A1", lastColLetter+"1")
	f.SetCellStyle(sheet, "A1", lastColLetter+"1", styles.HeaderBlue)

	setRichMeta("A2", "Exported by:", config.EntityName)
	f.MergeCell(sheet, "A2", lastColLetter+"2")

	if config.EntityABN != "" {
		setRichMeta("A3", "ABN:", config.EntityABN)
		f.MergeCell(sheet, "A3", lastColLetter+"3")
	}

	var dateText string
	if numYears > 1 {
		if baseline.EndDate != "" {
			dateText = fmt.Sprintf("As of %s (with %d Comparative Periods)", baseline.EndDate, numYears)
		}
	} else {
		if baseline.EndDate != "" {
			dateText = fmt.Sprintf("As of %s", baseline.EndDate)
		}
	}

	setRichMeta("A4", "Period:", dateText)
	f.MergeCell(sheet, "A4", lastColLetter+"4")

	currentTimeStr := time.Now().Format("02/01/2006, 3:04:05 pm")
	setRichMeta("A5", "Generated:", currentTimeStr)
	f.MergeCell(sheet, "A5", lastColLetter+"5")
	f.MergeCell(sheet, "A6", lastColLetter+"6")

	f.SetCellValue(sheet, "A7", "Account")
	f.SetCellStyle(sheet, "A7", "A7", centerHeaderStyle)
	for yIdx, yr := range data {
		colLetter := getColLetter(yIdx + 2)
		fyLabel := getFinancialYearHeader(yr.EndDate)
		f.SetCellValue(sheet, fmt.Sprintf("%s7", colLetter), fyLabel)
		f.SetCellStyle(sheet, fmt.Sprintf("%s7", colLetter), fmt.Sprintf("%s7", colLetter), centerHeaderStyle)
	}

	currentRow := 8

	renderBSSection := func(title string, sectionKey string) {
		// Render section category separator row title
		f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), title)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.SectionTitle)

		type UniqueAcc struct {
			Code int16
			Name string
		}
		var uniqueAccounts []UniqueAcc
		seen := make(map[int16]bool)

		for _, yr := range data {
			var targets []RsAccount
			switch sectionKey {
			case "assets":
				targets = yr.Assets
			case "liabilities":
				targets = yr.Liabilities
			case "equity":
				targets = yr.Equity
			}
			for _, acc := range targets {
				if !seen[acc.Code] {
					seen[acc.Code] = true
					uniqueAccounts = append(uniqueAccounts, UniqueAcc{Code: acc.Code, Name: acc.Name})
				}
			}
		}

		if len(uniqueAccounts) > 0 {
			tableRange := fmt.Sprintf("A%d:A%d", currentRow, currentRow+len(uniqueAccounts))
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

		for _, uAcc := range uniqueAccounts {
			f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), uAcc.Name)
			f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.DataLeft)

			for yIdx, yr := range data {
				colLetter := getColLetter(yIdx + 2)
				var targetBal float64 = 0.0

				var checkList []RsAccount
				switch sectionKey {
				case "assets":
					checkList = yr.Assets
				case "liabilities":
					checkList = yr.Liabilities
				case "equity":
					checkList = yr.Equity
				}

				for _, acc := range checkList {
					if acc.Code == uAcc.Code {
						targetBal = acc.Balance
						break
					}
				}

				f.SetCellValue(sheet, fmt.Sprintf("%s%d", colLetter, currentRow), targetBal)
				f.SetCellStyle(sheet, fmt.Sprintf("%s%d", colLetter, currentRow), fmt.Sprintf("%s%d", colLetter, currentRow), styles.DataGrid)
			}
			currentRow++
		}
		dataEndRow := currentRow - 1

		f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "TOTAL "+title)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.GroupTotal)

		for yIdx := 0; yIdx < numYears; yIdx++ {
			colLetter := getColLetter(yIdx + 2)
			totalCell := fmt.Sprintf("%s%d", colLetter, currentRow)

			if len(uniqueAccounts) > 0 {
				formula := fmt.Sprintf("SUBTOTAL(109, %s%d:%s%d)", colLetter, dataStartRow, colLetter, dataEndRow)
				f.SetCellFormula(sheet, totalCell, formula)
			} else {
				f.SetCellValue(sheet, totalCell, 0)
			}
			f.SetCellStyle(sheet, totalCell, totalCell, styles.GroupTotal)
		}
		currentRow += 2
	}

	renderBSSection("ASSETS", "assets")
	renderBSSection("LIABILITIES", "liabilities")
	renderBSSection("EQUITY", "equity")

	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "Current Year Profit")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.Profit)

	for yIdx, yr := range data {
		colLetter := getColLetter(yIdx + 2)
		cellCoords := fmt.Sprintf("%s%d", colLetter, currentRow)
		f.SetCellValue(sheet, cellCoords, yr.CurrentYearProfit)
		f.SetCellStyle(sheet, cellCoords, cellCoords, styles.ProfitGreen)
	}
	currentRow += 2

	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "TOTAL LIABILITIES & EQUITY")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.Profit)

	for yIdx, yr := range data {
		colLetter := getColLetter(yIdx + 2)
		cellCoords := fmt.Sprintf("%s%d", colLetter, currentRow)
		f.SetCellValue(sheet, cellCoords, yr.TotalLiabilitiesAndEquity)
		f.SetCellStyle(sheet, cellCoords, cellCoords, styles.ProfitGreen)
	}

	f.SetColWidth(sheet, "A", "A", 45)
	for i := 1; i <= numYears; i++ {
		f.SetColWidth(sheet, getColLetter(i+1), getColLetter(i+1), 30)
	}

	return f, nil
}
