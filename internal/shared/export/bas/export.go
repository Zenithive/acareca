package bas

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"github.com/iamarpitzala/acareca/internal/shared/export"
	"github.com/samber/lo"
	"github.com/xuri/excelize/v2"
)

type PeriodInfo struct {
	From  string
	To    string
	Label string
}

type RsBASReport struct {
	G1  float64
	A1  float64
	G11 float64
	B1  float64
}

type QuarterData struct {
	Period PeriodInfo
	Report *RsBASReport
}

type BASAmount struct {
	Gross float64 `json:"gross"`
	GST   float64 `json:"gst"`
	Net   float64 `json:"net"`
}

type BASLineItem struct {
	Name    string    `json:"name"`
	Amounts BASAmount `json:"amounts"`
}

type BASSection struct {
	Items []BASLineItem `json:"items"`
}

type BASQuarterInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	StartDate    string `json:"startDate"`
	EndDate      string `json:"endDate"`
	DisplayRange string `json:"displayRange"`
}

type BASColumn struct {
	Quarter  BASQuarterInfo `json:"quarter"`
	Sections struct {
		Income   BASSection `json:"income"`
		Expenses BASSection `json:"expenses"`
	} `json:"sections"`
	NetGSTPayable float64 `json:"net_gst_payable"`
}

type RsBASPreparation struct {
	Columns    []BASColumn `json:"columns"`
	GrandTotal BASColumn   `json:"grand_total"`
}

