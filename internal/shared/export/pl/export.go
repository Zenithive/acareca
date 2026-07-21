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

type RsReportAccount struct {
	CoaID      string  `json:"coa_id"`
	CoaName    string  `json:"coa_name"`
	TotalValue float64 `json:"total_value"`
}

type RsReportGroup struct {
	GroupTotal float64           `json:"group_total"`
	Accounts   []RsReportAccount `json:"accounts"`
}

type RsReportMetadata struct {
	DateFrom         string  `json:"date_from"`
	DateUntil        string  `json:"date_until"`
	OverallNetProfit float64 `json:"overall_net_profit"`
}

type RsReport struct {
	ReportMetadata   RsReportMetadata `json:"report_metadata"`
	Income           RsReportGroup    `json:"income"`
	CostOfSales      RsReportGroup    `json:"cost_of_sales"`
	GrossProfit      float64          `json:"gross_profit"`
	OtherCosts       RsReportGroup    `json:"other_costs"`
	ITRReportingItem RsReportGroup    `json:"itr_reporting_item"`
	NetProfit        float64          `json:"net_profit"`
}

func Round2(v float64) float64 {
	return math.Round(v*100) / 100
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

	return StyleSet{
		HeaderBlue:      hBlue,
		SectionTitle:    sTitle,
		DataLeft:        dLeft,
		DataGrid:        dGrid,
		GroupTotalLabel: gTotalLabel,
		GroupTotal:      gTotal,
		Profit:          profit,
		ProfitGreen:     profitGreen,
	}
}

type StyleSet struct {
	HeaderBlue      int
	SectionTitle    int
	DataLeft        int
	DataGrid        int
	GroupTotalLabel int
	GroupTotal      int
	Profit          int
	ProfitGreen     int
}

func FormatDateStr(dateStr string) string {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return dateStr
	}
	return t.Format("02-01-2006")
}

func getDynamicPeriodHeader(meta RsReportMetadata) string {
	parseTime := func(dStr string) (time.Time, error) {
		t, err := time.Parse("2006-01-02", dStr)
		if err != nil {
			t, err = time.Parse("02-01-2006", dStr)
		}
		return t, err
	}

	start, errFrom := parseTime(meta.DateFrom)
	until, errUntil := parseTime(meta.DateUntil)

	if errFrom != nil || errUntil != nil {
		return meta.DateUntil
	}

	// Output format: "May 2026"
	if start.Year() == until.Year() && start.Month() == until.Month() {
		return until.Format("January 2006")
	}

	// Output format: "1 June 2026 – 30 June 2026"
	return fmt.Sprintf("%d %s %d – %d %s %d",
		start.Day(), start.Format("January"), start.Year(),
		until.Day(), until.Format("January"), until.Year(),
	)
}

