package pl

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/business/clinic"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/xuri/excelize/v2"
)

type Service interface {
	GetMonthlySummary(ctx context.Context, f *PLFilter) ([]RsPLSummary, error)
	GetByAccount(ctx context.Context, f *PLFilter) ([]RsPLAccount, error)
	GetByResponsibility(ctx context.Context, f *PLFilter) ([]RsPLResponsibility, error)
	GetFYSummary(ctx context.Context, f *PLFilter) ([]RsPLFYSummary, error)
	GetReport(ctx context.Context, actorID uuid.UUID, f *PLReportFilter) (*RsReport, error)
	ExportPLReport(data *RsReport, exportType string) (interface{}, error)
}

type service struct {
	repo           Repository
	clinicRepo     clinic.Repository
	accountantRepo accountant.Repository

	practitionerSvc practitioner.IService
}

func NewService(repo Repository, clinicRepo clinic.Repository, accountantRepo accountant.Repository, practitionerSvc practitioner.IService) Service {
	return &service{repo: repo, clinicRepo: clinicRepo, accountantRepo: accountantRepo, practitionerSvc: practitionerSvc}
}

func (s *service) GetMonthlySummary(ctx context.Context, f *PLFilter) ([]RsPLSummary, error) {
	clinicID, err := parseAndValidate(f)
	if err != nil {
		return nil, err
	}

	rows, err := s.repo.GetMonthlySummary(ctx, clinicID, f)
	if err != nil {
		return nil, err
	}

	out := make([]RsPLSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ToRs())
	}
	return out, nil
}

func (s *service) GetByAccount(ctx context.Context, f *PLFilter) ([]RsPLAccount, error) {
	clinicID, err := parseAndValidate(f)
	if err != nil {
		return nil, err
	}

	rows, err := s.repo.GetByAccount(ctx, clinicID, f)
	if err != nil {
		return nil, err
	}

	out := make([]RsPLAccount, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ToRs())
	}
	return out, nil
}

func (s *service) GetByResponsibility(ctx context.Context, f *PLFilter) ([]RsPLResponsibility, error) {
	clinicID, err := parseAndValidate(f)
	if err != nil {
		return nil, err
	}

	rows, err := s.repo.GetByResponsibility(ctx, clinicID, f)
	if err != nil {
		return nil, err
	}

	out := make([]RsPLResponsibility, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ToRs())
	}
	return out, nil
}

func (s *service) GetFYSummary(ctx context.Context, f *PLFilter) ([]RsPLFYSummary, error) {
	clinicID, err := parseAndValidate(f)
	if err != nil {
		return nil, err
	}

	if f.FinancialYearID != nil {
		if _, err := uuid.Parse(*f.FinancialYearID); err != nil {
			return nil, fmt.Errorf("invalid financial_year_id: must be a valid UUID")
		}
	}

	rows, err := s.repo.GetFYSummary(ctx, clinicID, f)
	if err != nil {
		return nil, err
	}

	out := make([]RsPLFYSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ToRs())
	}
	return out, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

const dateLayout = "2006-01-02"

// parseAndValidate parses clinic_id and validates date range from the filter.
func parseAndValidate(f *PLFilter) (uuid.UUID, error) {
	clinicID, err := uuid.Parse(f.ClinicID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid clinic_id: must be a valid UUID")
	}

	var from, to time.Time

	if f.FromDate != nil {
		if from, err = time.Parse(dateLayout, *f.FromDate); err != nil {
			return uuid.Nil, fmt.Errorf("invalid from_date: use YYYY-MM-DD format")
		}
	}
	if f.ToDate != nil {
		if to, err = time.Parse(dateLayout, *f.ToDate); err != nil {
			return uuid.Nil, fmt.Errorf("invalid to_date: use YYYY-MM-DD format")
		}
	}
	if f.FromDate != nil && f.ToDate != nil && from.After(to) {
		return uuid.Nil, fmt.Errorf("from_date must not be after to_date")
	}

	return clinicID, nil
}

