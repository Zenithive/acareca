package entry

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/iamarpitzala/acareca/internal/shared/export"
	lo "github.com/samber/lo"
	"github.com/xuri/excelize/v2"
)

type CoaGroup struct {
	CoaID            string       `json:"coa_id"`
	CoaName          string       `json:"coa_name"`
	TotalNetAmount   float64      `json:"total_net_amount"`
	TotalGrossAmount float64      `json:"total_gross_amount"`
	Details          []*CoaDetail `json:"details"`
}

type CoaDetail struct {
	FormFieldName      string    `json:"form_field_name"`
	TaxTypeName        *string   `json:"tax_type_name"`
	FormName           string    `json:"form_name"`
	ClinicName         string    `json:"clinic_name"`
	NetAmount          *float64  `json:"net_amount"`
	GstAmount          *float64  `json:"gst_amount"`
	GrossAmount        *float64  `json:"gross_amount"`
	CreatedAt          time.Time `json:"created_at"`
	IsExpense          bool      `json:"is_expense"`
	BusinessPercentage *float64  `json:"business_percentage"`
	Notes              *string   `json:"notes"`
}

type ColumnDefinition struct {
	Header string
	Key    string
	Width  float64
}

func GenerateExcelReport(groups []*CoaGroup, config export.ExportConfig, formatDateFn func(string) string, selectedKeys []string) (*bytes.Buffer, error) {
	catalog := map[string]ColumnDefinition{
		"date":                {Header: "Date", Key: "date", Width: 15},
		"supplier_name":       {Header: "Supplier Name", Key: "supplier_name", Width: 30},
		"description":         {Header: "Description / Label", Key: "description", Width: 30},
		"clinic":              {Header: "Clinic", Key: "clinic", Width: 30},
		"expenses":            {Header: "Expenses", Key: "expenses", Width: 15},
		"net_amount":          {Header: "Net Amount", Key: "net_amount", Width: 16},
		"gst_amount":          {Header: "GST Amount", Key: "gst_amount", Width: 16},
		"gross_amount":        {Header: "Gross Amount", Key: "gross_amount", Width: 16},
		"gst_type":            {Header: "GST Type", Key: "gst_type", Width: 16},
		"business_percentage": {Header: "Business Percentage", Key: "business_percentage", Width: 20},
		"note":                {Header: "Note", Key: "note", Width: 30},
	}

	var enabledCols []ColumnDefinition
	for _, key := range selectedKeys {
		subKeys := []string{key}
		if strings.Contains(key, ",") {
			subKeys = strings.Split(key, ",")
		}

		for _, subKey := range subKeys {
			cleanKey := strings.TrimSpace(strings.ToLower(subKey))
			if cleanKey == "notes" {
				cleanKey = "note"
			}
			if col, exists := catalog[cleanKey]; exists {
				enabledCols = append(enabledCols, col)
			}
		}
	}

	if len(enabledCols) == 0 {
		enabledCols = []ColumnDefinition{
			catalog["date"], catalog["supplier_name"], catalog["description"],
			catalog["clinic"], catalog["expenses"], catalog["net_amount"],
			catalog["gst_amount"], catalog["gross_amount"], catalog["gst_type"],
			catalog["business_percentage"], catalog["note"],
		}
	}

	xl := excelize.NewFile()
	defer xl.Close()
	sheet := "Transactions"
	xl.SetSheetName("Sheet1", sheet)

	lastColLetter, _ := excelize.ColumnNumberToName(len(enabledCols))
	if lastColLetter == "" {
		lastColLetter = "K"
	}

	styleHeaderBlue, _ := xl.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14, Color: "FFFFFF", Family: "Segoe UI"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1F4E78"}, Pattern: 1},
	})
	headerStyle, _ := xl.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10, Color: "FFFFFF", Family: "Segoe UI"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"1F4E78"}, Pattern: 1},
	})
	groupHeaderStyle, _ := xl.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10, Family: "Segoe UI", Color: "1F4E78"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"DAEEF3"}, Pattern: 1},
	})
	normalCurrencyStyle, _ := xl.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Size: 10, Family: "Segoe UI"},
		CustomNumFmt: lo.ToPtr("$#,##0.00;($#,##0.00);$0.00"),
	})
	percentStyle, _ := xl.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Size: 10, Family: "Segoe UI"},
		CustomNumFmt: lo.ToPtr("0.00%"),
	})
	totalRowStyle, _ := xl.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10, Family: "Segoe UI"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"F2F2F2"}, Pattern: 1},
	})
	totalCurrencyStyle, _ := xl.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Size: 10, Family: "Segoe UI", Color: "1F4E78"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"F2F2F2"}, Pattern: 1},
		CustomNumFmt: lo.ToPtr("$#,##0.00;($#,##0.00);$0.00"),
	})

	getFloat := func(f *float64) float64 { return export.GetFloatValue(f) }
	getString := func(s *string) string { return export.GetStringValue(s) }

	xl.MergeCell(sheet, "A1", lastColLetter+"1")
	xl.SetCellValue(sheet, "A1", "Transaction Report")
	xl.SetCellStyle(sheet, "A1", "A1", styleHeaderBlue)

	setRichMeta := func(row int, label, value string) {
		cell := fmt.Sprintf("A%d", row)
		xl.MergeCell(sheet, cell, lastColLetter+strconv.Itoa(row))
		xl.SetCellRichText(sheet, cell, []excelize.RichTextRun{
			{Text: label, Font: &excelize.Font{Bold: true, Family: "Segoe UI", Size: 10, Color: "595959"}},
			{Text: " " + value, Font: &excelize.Font{Bold: false, Family: "Segoe UI", Size: 10, Color: "262626"}},
		})
	}

	metaRow := 2
	setRichMeta(metaRow, "Exported by:", config.EntityName)
	metaRow++

	if config.EntityABN != "" {
		setRichMeta(metaRow, "ABN:", config.EntityABN)
		metaRow++
	}
	if config.Period != "" {
		setRichMeta(metaRow, "Period:", config.Period)
		metaRow++
	}
	setRichMeta(metaRow, "Generated:", config.GeneratedTime)
	metaRow++

	xl.MergeCell(sheet, fmt.Sprintf("A%d", metaRow), fmt.Sprintf("%s%d", lastColLetter, metaRow))
	metaRow++

	headerRow := metaRow
	for idx, col := range enabledCols {
		cell, _ := excelize.CoordinatesToCellName(idx+1, headerRow)
		xl.SetCellValue(sheet, cell, col.Header)
	}
	xl.SetCellStyle(sheet, "A"+strconv.Itoa(headerRow), lastColLetter+strconv.Itoa(headerRow), headerStyle)

	currRow := headerRow + 1
	for _, g := range groups {
		xl.SetCellValue(sheet, fmt.Sprintf("A%d", currRow), g.CoaName)
		xl.MergeCell(sheet, fmt.Sprintf("A%d", currRow), fmt.Sprintf("%s%d", lastColLetter, currRow))
		xl.SetCellStyle(sheet, fmt.Sprintf("A%d", currRow), fmt.Sprintf("%s%d", lastColLetter, currRow), groupHeaderStyle)
		currRow++

		for _, d := range g.Details {
			for cIdx, col := range enabledCols {
				cell, _ := excelize.CoordinatesToCellName(cIdx+1, currRow)

				switch col.Key {
				case "date":
					xl.SetCellValue(sheet, cell, formatDateFn(d.CreatedAt.Format("2006-01-02")))
				case "supplier_name":
					xl.SetCellValue(sheet, cell, d.FormName)
				case "description":
					xl.SetCellValue(sheet, cell, "  "+d.FormFieldName)
				case "clinic":
					xl.SetCellValue(sheet, cell, d.ClinicName)
				case "expenses":
					if d.IsExpense {
						xl.SetCellValue(sheet, cell, "Yes")
					} else {
						xl.SetCellValue(sheet, cell, "No")
					}
				case "net_amount":
					xl.SetCellValue(sheet, cell, getFloat(d.NetAmount))
					xl.SetCellStyle(sheet, cell, cell, normalCurrencyStyle)
				case "gst_amount":
					xl.SetCellValue(sheet, cell, getFloat(d.GstAmount))
					xl.SetCellStyle(sheet, cell, cell, normalCurrencyStyle)
				case "gross_amount":
					xl.SetCellValue(sheet, cell, getFloat(d.GrossAmount))
					xl.SetCellStyle(sheet, cell, cell, normalCurrencyStyle)
				case "gst_type":
					xl.SetCellValue(sheet, cell, getString(d.TaxTypeName))
				case "business_percentage":
					pct := lo.FromPtrOr(d.BusinessPercentage, 100.0)
					xl.SetCellValue(sheet, cell, pct/100.0)
					xl.SetCellStyle(sheet, cell, cell, percentStyle)
				case "note":
					xl.SetCellValue(sheet, cell, lo.FromPtrOr(d.Notes, ""))
				}
			}
			currRow++
		}

		xl.SetCellStyle(sheet, fmt.Sprintf("A%d", currRow), fmt.Sprintf("%s%d", lastColLetter, currRow), totalRowStyle)

		for cIdx, col := range enabledCols {
			cell, _ := excelize.CoordinatesToCellName(cIdx+1, currRow)
			if cIdx == 0 {
				xl.SetCellValue(sheet, cell, "Total "+g.CoaName)
			}

			switch col.Key {
			case "net_amount":
				xl.SetCellValue(sheet, cell, g.TotalNetAmount)
				xl.SetCellStyle(sheet, cell, cell, totalCurrencyStyle)
			case "gst_amount":
				xl.SetCellValue(sheet, cell, g.TotalGrossAmount-g.TotalNetAmount)
				xl.SetCellStyle(sheet, cell, cell, totalCurrencyStyle)
			case "gross_amount":
				xl.SetCellValue(sheet, cell, g.TotalGrossAmount)
				xl.SetCellStyle(sheet, cell, cell, totalCurrencyStyle)
			}
		}
		currRow += 2
	}

	if err := xl.AutoFilter(sheet, fmt.Sprintf("A%d:%s%d", headerRow, lastColLetter, headerRow), nil); err != nil {
		return nil, err
	}

	for idx, col := range enabledCols {
		colLetter, _ := excelize.ColumnNumberToName(idx + 1)
		xl.SetColWidth(sheet, colLetter, colLetter, col.Width)
	}

	return xl.WriteToBuffer()
}