func GenerateActivityStatementExcelReport(quarters []QuarterData, prevDates PeriodInfo, config export.ExportConfig) (*bytes.Buffer, error) {
	xl := excelize.NewFile()
	defer xl.Close()

	sheet := "Activity Statement"
	dataSheet := "SourceData"
	xl.SetSheetName("Sheet1", sheet)
	xl.NewSheet(dataSheet)

	headers := []string{"Label", "G1", "1A", "G11", "1B", "Start", "End"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		xl.SetCellValue(dataSheet, cell, h)
	}

	for i, q := range quarters {
		row := i + 2
		g1, a1, g11, b1 := 0.0, 0.0, 0.0, 0.0
		if q.Report != nil {
			g1, a1, g11, b1 = q.Report.G1, q.Report.A1, q.Report.G11, q.Report.B1
		}

		xl.SetCellValue(dataSheet, fmt.Sprintf("A%d", row), q.Period.Label)
		xl.SetCellValue(dataSheet, fmt.Sprintf("B%d", row), g1)
		xl.SetCellValue(dataSheet, fmt.Sprintf("C%d", row), a1)
		xl.SetCellValue(dataSheet, fmt.Sprintf("D%d", row), g11)
		xl.SetCellValue(dataSheet, fmt.Sprintf("E%d", row), b1)
		xl.SetCellValue(dataSheet, fmt.Sprintf("F%d", row), q.Period.From)
		xl.SetCellValue(dataSheet, fmt.Sprintf("G%d", row), q.Period.To)
	}
	xl.SetSheetVisible(dataSheet, false)

	// --- Styles ---
	headerStyle, _ := xl.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#4EA7B3"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	subHeaderStyle, _ := xl.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E1F0F2"}, Pattern: 1},
	})
	labelStyle, _ := xl.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	currencyStyle, _ := xl.NewStyle(&excelize.Style{CustomNumFmt: lo.ToPtr("$#,##0.00")})
	totalRowStyle, _ := xl.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#4EA7B3"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})

	// --- Main Header ---
	xl.SetCellValue(sheet, "A1", "Activity Statement Information")
	xl.SetCellValue(sheet, "B1", "BAS")
	xl.SetCellStyle(sheet, "A1", "B1", headerStyle)

	// Helper to format dates to DD-MM-YYYY
	formatDate := func(d string) string {
		t, err := time.Parse("2006-01-02", d)
		if err != nil {
			return d
		}
		return t.Format("02-01-2006")
	}

	setRichMeta := func(cell string, label string, value string) {
		xl.SetCellRichText(sheet, cell, []excelize.RichTextRun{
			{Text: label, Font: &excelize.Font{Bold: true, Family: "Calibri", Size: 10}},
			{Text: " " + value, Font: &excelize.Font{Bold: false, Family: "Calibri", Size: 10}},
		})
	}

	rowOffset := 2
	setRichMeta(fmt.Sprintf("A%d", rowOffset), "Exported by:", config.EntityName)
	rowOffset++

	if config.EntityABN != "" {
		setRichMeta(fmt.Sprintf("A%d", rowOffset), "ABN:", config.EntityABN)
		rowOffset++
	}

	if len(quarters) > 0 {
		periodRange := fmt.Sprintf("%s (%s to %s)",
			quarters[0].Period.Label,
			formatDate(quarters[0].Period.From),
			formatDate(quarters[len(quarters)-1].Period.To),
		)
		setRichMeta(fmt.Sprintf("A%d", rowOffset), "Period:", periodRange)
		rowOffset++
	}

	setRichMeta(fmt.Sprintf("A%d", rowOffset), "Generated:", config.GeneratedTime)
	rowOffset++

	rowOffset++

	qtrRow := rowOffset
	var qLabels []string
	for _, q := range quarters {
		qLabels = append(qLabels, q.Period.Label)
	}
	dv := excelize.NewDataValidation(true)
	dv.Sqref = fmt.Sprintf("B%d", qtrRow)
	dv.SetDropList(qLabels)
	xl.AddDataValidation(sheet, dv)
	xl.SetCellValue(sheet, fmt.Sprintf("A%d", qtrRow), "Qtr")
	xl.SetCellStyle(sheet, fmt.Sprintf("A%d", qtrRow), fmt.Sprintf("A%d", qtrRow), labelStyle)
	if len(qLabels) > 0 {
		xl.SetCellValue(sheet, fmt.Sprintf("B%d", qtrRow), qLabels[0])
	}
	rowOffset++

	xl.SetCellValue(sheet, fmt.Sprintf("A%d", rowOffset), "Period start")
	xl.SetCellFormula(sheet, fmt.Sprintf("B%d", rowOffset), fmt.Sprintf("=VLOOKUP(B%d, %s!A:G, 6, FALSE)", qtrRow, dataSheet))
	xl.SetCellStyle(sheet, fmt.Sprintf("A%d", rowOffset), fmt.Sprintf("A%d", rowOffset), labelStyle)
	periodStartCell := fmt.Sprintf("B%d", rowOffset)
	rowOffset++

	xl.SetCellValue(sheet, fmt.Sprintf("A%d", rowOffset), "Period end")
	xl.SetCellFormula(sheet, fmt.Sprintf("B%d", rowOffset), fmt.Sprintf("=VLOOKUP(B%d, %s!A:G, 7, FALSE)", qtrRow, dataSheet))
	xl.SetCellStyle(sheet, fmt.Sprintf("A%d", rowOffset), fmt.Sprintf("A%d", rowOffset), labelStyle)
	periodEndCell := fmt.Sprintf("B%d", rowOffset)
	rowOffset++

	rowOffset++

	gstFields := []struct {
		Label string
		Col   int
	}{
		{"G1 (Total Sales)", 2},
		{"1A (GST on Sales)", 3},
		{"G11 (Total Purchases)", 4},
		{"1B (GST on Purchases)", 5},
	}

	gstStartRow := rowOffset
	rowIdx := rowOffset
	for _, f := range gstFields {
		xl.SetCellValue(sheet, "A"+strconv.Itoa(rowIdx), f.Label)
		xl.SetCellFormula(sheet, "B"+strconv.Itoa(rowIdx), fmt.Sprintf("=VLOOKUP(B%d, %s!A:G, %d, FALSE)", qtrRow, dataSheet, f.Col))
		xl.SetCellStyle(sheet, "B"+strconv.Itoa(rowIdx), "B"+strconv.Itoa(rowIdx), currencyStyle)
		rowIdx++
	}
	rowOffset = rowIdx

	cell1A := fmt.Sprintf("B%d", gstStartRow+1)
	cell1B := fmt.Sprintf("B%d", gstStartRow+3)

	rowOffset++
	xl.SetCellValue(sheet, "A"+strconv.Itoa(rowOffset), "PAYG tax withheld")
	xl.SetCellStyle(sheet, "A"+strconv.Itoa(rowOffset), "B"+strconv.Itoa(rowOffset), subHeaderStyle)
	rowOffset++

	paygWithheld := []string{
		"Period start",
		"Period end",
		"W1 (Total Wages, salary and other payments)",
		"W2 (Amount withheld from payments shown at W1)",
		"W3 (Other amounts withheld)",
		"W4 (Amount withheld where no ABN is quoted)",
		"W5 (Total amounts withheld)",
	}

	for _, label := range paygWithheld {
		xl.SetCellValue(sheet, "A"+strconv.Itoa(rowOffset), label)
		switch label {
		case "Period start":
			xl.SetCellFormula(sheet, "B"+strconv.Itoa(rowOffset), periodStartCell)
		case "Period end":
			xl.SetCellFormula(sheet, "B"+strconv.Itoa(rowOffset), periodEndCell)
		}
		rowOffset++
	}

	rowOffset++
	xl.SetCellValue(sheet, "A"+strconv.Itoa(rowOffset), "PAYG instalment")
	xl.SetCellStyle(sheet, "A"+strconv.Itoa(rowOffset), "B"+strconv.Itoa(rowOffset), subHeaderStyle)
	rowOffset++
	xl.SetCellValue(sheet, "A"+strconv.Itoa(rowOffset), "Option 1")
	xl.SetCellStyle(sheet, "A"+strconv.Itoa(rowOffset), "A"+strconv.Itoa(rowOffset), labelStyle)
	rowOffset++
	xl.SetCellValue(sheet, "A"+strconv.Itoa(rowOffset), "Option 2")
	xl.SetCellStyle(sheet, "A"+strconv.Itoa(rowOffset), "A"+strconv.Itoa(rowOffset), labelStyle)
	rowOffset++

	rowOffset++
	xl.SetCellValue(sheet, "A"+strconv.Itoa(rowOffset), "GST Payable or (Refund)")
	xl.SetCellFormula(sheet, "B"+strconv.Itoa(rowOffset), fmt.Sprintf("=%s-%s", cell1A, cell1B))
	xl.SetCellStyle(sheet, "A"+strconv.Itoa(rowOffset), "A"+strconv.Itoa(rowOffset), totalRowStyle)
	xl.SetCellStyle(sheet, "B"+strconv.Itoa(rowOffset), "B"+strconv.Itoa(rowOffset), currencyStyle)

	xl.SetColWidth(sheet, "A", "A", 55)
	xl.SetColWidth(sheet, "B", "B", 25)

	return xl.WriteToBuffer()
}