func (s *service) GetReport(ctx context.Context, actorID uuid.UUID, f *PLReportFilter) (*RsReport, error) {

	meta := auditctx.GetMetadata(ctx)

	isAccountant := false
	if meta.UserType != nil {
		isAccountant = strings.EqualFold(*meta.UserType, util.RoleAccountant)
	}

	accProfile, err := s.accountantRepo.GetAccountantByUserID(ctx, actorID.String())
	if err == nil && accProfile != nil {
		isAccountant = true
	}

	var finalOwnerID uuid.UUID

	if isAccountant {
		if accProfile == nil {
			return nil, fmt.Errorf("access denied: accountant profile not found")
		}

		if f.ClinicID != nil && *f.ClinicID != "" {
			clinicUUID, err := uuid.Parse(*f.ClinicID)
			if err != nil {
				return nil, fmt.Errorf("invalid clinic_id format")
			}
			permission, err := s.clinicRepo.GetAccountantPermission(ctx, accProfile.ID, clinicUUID)
			if err != nil {
				return nil, fmt.Errorf("permission denied: you are not associated with this clinic")
			}
			finalOwnerID = permission.PractitionerID
		} else {
			// Case B: Practice-wide
			if f.PractitionerID == "" {
				targetPracID, err := s.clinicRepo.GetPractitionerForAccountant(ctx, accProfile.ID)
				if err != nil {
					return nil, fmt.Errorf("no linked practitioner found: please provide a practitioner_id")
				}
				finalOwnerID = *targetPracID
			} else {
				targetPracID, err := uuid.Parse(f.PractitionerID)
				if err != nil {
					return nil, fmt.Errorf("invalid practitioner_id format")
				}
				isLinked, err := s.clinicRepo.IsAccountantInvitedByPractitioner(ctx, accProfile.ID, targetPracID)
				if err != nil || !isLinked {
					return nil, fmt.Errorf("permission denied: no association with this practitioner")
				}
				finalOwnerID = targetPracID
			}
		}
	} else {

		pracProfile, err := s.practitionerSvc.GetPractitionerByUserID(ctx, actorID.String())
		if err != nil {
			return nil, fmt.Errorf("access denied: practitioner profile not found")
		}
		finalOwnerID = pracProfile.ID

		// Verify clinic ownership if a specific one is requested
		if f.ClinicID != nil && *f.ClinicID != "" {
			clinicUUID, err := uuid.Parse(*f.ClinicID)
			if err != nil {
				return nil, fmt.Errorf("invalid clinic_id format")
			}

			_, err = s.clinicRepo.GetClinicByIDAndPractitioner(ctx, clinicUUID, finalOwnerID)
			if err != nil {
				return nil, fmt.Errorf("access denied: clinic not found or ownership mismatch")
			}
		}
	}

	// 3. APPLY VERIFIED PRACTITIONER ID (OUTSIDE all if/else blocks)
	f.PractitionerID = finalOwnerID.String()

	/*
		if f.ClinicID != nil {
			if _, err := uuid.Parse(*f.ClinicID); err != nil {
				return nil, fmt.Errorf("invalid clinic_id: must be a valid UUID")
			}
		}*/
	var from, to time.Time
	//var err error
	if f.DateFrom != nil {
		if from, err = time.Parse(dateLayout, *f.DateFrom); err != nil {
			return nil, fmt.Errorf("invalid date_from: use YYYY-MM-DD format")
		}
	}
	if f.DateUntil != nil {
		if to, err = time.Parse(dateLayout, *f.DateUntil); err != nil {
			return nil, fmt.Errorf("invalid date_until: use YYYY-MM-DD format")
		}
	}
	if f.DateFrom != nil && f.DateUntil != nil && from.After(to) {
		return nil, fmt.Errorf("date_from must not be after date_until")
	}

	rows, err := s.repo.GetReport(ctx, f)
	if err != nil {
		return nil, err
	}

	return buildReport(f, rows), nil
}

