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

// helper to translate DateUntil string safely into Financial Year values
func getFinancialYearHeaderPL(dateStr string) string {
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

func GenerateExcelReport(data []*RsReport, config export.ExportConfig) (*excelize.File, error) {
	f := excelize.NewFile()
	sheet := "Profit and Loss"
	f.NewSheet(sheet)
	f.DeleteSheet("Sheet1")

	styles := applyReportStyles(f)

	centerHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Family: "Calibri", Size: 11, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1F4E78"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	if len(data) == 0 {
		return f, fmt.Errorf("no report data datasets parsed")
	}
	numYears := len(data)

	getColLetter := func(colIndex int) string {
		letter, _ := excelize.ColumnNumberToName(colIndex)
		return letter
	}
	lastColLetter := getColLetter(numYears + 1)

	setMetaRow := func(cell string, label string, value string) {
		f.SetCellRichText(sheet, cell, []excelize.RichTextRun{
			{Text: label, Font: &excelize.Font{Bold: true, Family: "Calibri", Size: 10}},
			{Text: " " + value, Font: &excelize.Font{Bold: false, Family: "Calibri", Size: 10}},
		})
	}

	f.SetCellValue(sheet, "A1", "Profit and Loss Report")
	f.MergeCell(sheet, "A1", lastColLetter+"1")
	f.SetCellStyle(sheet, "A1", lastColLetter+"1", styles.HeaderBlue)

	currentRow := 2
	setMetaRow(fmt.Sprintf("A%d", currentRow), "Exported by:", config.EntityName)
	currentRow++

	if config.EntityABN != "" {
		setMetaRow(fmt.Sprintf("A%d", currentRow), "ABN:", config.EntityABN)
		currentRow++
	}

	if config.Period != "" {
		setMetaRow(fmt.Sprintf("A%d", currentRow), "Period:", config.Period)
		currentRow++
	}

	currentTimeStr := time.Now().Format("02/01/2006, 3:04:05 pm")
	setMetaRow(fmt.Sprintf("A%d", currentRow), "Generated:", currentTimeStr)
	currentRow += 3

	f.SetCellValue(sheet, "A7", "Account")
	f.SetCellStyle(sheet, "A7", "A7", centerHeaderStyle)
	for yIdx, yr := range data {
		colLetter := getColLetter(yIdx + 2)
		var columnLabel string
		if numYears > 1 {
			columnLabel = getFinancialYearHeaderPL(yr.ReportMetadata.DateUntil)
		} else {
			columnLabel = "Balance"
		}
		f.SetCellValue(sheet, fmt.Sprintf("%s7", colLetter), columnLabel)
		f.SetCellStyle(sheet, fmt.Sprintf("%s7", colLetter), fmt.Sprintf("%s7", colLetter), centerHeaderStyle)
	}

	currentRow = 8

	renderGroupMultiColumn := func(title string, sectionKey string) []string {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), title)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.SectionTitle)

		type UniqueAcc struct {
			CoaID   string
			CoaName string
		}
		var uniqueAccounts []UniqueAcc
		seen := map[string]bool{}

		for _, yr := range data {
			var targets []RsReportAccount
			switch sectionKey {
			case "income":
				targets = yr.Income.Accounts
			case "cos":
				targets = yr.CostOfSales.Accounts
			case "other":
				targets = yr.OtherCosts.Accounts
			}
			for _, acc := range targets {
				if !seen[acc.CoaID] {
					seen[acc.CoaID] = true
					uniqueAccounts = append(uniqueAccounts, UniqueAcc{CoaID: acc.CoaID, CoaName: acc.CoaName})
				}
			}
		}

		// Inject single-column filter bounding structure logic
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
			f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), uAcc.CoaName)
			f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.DataLeft)

			for yIdx, yr := range data {
				colLetter := getColLetter(yIdx + 2)
				var amt float64 = 0.0

				var checkList []RsReportAccount
				switch sectionKey {
				case "income":
					checkList = yr.Income.Accounts
				case "cos":
					checkList = yr.CostOfSales.Accounts
				case "other":
					checkList = yr.OtherCosts.Accounts
				}

				for _, acc := range checkList {
					if acc.CoaID == uAcc.CoaID {
						amt = acc.TotalValue
						break
					}
				}
				f.SetCellValue(sheet, fmt.Sprintf("%s%d", colLetter, currentRow), amt)
				f.SetCellStyle(sheet, fmt.Sprintf("%s%d", colLetter, currentRow), fmt.Sprintf("%s%d", colLetter, currentRow), styles.DataGrid)
			}
			currentRow++
		}
		dataEndRow := currentRow - 1

		// Total summary tracking strings vector array mapping slices
		var cellsCollected []string
		f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "TOTAL "+title)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.GroupTotal)

		for yIdx := 0; yIdx < numYears; yIdx++ {
			colLetter := getColLetter(yIdx + 2)
			totalCell := fmt.Sprintf("%s%d", colLetter, currentRow)
			cellsCollected = append(cellsCollected, totalCell)

			if len(uniqueAccounts) > 0 {
				f.SetCellFormula(sheet, totalCell, fmt.Sprintf("SUBTOTAL(109, %s%d:%s%d)", colLetter, dataStartRow, colLetter, dataEndRow))
			} else {
				f.SetCellValue(sheet, totalCell, 0)
			}
			f.SetCellStyle(sheet, totalCell, totalCell, styles.GroupTotal)
		}
		currentRow += 2
		return cellsCollected
	}

	incomeCells := renderGroupMultiColumn("INCOME", "income")
	cosCells := renderGroupMultiColumn("COST OF SALES", "cos")

	// Gross Profit Calculation Row Execution
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "GROSS PROFIT")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.Profit)

	grossProfitCells := make([]string, numYears)
	for yIdx := 0; yIdx < numYears; yIdx++ {
		colLetter := getColLetter(yIdx + 2)
		targetCell := fmt.Sprintf("%s%d", colLetter, currentRow)
		grossProfitCells[yIdx] = targetCell
		f.SetCellFormula(sheet, targetCell, fmt.Sprintf("%s-%s", incomeCells[yIdx], cosCells[yIdx]))
		f.SetCellStyle(sheet, targetCell, targetCell, styles.ProfitGreen)
	}
	currentRow += 2

	otherCostsCells := renderGroupMultiColumn("OTHER COSTS", "other")

	// Net Profit Calculation Row Execution
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "NET PROFIT")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.Profit)

	for yIdx := 0; yIdx < numYears; yIdx++ {
		colLetter := getColLetter(yIdx + 2)
		targetCell := fmt.Sprintf("%s%d", colLetter, currentRow)
		f.SetCellFormula(sheet, targetCell, fmt.Sprintf("%s-%s", grossProfitCells[yIdx], otherCostsCells[yIdx]))
		f.SetCellStyle(sheet, targetCell, targetCell, styles.ProfitGreen)
	}

	// Scale column widths to fit financial headers comfortably
	f.SetColWidth(sheet, "A", "A", 45)
	for i := 1; i <= numYears; i++ {
		f.SetColWidth(sheet, getColLetter(i+1), getColLetter(i+1), 30)
	}
	f.UpdateLinkedValue()

	return f, nil
}
