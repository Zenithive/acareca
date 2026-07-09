package bs

import (
	"fmt"
	"strings"
	"time"

	"github.com/iamarpitzala/acareca/internal/shared/export"
	lo "github.com/samber/lo"
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

type StyleSet struct {
	HeaderBlue        int
	SectionTitle      int
	DataLeft          int
	DataGrid          int
	GroupTotalLabel   int
	GroupTotal        int
	Profit            int
	ProfitGreen       int
	CurrentProfitText int
	CurrentProfitNum  int
}

func applyReportStyles(f *excelize.File) StyleSet {
	hBlue, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Family: "Segoe UI", Size: 12, Color: "FFFFFF"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1F4E78"}, Pattern: 1},
	})

	sTitle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Family: "Segoe UI", Size: 11, Color: "1F4E78"},
	})

	dLeft, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Family: "Segoe UI", Size: 10, Color: "262626"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", Indent: 1},
		Border: []excelize.Border{
			{Type: "bottom", Color: "F2F2F2", Style: 1},
		},
	})

	dGrid, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Family: "Segoe UI", Size: 10, Color: "262626"},
		CustomNumFmt: lo.ToPtr("$#,##0.00;($#,##0.00);$0.00"),
		Alignment:    &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "bottom", Color: "F2F2F2", Style: 1},
		},
	})

	gTotalLabel, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Family: "Segoe UI", Size: 10, Color: "000000"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"F8F9FA"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", Indent: 1},
		Border: []excelize.Border{
			{Type: "top", Color: "D9D9D9", Style: 1},
			{Type: "bottom", Color: "D9D9D9", Style: 1},
		},
	})

	gTotal, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Family: "Segoe UI", Size: 10, Color: "000000"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"F8F9FA"}, Pattern: 1},
		CustomNumFmt: lo.ToPtr("$#,##0.00;($#,##0.00);$0.00"),
		Alignment:    &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "top", Color: "D9D9D9", Style: 1},
			{Type: "bottom", Color: "D9D9D9", Style: 1},
		},
	})

	profit, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Family: "Segoe UI", Size: 11, Color: "1F4E78"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"EDF2F8"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", Indent: 1},
		Border: []excelize.Border{
			{Type: "top", Color: "1F4E78", Style: 1},
			{Type: "bottom", Color: "1F4E78", Style: 6},
		},
	})

	profitGreen, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Family: "Segoe UI", Size: 11, Color: "1F4E78"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"EDF2F8"}, Pattern: 1},
		CustomNumFmt: lo.ToPtr("$#,##0.00;($#,##0.00);$0.00"),
		Alignment:    &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "top", Color: "1F4E78", Style: 1},
			{Type: "bottom", Color: "1F4E78", Style: 6},
		},
	})

	cProfitText, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Italic: true, Family: "Segoe UI", Size: 10, Color: "1F4E78"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"EDF2F8"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", Indent: 1},
		Border: []excelize.Border{
			{Type: "top", Color: "D9D9D9", Style: 1},
			{Type: "bottom", Color: "D9D9D9", Style: 1},
		},
	})

	cProfitNum, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Italic: true, Family: "Segoe UI", Size: 10, Color: "1F4E78"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"EDF2F8"}, Pattern: 1},
		CustomNumFmt: lo.ToPtr("$#,##0.00;($#,##0.00);$0.00"),
		Alignment:    &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "top", Color: "D9D9D9", Style: 1},
			{Type: "bottom", Color: "D9D9D9", Style: 1},
		},
	})

	return StyleSet{
		HeaderBlue:        hBlue,
		SectionTitle:      sTitle,
		DataLeft:          dLeft,
		DataGrid:          dGrid,
		GroupTotalLabel:   gTotalLabel,
		GroupTotal:        gTotal,
		Profit:            profit,
		ProfitGreen:       profitGreen,
		CurrentProfitText: cProfitText,
		CurrentProfitNum:  cProfitNum,
	}
}

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

	styles := applyReportStyles(f)

	centerHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Family: "Segoe UI", Size: 10, Color: "FFFFFF"},
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

	setMetaRow := func(cell string, label string, value string) {
		f.SetCellRichText(sheet, cell, []excelize.RichTextRun{
			{Text: label, Font: &excelize.Font{Bold: true, Family: "Segoe UI", Size: 9.5, Color: "595959"}},
			{Text: " " + value, Font: &excelize.Font{Bold: false, Family: "Segoe UI", Size: 9.5, Color: "262626"}},
		})
	}

	f.SetRowHeight(sheet, 1, 28)
	f.SetCellValue(sheet, "A1", "Balance Sheet")
	f.MergeCell(sheet, "A1", lastColLetter+"1")
	f.SetCellStyle(sheet, "A1", lastColLetter+"1", styles.HeaderBlue)

	currentRow := 2
	setMetaRow(fmt.Sprintf("A%d", currentRow), "Exported by:", config.EntityName)
	f.MergeCell(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("%s%d", lastColLetter, currentRow))
	f.SetRowHeight(sheet, currentRow, 18)
	currentRow++

	if config.EntityABN != "" {
		setMetaRow(fmt.Sprintf("A%d", currentRow), "ABN:", config.EntityABN)
		f.MergeCell(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("%s%d", lastColLetter, currentRow))
		f.SetRowHeight(sheet, currentRow, 18)
		currentRow++
	}

	var dateText string
	if numYears > 1 {
		if baseline.EndDate != "" {
			dateText = fmt.Sprintf("As at %s (with %d Comparative Periods)", baseline.EndDate, numYears)
		}
	} else {
		if baseline.EndDate != "" {
			dateText = fmt.Sprintf("As at %s", baseline.EndDate)
		}
	}

	if dateText != "" {
		setMetaRow(fmt.Sprintf("A%d", currentRow), "Period:", dateText)
		f.MergeCell(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("%s%d", lastColLetter, currentRow))
		f.SetRowHeight(sheet, currentRow, 18)
		currentRow++
	}

	currentTimeStr := time.Now().Format("02/01/2006, 3:04:05 pm")
	setMetaRow(fmt.Sprintf("A%d", currentRow), "Generated:", currentTimeStr)
	f.MergeCell(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("%s%d", lastColLetter, currentRow))
	f.SetRowHeight(sheet, currentRow, 18)

	currentRow += 2

	f.SetRowHeight(sheet, currentRow, 26)
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "Account")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), centerHeaderStyle)

	for yIdx, yr := range data {
		colLetter := getColLetter(yIdx + 2)
		fyLabel := getFinancialYearHeader(yr.EndDate)
		cellRef := fmt.Sprintf("%s%d", colLetter, currentRow)
		f.SetCellValue(sheet, cellRef, fyLabel)
		f.SetCellStyle(sheet, cellRef, cellRef, centerHeaderStyle)
	}

	renderBSSection := func(title string, sectionKey string) (int, int, bool) {
		currentRow++
		f.SetRowHeight(sheet, currentRow, 22)
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
			f.SetRowHeight(sheet, currentRow, 20)
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

		return dataStartRow, dataEndRow, len(uniqueAccounts) > 0
	}

	assetStart, assetEnd, hasAssets := renderBSSection("ASSETS", "assets")
	assetTotalRows := make([]string, numYears)

	f.SetRowHeight(sheet, currentRow, 22)
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "TOTAL ASSETS")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.GroupTotalLabel)
	for yIdx := 0; yIdx < numYears; yIdx++ {
		colLetter := getColLetter(yIdx + 2)
		cell := fmt.Sprintf("%s%d", colLetter, currentRow)
		assetTotalRows[yIdx] = cell
		if hasAssets {
			f.SetCellFormula(sheet, cell, fmt.Sprintf("SUBTOTAL(9, %s%d:%s%d)", colLetter, assetStart, colLetter, assetEnd))
		} else {
			f.SetCellValue(sheet, cell, 0.00)
		}
		f.SetCellStyle(sheet, cell, cell, styles.GroupTotal)
	}
	currentRow++
	f.SetRowHeight(sheet, currentRow, 12)

	liabStart, liabEnd, hasLiab := renderBSSection("LIABILITIES", "liabilities")
	liabilityTotalRows := make([]string, numYears)

	f.SetRowHeight(sheet, currentRow, 22)
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "TOTAL LIABILITIES")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.GroupTotalLabel)
	for yIdx := 0; yIdx < numYears; yIdx++ {
		colLetter := getColLetter(yIdx + 2)
		cell := fmt.Sprintf("%s%d", colLetter, currentRow)
		liabilityTotalRows[yIdx] = cell
		if hasLiab {
			f.SetCellFormula(sheet, cell, fmt.Sprintf("SUBTOTAL(9, %s%d:%s%d)", colLetter, liabStart, colLetter, liabEnd))
		} else {
			f.SetCellValue(sheet, cell, 0.00)
		}
		f.SetCellStyle(sheet, cell, cell, styles.GroupTotal)
	}
	currentRow++
	f.SetRowHeight(sheet, currentRow, 12)

	equityStart, equityEnd, hasEquity := renderBSSection("EQUITY", "equity")

	f.SetRowHeight(sheet, currentRow, 24)
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "Current Year Profit")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.CurrentProfitText)

	currentYearProfitCells := make([]string, numYears)
	for yIdx, yr := range data {
		colLetter := getColLetter(yIdx + 2)
		targetCell := fmt.Sprintf("%s%d", colLetter, currentRow)
		currentYearProfitCells[yIdx] = targetCell
		f.SetCellValue(sheet, targetCell, yr.CurrentYearProfit)
		f.SetCellStyle(sheet, targetCell, targetCell, styles.CurrentProfitNum)
	}

	currentRow++
	f.SetRowHeight(sheet, currentRow, 12)

	f.SetRowHeight(sheet, currentRow, 22)
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "TOTAL EQUITY")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.GroupTotalLabel)

	for yIdx := 0; yIdx < numYears; yIdx++ {
		colLetter := getColLetter(yIdx + 2)
		totalCell := fmt.Sprintf("%s%d", colLetter, currentRow)

		var subtotalStr string
		if hasEquity {
			subtotalStr = fmt.Sprintf("SUBTOTAL(9, %s%d:%s%d)", colLetter, equityStart, colLetter, equityEnd)
		} else {
			subtotalStr = "0.00"
		}

		f.SetCellFormula(sheet, totalCell, fmt.Sprintf("%s+%s", subtotalStr, currentYearProfitCells[yIdx]))
		f.SetCellStyle(sheet, totalCell, totalCell, styles.GroupTotal)
	}

	currentRow++
	f.SetRowHeight(sheet, currentRow, 12)

	currentRow++
	f.SetRowHeight(sheet, currentRow, 26)
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "NET ASSETS")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.Profit)

	for yIdx := 0; yIdx < numYears; yIdx++ {
		colLetter := getColLetter(yIdx + 2)
		targetCell := fmt.Sprintf("%s%d", colLetter, currentRow)

		formula := fmt.Sprintf("%s-%s", assetTotalRows[yIdx], liabilityTotalRows[yIdx])
		f.SetCellFormula(sheet, targetCell, formula)
		f.SetCellStyle(sheet, targetCell, targetCell, styles.ProfitGreen)
	}

	f.SetColWidth(sheet, "A", "A", 60)
	for i := 1; i <= numYears; i++ {
		f.SetColWidth(sheet, getColLetter(i+1), getColLetter(i+1), 30)
	}
	f.UpdateLinkedValue()

	return f, nil
}