// buildReport assembles a flat P&L report aggregated across all clinics/forms,
// grouped by COA account within each section.
func buildReport(f *PLReportFilter, rows []*PLReportRow) *RsReport {
	// coaKey → accumulated total per section
	type coaKey struct {
		sectionType string
		coaID       string
	}
	coaOrder := map[string][]string{} // sectionType → ordered coaIDs
	coaSeen := map[coaKey]bool{}
	coaNames := map[coaKey]string{}
	coaTotals := map[coaKey]float64{}

	for _, r := range rows {
		// Treat NULL section_type as 'COST' (operating expenses)
		sectionType := "COST"
		if r.SectionType != nil {
			sectionType = *r.SectionType
		}

		// Use net_amount consistently across all sections for P&L reporting.
		// P&L should show revenue and expenses on a GST-exclusive basis:
		// - Income: NET (actual revenue earned, GST is collected for government)
		// - Costs: NET (actual expenses, GST can be claimed back)
		// This aligns with standard accounting practice where GST is a pass-through.
		val := r.NetAmount

		ck := coaKey{sectionType, r.CoaID}
		if !coaSeen[ck] {
			coaSeen[ck] = true
			coaOrder[sectionType] = append(coaOrder[sectionType], r.CoaID)
			coaNames[ck] = r.AccountName
		}
		coaTotals[ck] += val
	}

	buildGroup := func(sectionType string) RsReportGroup {
		accounts := make([]RsReportAccount, 0)
		var total float64
		for _, cid := range coaOrder[sectionType] {
			ck := coaKey{sectionType, cid}
			total += coaTotals[ck]
			accounts = append(accounts, RsReportAccount{
				CoaID:      cid,
				CoaName:    coaNames[ck],
				TotalValue: round2(coaTotals[ck]),
			})
		}
		return RsReportGroup{GroupTotal: round2(total), Accounts: accounts}
	}

	income := buildGroup("COLLECTION")
	cos := buildGroup("COST")
	other := buildGroup("OTHER_COST")

	grossProfit := round2(income.GroupTotal - cos.GroupTotal)
	netProfit := round2(grossProfit - other.GroupTotal)

	dateFrom := ""
	dateUntil := ""
	if f.DateFrom != nil {
		dateFrom = *f.DateFrom
	}
	if f.DateUntil != nil {
		dateUntil = *f.DateUntil
	}

	return &RsReport{
		ReportMetadata: RsReportMetadata{
			DateFrom:         dateFrom,
			DateUntil:        dateUntil,
			OverallNetProfit: netProfit,
		},
		Income:      income,
		CostOfSales: cos,
		GrossProfit: grossProfit,
		OtherCosts:  other,
		NetProfit:   netProfit,
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func (s *service) ExportPLReport(data *RsReport, exportType string) (interface{}, error) {
	f := excelize.NewFile()
	sheet := "Profit and Loss"
	f.NewSheet(sheet)
	f.DeleteSheet("Sheet1")

	// --- STYLES ---

	// Main Header
	styleHeaderBlue, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Family: "Calibri", Size: 14},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#DAEEF3"}, Pattern: 1},
	})

	// Section Title
	styleSectionTitle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Family: "Calibri", Size: 12},
	})

	// Style for Particulars/Names (Left Aligned)
	styleDataLeft, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Family: "Calibri", Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "left"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})

	// Data Cell Grid (Currency)
	styleDataGrid, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Family: "Calibri", Size: 10},
		CustomNumFmt: func() *string { s := "$#,##0.00;$#,##0.00;$0.00"; return &s }(),
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})

	// Group Total Style
	styleGroupTotal, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Family: "Calibri"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"#DAEEF3"}, Pattern: 1},
		CustomNumFmt: func() *string { s := "$#,##0.00;$#,##0.00;$0.00"; return &s }(),
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})

	// Final Profit Style
	styleProfit, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Family: "Calibri"},
		CustomNumFmt: func() *string { s := "$#,##0.00;$#,##0.00;$0.00"; return &s }(),
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"#c4f0ce"}, Pattern: 1},
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 2}, {Type: "top", Color: "000000", Style: 2},
			{Type: "bottom", Color: "000000", Style: 2}, {Type: "right", Color: "000000", Style: 2},
		},
	})

	styleProfitGreen, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Family: "Calibri", Color: "28a745"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"#c4f0ce"}, Pattern: 1},
		CustomNumFmt: func() *string { s := "$#,##0.00;$#,##0.00;$0.00"; return &s }(),
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 2}, {Type: "top", Color: "000000", Style: 2},
			{Type: "bottom", Color: "000000", Style: 2}, {Type: "right", Color: "000000", Style: 2},
		},
	})

	// --- RENDER HEADERS ---
	f.SetCellValue(sheet, "A1", "Profit and Loss Report")
	f.MergeCell(sheet, "A1", "B1")
	f.SetCellStyle(sheet, "A1", "B1", styleHeaderBlue)

	currentRow := 3 // Default start if no date
	if data.ReportMetadata.DateFrom != "" && data.ReportMetadata.DateUntil != "" {
		f.SetCellValue(sheet, "A2", fmt.Sprintf("Period: %s to %s", data.ReportMetadata.DateFrom, data.ReportMetadata.DateUntil))
		currentRow = 4
	}

	var totalIncomeCell, totalCOSCell, totalOtherCostsCell string

	// Helper closure to render sections
	renderGroup := func(title string, group RsReportGroup) string {

		f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), title)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleSectionTitle)

		// Set the table filter
		if len(group.Accounts) > 0 {
			tableRange := fmt.Sprintf("A%d:A%d", currentRow, currentRow+len(group.Accounts))
			tableName := strings.ReplaceAll(title, " ", "_") + fmt.Sprintf("_%d", currentRow)

			showHeaders := true
			f.AddTable(sheet, &excelize.Table{
				Range:         tableRange,
				Name:          tableName,
				StyleName:     "", // Keeps your custom colors
				ShowHeaderRow: &showHeaders,
			})
		}

		currentRow++

		dataStartRow := currentRow
		for _, acc := range group.Accounts {
			// Column A: Account Name
			f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), acc.CoaName)
			f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleDataLeft)

			// Column B: Total Value
			f.SetCellValue(sheet, fmt.Sprintf("B%d", currentRow), acc.TotalValue)
			f.SetCellStyle(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("B%d", currentRow), styleDataGrid)
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

		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("B%d", currentRow), styleGroupTotal)
		currentRow += 2

		return totalCell
	}

	// --- DATA SECTIONS ---
	totalIncomeCell = renderGroup("INCOME", data.Income)
	totalCOSCell = renderGroup("COST OF SALES", data.CostOfSales)

	// --- GROSS PROFIT (Dynamic) ---
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "GROSS PROFIT")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleProfit)

	// Formula: Total Income - Cost of Sales
	f.SetCellFormula(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("%s-%s", totalIncomeCell, totalCOSCell))
	f.SetCellStyle(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("B%d", currentRow), styleProfitGreen)
	grossProfitCell := fmt.Sprintf("B%d", currentRow)
	currentRow += 2

	totalOtherCostsCell = renderGroup("OTHER COSTS", data.OtherCosts)

	// --- NET PROFIT (Dynamic) ---
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "NET PROFIT")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleProfit)

	// Formula: Gross Profit - Other Costs
	f.SetCellFormula(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("%s-%s", grossProfitCell, totalOtherCostsCell))
	f.SetCellStyle(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("B%d", currentRow), styleProfitGreen)

	// --- FORMATTING ---
	f.SetColWidth(sheet, "A", "A", 45)
	f.SetColWidth(sheet, "B", "B", 20)
	f.UpdateLinkedValue()

	// if exportType == "pdf" {
	// 	return s.convertExcelToPDF(f, sheet)
	// }

	if exportType == "pdf" {
		return s.convertExcelToPDF(f, sheet, data) // Pass the API data here
	}

	return f, nil
}

