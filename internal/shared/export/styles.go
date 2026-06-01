package export

import "github.com/xuri/excelize/v2"

// ApplyCommonStyles applies standard styling to an Excel workbook
func ApplyCommonStyles(f *excelize.File) StyleSet {
	styles := StyleSet{}

	// Header Blue - Main title style
	styles.HeaderBlue, _ = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Family: "Calibri", Size: 14},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#DAEEF3"}, Pattern: 1},
	})

	// Header White - Column header style
	styles.HeaderWhite, _ = f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#4EA7B3"}, Pattern: 1},
	})

	// Section Title - Section header style
	styles.SectionTitle, _ = f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Family: "Calibri", Size: 12},
	})

	// Group Header - Category header style
	styles.GroupHeader, _ = f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#DAEEF3"}, Pattern: 1},
	})

	// Data Left - Left-aligned data cell
	styles.DataLeft, _ = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Family: "Calibri", Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "left"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})

	// Data Grid - Right-aligned currency data cell
	styles.DataGrid, _ = f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Family: "Calibri", Size: 10},
		CustomNumFmt: func() *string { s := "$#,##0.00;$#,##0.00;$0.00"; return &s }(),
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})

	// Data Grid Bold - Bold currency data cell
	styles.DataGridBold, _ = f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Family: "Calibri", Size: 10},
		CustomNumFmt: func() *string { s := "$#,##0.00;$#,##0.00;$0.00"; return &s }(),
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})

	// Group Total - Subtotal row style
	styles.GroupTotal, _ = f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Family: "Calibri"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"#DAEEF3"}, Pattern: 1},
		CustomNumFmt: func() *string { s := "$#,##0.00;$#,##0.00;$0.00"; return &s }(),
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})

	// Profit - Profit section style
	styles.Profit, _ = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Family: "Calibri"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#c4f0ce"}, Pattern: 1},
	})

	// Profit Green - Profit value style (green text)
	styles.ProfitGreen, _ = f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Family: "Calibri", Color: "28a745"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"#c4f0ce"}, Pattern: 1},
		CustomNumFmt: func() *string { s := "$#,##0.00;$#,##0.00;$0.00"; return &s }(),
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 2},
			{Type: "top", Color: "000000", Style: 2},
			{Type: "bottom", Color: "000000", Style: 2},
			{Type: "right", Color: "000000", Style: 2},
		},
	})

	// Total Row - Bottom total row style
	styles.TotalRow, _ = f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E1E1E1"}, Pattern: 1},
	})

	// Total Currency - Total currency cell style
	styles.TotalCurrency, _ = f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"#F2F2F2"}, Pattern: 1},
		CustomNumFmt: func() *string { s := "$#,##0.00"; return &s }(),
	})

	// Normal Currency - Regular currency cell style
	styles.NormalCurrency, _ = f.NewStyle(&excelize.Style{
		CustomNumFmt: func() *string { s := "$#,##0.00"; return &s }(),
	})

	return styles
}

// ApplyHeaderBlueStyle applies the header blue style to a range
func ApplyHeaderBlueStyle(f *excelize.File, sheet, cell1, cell2 string, styles StyleSet) {
	f.SetCellStyle(sheet, cell1, cell2, styles.HeaderBlue)
}

// ApplyHeaderWhiteStyle applies the header white style to a range
func ApplyHeaderWhiteStyle(f *excelize.File, sheet, cell1, cell2 string, styles StyleSet) {
	f.SetCellStyle(sheet, cell1, cell2, styles.HeaderWhite)
}

// ApplySectionTitleStyle applies the section title style
func ApplySectionTitleStyle(f *excelize.File, sheet, cell string, styles StyleSet) {
	f.SetCellStyle(sheet, cell, cell, styles.SectionTitle)
}

// ApplyDataGridStyle applies the data grid style
func ApplyDataGridStyle(f *excelize.File, sheet, cell1, cell2 string, styles StyleSet) {
	f.SetCellStyle(sheet, cell1, cell2, styles.DataGrid)
}

// ApplyGroupTotalStyle applies the group total style
func ApplyGroupTotalStyle(f *excelize.File, sheet, cell1, cell2 string, styles StyleSet) {
	f.SetCellStyle(sheet, cell1, cell2, styles.GroupTotal)
}

// ApplyProfitStyle applies the profit style
func ApplyProfitStyle(f *excelize.File, sheet, cell1, cell2 string, styles StyleSet) {
	f.SetCellStyle(sheet, cell1, cell2, styles.Profit)
}

// ApplyProfitGreenStyle applies the profit green style
func ApplyProfitGreenStyle(f *excelize.File, sheet, cell string, styles StyleSet) {
	f.SetCellStyle(sheet, cell, cell, styles.ProfitGreen)
}