func GenerateBASPreparationExcelReport(data *RsBASPreparation, config export.ExportConfig, fyLabel string) (*excelize.File, error) {
	f := excelize.NewFile()
	sheet := "Quarterly BAS REPORT"
	f.NewSheet(sheet)
	f.DeleteSheet("Sheet1")

	styleHeaderBlue, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Family: "Calibri", Size: 11},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#DAEEF3"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})

	styleDataGrid, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Family: "Calibri", Size: 10},
		CustomNumFmt: lo.ToPtr("$#,##0.00;[Red] $#,##0.00;$0.00"),
		Alignment:    &excelize.Alignment{Horizontal: "left"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})

	styleTableGrid, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Family: "Calibri", Size: 10},
		CustomNumFmt: lo.ToPtr("$#,##0.00;[Red] $#,##0.00;$0.00"),
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"#DAEEF3"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})

	styleSectionTitle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Family: "Calibri", Size: 12},
	})

	styleGSTPayableCol, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Color: "dc3545"},
		CustomNumFmt: lo.ToPtr("$#,##0.00;$#,##0.00"),
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})

	allCols := append(data.Columns, data.GrandTotal)

	setRichMeta := func(cell string, label string, value string) {
		f.SetCellRichText(sheet, cell, []excelize.RichTextRun{
			{Text: label, Font: &excelize.Font{Bold: true, Family: "Calibri", Size: 10}},
			{Text: " " + value, Font: &excelize.Font{Bold: false, Family: "Calibri", Size: 10}},
		})
	}

	lastColIdx := (len(allCols) * 4)
	lastColName, _ := excelize.ColumnNumberToName(lastColIdx)

	f.MergeCell(sheet, "A2", fmt.Sprintf("%s2", lastColName))
	f.SetCellValue(sheet, "A2", fyLabel)
	f.SetCellStyle(sheet, "A2", fmt.Sprintf("%s2", lastColName), styleHeaderBlue)

	f.MergeCell(sheet, "A3", fmt.Sprintf("%s3", lastColName))
	setRichMeta("A3", "Exported by:", config.EntityName)

	f.MergeCell(sheet, "A4", fmt.Sprintf("%s4", lastColName))
	if config.EntityABN != "" {
		setRichMeta("A4", "ABN:", config.EntityABN)
	}

	f.MergeCell(sheet, "A5", fmt.Sprintf("%s5", lastColName))
	currentTimeStr := config.GeneratedTime
	setRichMeta("A5", "Generated:", currentTimeStr)

	f.MergeCell(sheet, "A6", fmt.Sprintf("%s6", lastColName))

	for i := range allCols {
		cIdx := 1 + (i * 4)
		startCol, _ := excelize.ColumnNumberToName(cIdx + 1)
		midCol, _ := excelize.ColumnNumberToName(cIdx + 2)
		endCol, _ := excelize.ColumnNumberToName(cIdx + 3)

		headerValue := allCols[i].Quarter.Name
		if allCols[i].Quarter.StartDate != "" {
			t, err := time.Parse("2006-01-02", allCols[i].Quarter.StartDate)
			yearStr := ""
			if err == nil {
				yearStr = fmt.Sprintf("%d", t.Year())
			}

			headerValue = fmt.Sprintf("%s (%s) %s", allCols[i].Quarter.Name, allCols[i].Quarter.DisplayRange, yearStr)
		}

		f.MergeCell(sheet, fmt.Sprintf("%s7", startCol), fmt.Sprintf("%s7", endCol))
		f.SetCellValue(sheet, fmt.Sprintf("%s7", startCol), headerValue)
		f.SetCellStyle(sheet, fmt.Sprintf("%s7", startCol), fmt.Sprintf("%s7", endCol), styleHeaderBlue)

		f.SetCellValue(sheet, fmt.Sprintf("%s8", startCol), "Gross")
		f.SetCellValue(sheet, fmt.Sprintf("%s8", midCol), "GST")
		f.SetCellValue(sheet, fmt.Sprintf("%s8", endCol), "Net")
		f.SetCellStyle(sheet, fmt.Sprintf("%s8", startCol), fmt.Sprintf("%s8", endCol), styleHeaderBlue)
	}

	type SectionMeta struct {
		StartRow int
		EndRow   int
	}
	var incomeMeta, expenseMeta SectionMeta

	currentRow := 9
	incomeHeaderRow := currentRow
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "INCOME")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleSectionTitle)
	currentRow++
	incomeMeta.StartRow = currentRow

	incomeRows := getUniqueNamesFromSection(allCols, "income")
	for _, name := range incomeRows {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), name)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleDataGrid)

		for i := range allCols {
			cIdx := 1 + (i * 4)
			startCol, _ := excelize.ColumnNumberToName(cIdx + 1)
			midCol, _ := excelize.ColumnNumberToName(cIdx + 2)
			endCol, _ := excelize.ColumnNumberToName(cIdx + 3)

			f.SetCellValue(sheet, fmt.Sprintf("%s%d", startCol, currentRow), 0)
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", midCol, currentRow), 0)
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", endCol, currentRow), 0)

			f.SetCellStyle(sheet, fmt.Sprintf("%s%d", startCol, currentRow), fmt.Sprintf("%s%d", endCol, currentRow), styleTableGrid)
			writeFormattedAmounts(f, sheet, cIdx, currentRow, allCols[i].Sections.Income.Items, name, styleTableGrid)
		}
		currentRow++
	}
	incomeMeta.EndRow = currentRow - 1

	if len(incomeRows) > 0 {
		tblRange := fmt.Sprintf("A%d:A%d", incomeHeaderRow, incomeMeta.EndRow)
		showH := true
		f.AddTable(sheet, &excelize.Table{
			Range:         tblRange,
			Name:          "IncomeTable",
			StyleName:     "",
			ShowHeaderRow: &showH,
		})
	}

	currentRow += 1
	expenseHeaderRow := currentRow
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "EXPENSES")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleSectionTitle)
	currentRow++
	expenseMeta.StartRow = currentRow

	expenseRows := getUniqueNamesFromSection(allCols, "expenses")
	for _, name := range expenseRows {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), name)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleDataGrid)

		for i := range allCols {
			cIdx := 1 + (i * 4)
			startCol, _ := excelize.ColumnNumberToName(cIdx + 1)
			midCol, _ := excelize.ColumnNumberToName(cIdx + 2)
			endCol, _ := excelize.ColumnNumberToName(cIdx + 3)

			f.SetCellValue(sheet, fmt.Sprintf("%s%d", startCol, currentRow), 0)
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", midCol, currentRow), 0)
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", endCol, currentRow), 0)

			f.SetCellStyle(sheet, fmt.Sprintf("%s%d", startCol, currentRow), fmt.Sprintf("%s%d", endCol, currentRow), styleTableGrid)
			writeFormattedAmounts(f, sheet, cIdx, currentRow, allCols[i].Sections.Expenses.Items, name, styleTableGrid)
		}
		currentRow++
	}
	expenseMeta.EndRow = currentRow - 1

	if len(expenseRows) > 0 {
		tblRange := fmt.Sprintf("A%d:A%d", expenseHeaderRow, expenseMeta.EndRow)
		showH := true
		f.AddTable(sheet, &excelize.Table{
			Range:         tblRange,
			Name:          "ExpenseTable",
			StyleName:     "",
			ShowHeaderRow: &showH,
		})
	}

	currentRow += 2
	netGSTRow := currentRow
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "Net GST Payable")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleSectionTitle)

	for i := range allCols {
		cIdx := 1 + (i * 4)
		grossCol, _ := excelize.ColumnNumberToName(cIdx + 1)
		gstCol, _ := excelize.ColumnNumberToName(cIdx + 2)
		netCol, _ := excelize.ColumnNumberToName(cIdx + 3)

		f.MergeCell(sheet, fmt.Sprintf("%s%d", grossCol, netGSTRow), fmt.Sprintf("%s%d", netCol, netGSTRow))

		incomeGST := fmt.Sprintf("SUBTOTAL(109, %s%d:%s%d)", gstCol, incomeMeta.StartRow, gstCol, incomeMeta.EndRow)
		expenseGST := fmt.Sprintf("SUBTOTAL(109, %s%d:%s%d)", gstCol, expenseMeta.StartRow, gstCol, expenseMeta.EndRow)
		f.SetCellFormula(sheet, fmt.Sprintf("%s%d", netCol, netGSTRow), fmt.Sprintf("%s-%s", incomeGST, expenseGST))
		f.SetCellStyle(sheet, fmt.Sprintf("%s%d", grossCol, netGSTRow), fmt.Sprintf("%s%d", netCol, netGSTRow), styleGSTPayableCol)
	}

	f.SetColWidth(sheet, "A", "A", 45)
	for col := 2; col <= 1+(len(allCols)*4); col++ {
		name, _ := excelize.ColumnNumberToName(col)
		if (col-1)%4 == 0 {
			f.SetColWidth(sheet, name, name, 3)
		} else {
			f.SetColWidth(sheet, name, name, 15)
		}
	}

	return f, nil
}

func writeFormattedAmounts(f *excelize.File, sheet string, startIdx, row int, items []BASLineItem, name string, styleID int) {
	for _, item := range items {
		if item.Name == name {
			g, _ := excelize.ColumnNumberToName(startIdx + 1)
			t, _ := excelize.ColumnNumberToName(startIdx + 2)
			n, _ := excelize.ColumnNumberToName(startIdx + 3)

			f.SetCellValue(sheet, fmt.Sprintf("%s%d", g, row), item.Amounts.Gross)
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", t, row), item.Amounts.GST)
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", n, row), item.Amounts.Net)

			f.SetCellStyle(sheet, fmt.Sprintf("%s%d", g, row), fmt.Sprintf("%s%d", n, row), styleID)
			return
		}
	}
}

func getUniqueNamesFromSection(allCols []BASColumn, section string) []string {
	m := make(map[string]bool)
	var names []string
	for _, col := range allCols {
		var items []BASLineItem
		if section == "income" {
			items = col.Sections.Income.Items
		} else {
			items = col.Sections.Expenses.Items
		}
		for _, itm := range items {
			if itm.Name != "" && !m[itm.Name] {
				m[itm.Name] = true
				names = append(names, itm.Name)
			}
		}
	}
	return names
}