func (s *service) convertExcelToPDF(f *excelize.File, sheetName string, data *RsReport) ([]byte, error) {
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer
	b.WriteString("<html><head><style>")
	b.WriteString(`
		@page { size: A4; margin: 1cm; }
		body { font-family: 'Calibri', sans-serif; margin: 0; padding: 20px; color: #333; }
		table { width: 100%; border-collapse: collapse; table-layout: fixed; }
		td { padding: 6px 8px; font-size: 10pt; vertical-align: middle; }
		.header-blue { background-color: #DAEEF3 !important; font-weight: bold; font-size: 14pt; text-align: center; border: 1px solid #000; }
		.period-text { font-size: 10pt; padding: 10px 0; font-style: italic; }
		.section-title { font-weight: bold; font-size: 12pt; padding-top: 15px; }
		.data-left { border: 0.5pt solid #000; text-align: left; }
		.data-grid { border: 0.5pt solid #000; text-align: right; }
		.group-total { background-color: #DAEEF3 !important; font-weight: bold; text-align: right; border: 0.5pt solid #000; }
		.profit-label { background-color: #c4f0ce !important; font-weight: bold; border: 1.5pt solid #000; }
		.profit-value { background-color: #c4f0ce !important; font-weight: bold; color: #28a745; text-align: right; border: 1.5pt solid #000; }
		.spacer { height: 15px; border: none; }
	`)
	b.WriteString("</style></head><body><table>")
	b.WriteString("<colgroup><col style='width: 70%;'><col style='width: 30%;'></colgroup>")

	// Helper to format currency
	formatCurr := func(v float64) string {
		return fmt.Sprintf("$%.2f", v)
	}

	// Calculate totals from API data for PDF display
	calcTotal := func(accounts []RsReportAccount) float64 {
		var t float64
		for _, a := range accounts {
			t += a.TotalValue
		}
		return t
	}

	totalInc := calcTotal(data.Income.Accounts)
	totalCOS := calcTotal(data.CostOfSales.Accounts)
	totalOther := calcTotal(data.OtherCosts.Accounts)
	grossProfit := totalInc - totalCOS
	netProfit := grossProfit - totalOther

	for rIdx, row := range rows {
		rowNum := rIdx + 1
		if len(row) == 0 {
			b.WriteString("<tr><td colspan='2' class='spacer'></td></tr>")
			continue
		}

		valA := row[0]
		var valB string
		classA, classB := "", ""

		// Identify the row type and override valB with API data
		switch {
		case rowNum == 1:
			classA = "header-blue"
			b.WriteString(fmt.Sprintf("<tr><td colspan='2' class='%s'>%s</td></tr>", classA, valA))
			continue

		case strings.HasPrefix(valA, "Period:"):
			classA = "period-text"

		case valA == "INCOME" || valA == "COST OF SALES" || valA == "OTHER COSTS":
			classA = "section-title"

		case valA == "TOTAL INCOME":
			classA, classB = "group-total", "group-total"
			valB = formatCurr(totalInc)

		case valA == "TOTAL COST OF SALES":
			classA, classB = "group-total", "group-total"
			valB = formatCurr(totalCOS)

		case valA == "TOTAL OTHER COSTS":
			classA, classB = "group-total", "group-total"
			valB = formatCurr(totalOther)

		case valA == "GROSS PROFIT":
			classA, classB = "profit-label", "profit-value"
			valB = formatCurr(grossProfit)

		case valA == "NET PROFIT":
			classA, classB = "profit-label", "profit-value"
			valB = formatCurr(netProfit)

		default:
			classA, classB = "data-left", "data-grid"
			if len(row) > 1 {
				valB = row[1]
			} // Account values are already static in Excel rows
		}

		b.WriteString(fmt.Sprintf("<tr><td class='%s'>%s</td>", classA, valA))
		b.WriteString(fmt.Sprintf("<td class='%s'>%s</td></tr>", classB, valB))
	}

	b.WriteString("</table></body></html>")

	// Render via Chromedp
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()
	var buf []byte
	err = chromedp.Run(ctx,
		chromedp.Navigate("about:blank"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			lTree, _ := page.GetFrameTree().Do(ctx)
			return page.SetDocumentContent(lTree.Frame.ID, b.String()).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			buf, _, err = page.PrintToPDF().WithPrintBackground(true).Do(ctx)
			return err
		}),
	)
	return buf, err
}