func GenerateExcelReport(data []*RsReport, config export.ExportConfig) (*excelize.File, error) {
	f := excelize.NewFile()
	sheet := "Profit and Loss"
	f.NewSheet(sheet)
	f.DeleteSheet("Sheet1")

	styles := applyReportStyles(f)

	centerHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Family: "Segoe UI", Size: 10, Color: "FFFFFF"},
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
			{Text: label, Font: &excelize.Font{Bold: true, Family: "Segoe UI", Size: 9.5, Color: "595959"}},
			{Text: " " + value, Font: &excelize.Font{Bold: false, Family: "Segoe UI", Size: 9.5, Color: "262626"}},
		})
	}

	f.SetRowHeight(sheet, 1, 28)
	f.SetCellValue(sheet, "A1", "Profit and Loss Report")
	f.MergeCell(sheet, "A1", lastColLetter+"1")
	f.SetCellStyle(sheet, "A1", lastColLetter+"1", styles.HeaderBlue)

	currentRow := 2
	setMetaRow(fmt.Sprintf("A%d", currentRow), "Exported By:", config.EntityName)
	f.MergeCell(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("%s%d", lastColLetter, currentRow))
	f.SetRowHeight(sheet, currentRow, 18)
	currentRow++

	if config.EntityABN != "" {
		setMetaRow(fmt.Sprintf("A%d", currentRow), "ABN:", config.EntityABN)
		f.MergeCell(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("%s%d", lastColLetter, currentRow))
		f.SetRowHeight(sheet, currentRow, 18)
		currentRow++
	}

	if config.Period != "" {
		setMetaRow(fmt.Sprintf("A%d", currentRow), "Period:", config.Period)
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
		var columnLabel string
		if numYears > 1 {
			columnLabel = getDynamicPeriodHeader(yr.ReportMetadata)
		} else {
			columnLabel = "Balance"
		}
		cellRef := fmt.Sprintf("%s%d", colLetter, currentRow)
		f.SetCellValue(sheet, cellRef, columnLabel)
		f.SetCellStyle(sheet, cellRef, cellRef, centerHeaderStyle)
	}

	renderGroupMultiColumn := func(title string, sectionKey string) []string {
		currentRow++
		f.SetRowHeight(sheet, currentRow, 22)
		f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), title)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.SectionTitle)

		type UniqueAcc struct {
			CoaID   string
			CoaName string
		}
		var uniqueAccounts []UniqueAcc
		seen := map[string]bool{}

		if sectionKey != "" {
			for _, yr := range data {
				var targets []RsReportAccount
				switch sectionKey {
				case "income":
					targets = yr.Income.Accounts
				case "cos":
					targets = yr.CostOfSales.Accounts
				case "other":
					targets = yr.OtherCosts.Accounts
				case "itr":
					targets = yr.ITRReportingItem.Accounts
				}
				for _, acc := range targets {
					if !seen[acc.CoaID] {
						seen[acc.CoaID] = true
						uniqueAccounts = append(uniqueAccounts, UniqueAcc{CoaID: acc.CoaID, CoaName: acc.CoaName})
					}
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
			f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), uAcc.CoaName)
			f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.DataLeft)

			for yIdx, yr := range data {
				colLetter := getColLetter(yIdx + 2)
				var amt float64 = 0.0

				if sectionKey != "" {
					var checkList []RsReportAccount
					switch sectionKey {
					case "income":
						checkList = yr.Income.Accounts
					case "cos":
						checkList = yr.CostOfSales.Accounts
					case "other":
						checkList = yr.OtherCosts.Accounts
					case "itr":
						checkList = yr.ITRReportingItem.Accounts
					}

					for _, acc := range checkList {
						if acc.CoaID == uAcc.CoaID {
							amt = acc.TotalValue
							break
						}
					}
				} else {
					amt = 0.00
				}
				f.SetCellValue(sheet, fmt.Sprintf("%s%d", colLetter, currentRow), amt)
				f.SetCellStyle(sheet, fmt.Sprintf("%s%d", colLetter, currentRow), fmt.Sprintf("%s%d", colLetter, currentRow), styles.DataGrid)
			}
			currentRow++
		}
		dataEndRow := currentRow - 1

		var cellsCollected []string
		f.SetRowHeight(sheet, currentRow, 22)

		var displayLabel string
		if title == "ITR REPORTABLE ITEMS" {
			displayLabel = "Total ITR Reportable Items"
		} else {
			displayLabel = "Total " + strings.Title(strings.ToLower(title))
		}

		f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), displayLabel)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.GroupTotalLabel)

		for yIdx := 0; yIdx < numYears; yIdx++ {
			colLetter := getColLetter(yIdx + 2)
			totalCell := fmt.Sprintf("%s%d", colLetter, currentRow)
			cellsCollected = append(cellsCollected, totalCell)

			if len(uniqueAccounts) > 0 {
				f.SetCellFormula(sheet, totalCell, fmt.Sprintf("SUBTOTAL(9, %s%d:%s%d)", colLetter, dataStartRow, colLetter, dataEndRow))
			} else {
				f.SetCellValue(sheet, totalCell, 0.00)
			}
			f.SetCellStyle(sheet, totalCell, totalCell, styles.GroupTotal)
		}

		currentRow++
		f.SetRowHeight(sheet, currentRow, 12)

		return cellsCollected
	}

	incomeCells := renderGroupMultiColumn("INCOME", "income")
	cosCells := renderGroupMultiColumn("COST OF SALES", "cos")

	currentRow++
	f.SetRowHeight(sheet, currentRow, 26)
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

	currentRow++
	f.SetRowHeight(sheet, currentRow, 12)

	otherCostsCells := renderGroupMultiColumn("OTHER COSTS", "other")

	itrCells := renderGroupMultiColumn("ITR REPORTABLE ITEMS", "itr")

	currentRow++
	f.SetRowHeight(sheet, currentRow, 26)
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "NET PROFIT")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styles.Profit)

	netProfitCells := make([]string, numYears)
	for yIdx := 0; yIdx < numYears; yIdx++ {
		colLetter := getColLetter(yIdx + 2)
		targetCell := fmt.Sprintf("%s%d", colLetter, currentRow)
		netProfitCells[yIdx] = targetCell
		f.SetCellFormula(sheet, targetCell, fmt.Sprintf("%s-%s-%s", grossProfitCells[yIdx], otherCostsCells[yIdx], itrCells[yIdx]))
		f.SetCellStyle(sheet, targetCell, targetCell, styles.ProfitGreen)
	}

	currentRow++
	f.SetRowHeight(sheet, currentRow, 12)

	f.SetColWidth(sheet, "A", "A", 60)
	for i := 1; i <= numYears; i++ {
		f.SetColWidth(sheet, getColLetter(i+1), getColLetter(i+1), 30)
	}
	f.UpdateLinkedValue()

	return f, nil
}
