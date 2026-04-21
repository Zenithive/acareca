package bas

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/business/clinic"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/xuri/excelize/v2"
)

// Service defines the business-logic layer for the BAS module.
type Service interface {
	GetQuarterlySummary(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]RsBASSummary, error)
	GetByAccount(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]RsBASByAccount, error)
	GetMonthly(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]RsBASMonthly, error)
	GetReport(ctx context.Context, f *BASReportFilter) (*RsBASReport, error)
	GetBASPreparation(ctx context.Context, actorID uuid.UUID, f *BASFilter) (*RsBASPreparation, error)
	ExportActivityStatement(ctx context.Context, quarters []QuarterData, prevDates PeriodInfo) (*bytes.Buffer, error)
	GetPeriodDates(ctx context.Context, f *BASReportFilter) (curr PeriodInfo, prev PeriodInfo, err error)
	GetAllQuartersInYear(ctx context.Context, quarterID uuid.UUID) ([]BASQuarterInfo, error)
	// ExportBASPreparation(ctx context.Context, actorID uuid.UUID, f *BASFilter, w io.Writer) error
	// ExportBASPreparation(ctx context.Context, actorID uuid.UUID, f *BASFilter, w io.Writer) error
	ExportBASPreparation(data *RsBASPreparation) (*excelize.File, error)
}

type service struct {
	repo           Repository
	accountantRepo accountant.Repository
	auditSvc       audit.Service
	clinicRepo     clinic.Repository
}

func NewService(repo Repository, accountantRepo accountant.Repository, auditSvc audit.Service, clinicRepo clinic.Repository) Service {
	return &service{repo: repo, accountantRepo: accountantRepo, auditSvc: auditSvc, clinicRepo: clinicRepo}
}

func (s *service) GetQuarterlySummary(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]RsBASSummary, error) {
	if err := validateDateFilter(f); err != nil {
		return nil, err
	}
	if err := validateFYID(f); err != nil {
		return nil, err
	}

	rows, err := s.repo.GetQuarterlySummary(ctx, clinicID, f)
	if err != nil {
		return nil, err
	}

	out := make([]RsBASSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ToRs())
	}
	return out, nil
}

func (s *service) GetByAccount(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]RsBASByAccount, error) {
	if err := validateDateFilter(f); err != nil {
		return nil, err
	}
	if err := validateFYID(f); err != nil {
		return nil, err
	}

	rows, err := s.repo.GetByAccount(ctx, clinicID, f)
	if err != nil {
		return nil, err
	}

	out := make([]RsBASByAccount, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ToRs())
	}
	return out, nil
}

func (s *service) GetMonthly(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]RsBASMonthly, error) {
	if err := validateDateFilter(f); err != nil {
		return nil, err
	}

	rows, err := s.repo.GetMonthly(ctx, clinicID, f)
	if err != nil {
		return nil, err
	}

	out := make([]RsBASMonthly, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ToRs())
	}
	return out, nil
}

func validateFYID(f *BASFilter) error {
	if f.FinancialYearID != nil {
		if _, err := parseUUID(*f.FinancialYearID); err != nil {
			return fmt.Errorf("invalid financial_year_id: must be a valid UUID")
		}
	}
	return nil
}

func parseUUID(s string) ([16]byte, error) {
	var id [16]byte
	parsed, err := uuid.Parse(s)
	if err != nil {
		return id, err
	}
	return parsed, nil
}

func (s *service) GetReport(ctx context.Context, f *BASReportFilter) (*RsBASReport, error) {
	pracID, err := uuid.Parse(f.PractitionerID)
	if err != nil {
		return nil, fmt.Errorf("invalid practitioner_id")
	}

	var from, to string

	switch {
	case f.QuarterID != nil:
		qID, err := uuid.Parse(*f.QuarterID)
		if err != nil {
			return nil, fmt.Errorf("invalid quarter_id: must be a valid UUID")
		}
		from, to, err = s.repo.GetQuarterDates(ctx, qID)
		if err != nil {
			return nil, err
		}

	case f.Month != nil:
		start, end, err := util.GetMonthRange(*f.Month)
		if err != nil {
			return nil, fmt.Errorf("invalid month: use full month name e.g. January")
		}
		from = start.Format("2006-01-02")
		to = end.Format("2006-01-02")

	default:
		return nil, fmt.Errorf("provide either quarter_id or month filter")
	}

	row, err := s.repo.GetReport(ctx, pracID, from, to)
	if err != nil {
		return nil, err
	}

	return &RsBASReport{
		G1:  row.G1TotalSalesGross,
		A1:  row.Label1AGSTOnSales,
		G11: row.G11TotalPurchasesGross,
		B1:  row.Label1BGSTOnPurchases,
	}, nil
}

func (s *service) GetBASPreparation(ctx context.Context, actorID uuid.UUID, f *BASFilter) (*RsBASPreparation, error) {
	meta := auditctx.GetMetadata(ctx)

	isAccountant := false
	if meta.UserType != nil {
		isAccountant = strings.EqualFold(*meta.UserType, util.RoleAccountant)
	} else {

		acc, err := s.accountantRepo.GetAccountantByUserID(ctx, actorID.String())
		if err == nil && acc != nil {
			isAccountant = true
		}
	}

	var ownerID uuid.UUID
	var clinicIDs []uuid.UUID

	// Convert BASFilter to common.Filter for clinic listing
	commonFilter := f.MapToFilter()

	// Use clinic_id array from BASFilter
	requestedClinicIDs := f.ParsedClinicIDs

	if isAccountant {

		accProfile, err := s.accountantRepo.GetAccountantByUserID(ctx, actorID.String())
		if err != nil {
			return nil, fmt.Errorf("access denied: accountant profile not found")
		}

		// If clinic_ids are provided, verify permission for each clinic
		if len(requestedClinicIDs) > 0 {
			for _, clinicID := range requestedClinicIDs {
				permission, err := s.clinicRepo.GetAccountantPermission(ctx, accProfile.ID, clinicID)
				if err != nil {
					return nil, fmt.Errorf("permission denied for clinic %s", clinicID)
				}
				ownerID = permission.PractitionerID
				clinicIDs = append(clinicIDs, clinicID)
			}
		} else {
			// If no clinic_ids provided, get all clinics the accountant has access to
			clinics, err := s.clinicRepo.ListClinicByAccountant(ctx, accProfile.ID, commonFilter)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch clinics: %w", err)
			}
			if len(clinics) == 0 {
				return nil, fmt.Errorf("no clinics found for this accountant")
			}
			// Use the first clinic's practitioner as owner (they should all belong to same practitioner)
			ownerID = clinics[0].PractitionerID
			for _, clinic := range clinics {
				clinicIDs = append(clinicIDs, clinic.ID)
			}
		}
	} else {
		pID, er := s.clinicRepo.GetPractitionerIDByUserID(ctx, actorID.String())
		if er != nil {
			return nil, fmt.Errorf("practitioner profile not found")
		}
		ownerID = *pID

		if len(requestedClinicIDs) > 0 {
			// Verify the practitioner owns each requested clinic
			for _, clinicID := range requestedClinicIDs {
				_, err := s.clinicRepo.GetClinicByIDAndPractitioner(ctx, clinicID, ownerID)
				if err != nil {
					return nil, fmt.Errorf("clinic %s not found or access denied", clinicID.String())
				}
				clinicIDs = append(clinicIDs, clinicID)
			}
		} else {
			// Get all clinics for this practitioner
			clinics, err := s.clinicRepo.ListClinicByPractitioner(ctx, ownerID, commonFilter)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch clinics: %w", err)
			}
			if len(clinics) == 0 {
				return nil, fmt.Errorf("no clinics found for this practitioner")
			}
			for _, clinic := range clinics {
				clinicIDs = append(clinicIDs, clinic.ID)
			}
		}
	}

	// Aggregate data from all relevant clinics
	var allRows []*BASLineItemRow
	for _, cID := range clinicIDs {
		rows, err := s.repo.GetBASLineItems(ctx, cID, f)
		if err != nil {
			return nil, err
		}
		allRows = append(allRows, rows...)
	}

	quarterGroups := make(map[string][]*BASLineItemRow)
	for _, r := range allRows {
		k := r.PeriodQuarter.Format("2006-01-02")
		quarterGroups[k] = append(quarterGroups[k], r)
	}

	resp := &RsBASPreparation{Columns: []BASColumn{}}
	var finalizedRowsForTotal []*BASLineItemRow

	// --- Iterate over SELECTED Quarters first ---
	if len(f.ParsedQuarterIDs) > 0 {
		for _, qID := range f.ParsedQuarterIDs {

			// Get metadata by ID (Always works even if no transactions)
			qInfo, err := s.repo.GetQuarterInfoByID(ctx, qID)
			if err != nil {
				continue
			}

			// Get data from our map (might be nil/empty)
			qRows := quarterGroups[qInfo.StartDate]

			finalizedRowsForTotal = append(finalizedRowsForTotal, qRows...)

			// Map to column (mapToBASColumn handles nil/empty rows by returning $0)
			col := s.mapToBASColumn(qRows)
			col.Quarter = *qInfo
			resp.Columns = append(resp.Columns, col)
		}
	} else {
		// Fallback for when no specific quarters are selected (Show what exists)
		for key, qRows := range quarterGroups {
			finalizedRowsForTotal = append(finalizedRowsForTotal, qRows...)

			col := s.mapToBASColumn(qRows)
			quarterDate, _ := time.Parse("2006-01-02", key)
			qInfo, _ := s.repo.GetQuarterInfoByDate(ctx, quarterDate)
			if qInfo != nil {
				col.Quarter = *qInfo
			}
			resp.Columns = append(resp.Columns, col)
		}
	}

	// --- CRITICAL SORTING STEP ---
	// This ensures Q1 comes before Q2, even if Q3 is missing.
	sort.Slice(resp.Columns, func(i, j int) bool {
		return resp.Columns[i].Quarter.StartDate < resp.Columns[j].Quarter.StartDate
	})

	// Build Grand Total last
	resp.GrandTotal = s.mapToBASColumn(finalizedRowsForTotal)
	resp.GrandTotal.Quarter.Name = "Total"

	return resp, nil
}

func (s *service) mapToBASColumn(rows []*BASLineItemRow) BASColumn {
	var col BASColumn
	col.Sections.Income.Items = make([]BASLineItem, 0)
	col.Sections.Expenses.Items = make([]BASLineItem, 0)

	type incomeAcc struct {
		Name    string
		Amounts BASAmount
	}
	incomeOrder := []string{} // To maintain order of appearance
	incomeAccounts := map[string]*incomeAcc{}

	var b1 BASAmount
	var mgtFee, labWork, otherExp BASAmount

	for _, r := range rows {
		if BASCategory(r.BasCategory) == BASCategoryBASExcluded {
			continue
		}

		// Handle NULL section_type as expense (matches vw_bas_summary logic)
		sectionType := ""
		if r.SectionType != nil {
			sectionType = *r.SectionType
		}

		switch sectionType {
		case "COLLECTION":

			//  Accumulate Individual COA Totals for Display
			if _, seen := incomeAccounts[r.CoaID]; !seen {
				incomeOrder = append(incomeOrder, r.CoaID)
				incomeAccounts[r.CoaID] = &incomeAcc{Name: r.AccountName}
			}
			incomeAccounts[r.CoaID].Amounts.Gross += r.GrossAmount
			incomeAccounts[r.CoaID].Amounts.GST += r.GstAmount
			incomeAccounts[r.CoaID].Amounts.Net += r.NetAmount

		case "COST", "OTHER_COST", "":

			b1.Gross += r.GstAmount

			// Categorize by Account Name, not by BAS Category
			accName := strings.ToLower(r.AccountName)
			switch {
			case strings.Contains(accName, "management"):
				mgtFee.Gross += r.GrossAmount
				mgtFee.GST += r.GstAmount
				mgtFee.Net += r.NetAmount
			case strings.Contains(accName, "lab"): // Catch "Lab Fees" and "Laboratory"
				labWork.Gross += r.GrossAmount
				labWork.GST += r.GstAmount
				labWork.Net += r.NetAmount
			default:
				// Captures Merchant Fees, Bank Fees, and other overheads
				otherExp.Gross += r.GrossAmount
				otherExp.GST += r.GstAmount
				otherExp.Net += r.NetAmount
			}
		}
	}

	// Helper to finalize a BASAmount with rounding
	finalize := func(amt BASAmount) BASAmount {
		return BASAmount{
			Gross: roundToTwo(amt.Gross),
			GST:   roundToTwo(amt.GST),
			Net:   roundToTwo(amt.Net),
		}
	}

	// --- 1. Income Section Construction & Calculation ---
	var totalIncome BASAmount
	for _, cid := range incomeOrder {
		acc := incomeAccounts[cid]
		finalAmounts := finalize(acc.Amounts)

		// Add to display items
		col.Sections.Income.Items = append(col.Sections.Income.Items, BASLineItem{
			Name:    acc.Name,
			Amounts: finalAmounts,
		})

		// Sum up for Net Profit/Loss calculation
		totalIncome.Gross += finalAmounts.Gross
		totalIncome.GST += finalAmounts.GST
		totalIncome.Net += finalAmounts.Net
	}

	// Ensure total is rounded
	totalIncome = finalize(totalIncome)

	// Expenses Section
	mgtFee = finalize(mgtFee)
	labWork = finalize(labWork)
	otherExp = finalize(otherExp)

	subtotalExpenses := BASAmount{
		Gross: roundToTwo(mgtFee.Gross + labWork.Gross + otherExp.Gross),
		GST:   roundToTwo(mgtFee.GST + labWork.GST + otherExp.GST),
		Net:   roundToTwo(mgtFee.Net + labWork.Net + otherExp.Net),
	}

	col.Sections.Expenses.Items = []BASLineItem{
		{Name: "Management Fee (Gross Up)", Amounts: mgtFee},
		{Name: "Laboratory Work (GST Free)", Amounts: labWork},
		{Name: "Other Expenses (GST)", Amounts: otherExp},
	}

	// Net Profit/Loss
	col.Sections.NetProfitLoss.Items = []BASLineItem{
		{
			Name: "Net Profit/Loss",
			Amounts: BASAmount{
				Net: roundToTwo(totalIncome.Net - subtotalExpenses.Net),
			},
		},
	}

	// Totals
	// Net GST Payable = 1A (GST on taxable sales) - 1B (GST on purchases)
	col.NetGSTPayable = roundToTwo(0 - b1.Gross)

	return col
}

// Helper to round values after calculation
func roundToTwo(val float64) float64 {
	return math.Round(val*100) / 100
}

func ptrString(s string) *string {
	return &s
}

type QuarterData struct {
	Period PeriodInfo
	Report *RsBASReport
}

func (s *service) ExportActivityStatement(ctx context.Context, quarters []QuarterData, prevDates PeriodInfo) (*bytes.Buffer, error) {
	xl := excelize.NewFile()
	defer xl.Close()

	sheet := "Activity Statement"
	dataSheet := "SourceData"
	xl.SetSheetName("Sheet1", sheet)
	xl.NewSheet(dataSheet)

	// --- 1. Populate Hidden Data Sheet (The Lookup Table) ---
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

	// --- 2. Styles ---
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
	currencyStyle, _ := xl.NewStyle(&excelize.Style{CustomNumFmt: ptrString("$#,##0.00")})
	totalRowStyle, _ := xl.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#4EA7B3"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})

	// --- 3. Main Header & Quarter Dropdown (Simplified to single column) ---
	xl.SetCellValue(sheet, "A1", "Activity Statement Information")
	xl.SetCellValue(sheet, "B1", "BAS") // Renamed from "Current BAS"
	xl.SetCellStyle(sheet, "A1", "B1", headerStyle)

	// Create Dropdown in Cell B4
	var qLabels []string
	for _, q := range quarters {
		qLabels = append(qLabels, q.Period.Label)
	}
	dv := excelize.NewDataValidation(true)
	dv.Sqref = "B4"
	dv.SetDropList(qLabels)
	xl.AddDataValidation(sheet, dv)

	if len(qLabels) > 0 {
		xl.SetCellValue(sheet, "B4", qLabels[0])
	}

	// --- 4. Information Section ---
	xl.SetCellValue(sheet, "A2", "Period start")
	xl.SetCellFormula(sheet, "B2", fmt.Sprintf("=VLOOKUP(B4, %s!A:G, 6, FALSE)", dataSheet))

	xl.SetCellValue(sheet, "A3", "Period end")
	xl.SetCellFormula(sheet, "B3", fmt.Sprintf("=VLOOKUP(B4, %s!A:G, 7, FALSE)", dataSheet))

	xl.SetCellValue(sheet, "A4", "Qtr")
	xl.SetCellStyle(sheet, "A2", "A4", labelStyle)

	// --- 5. GST Section ---
	gstFields := []struct {
		Label string
		Col   int
	}{
		{"G1 (Total Sales)", 2},
		{"1A (GST on Sales)", 3},
		{"G11 (Total Purchases)", 4},
		{"1B (GST on Purchases)", 5},
	}

	rowIdx := 6
	for _, f := range gstFields {
		xl.SetCellValue(sheet, "A"+strconv.Itoa(rowIdx), f.Label)
		xl.SetCellFormula(sheet, "B"+strconv.Itoa(rowIdx), fmt.Sprintf("=VLOOKUP(B4, %s!A:G, %d, FALSE)", dataSheet, f.Col))
		xl.SetCellStyle(sheet, "B"+strconv.Itoa(rowIdx), "B"+strconv.Itoa(rowIdx), currencyStyle)
		rowIdx++
	}

	// --- 6. PAYG Tax Withheld Section ---
	rowIdx++
	xl.SetCellValue(sheet, "A"+strconv.Itoa(rowIdx), "PAYG tax withheld")
	xl.SetCellStyle(sheet, "A"+strconv.Itoa(rowIdx), "B"+strconv.Itoa(rowIdx), subHeaderStyle)
	rowIdx++

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
		xl.SetCellValue(sheet, "A"+strconv.Itoa(rowIdx), label)
		if label == "Period start" {
			xl.SetCellFormula(sheet, "B"+strconv.Itoa(rowIdx), "B2")
		} else if label == "Period end" {
			xl.SetCellFormula(sheet, "B"+strconv.Itoa(rowIdx), "B3")
		}
		rowIdx++
	}

	// --- 7. PAYG Instalment Section ---
	rowIdx++
	xl.SetCellValue(sheet, "A"+strconv.Itoa(rowIdx), "PAYG instalment")
	xl.SetCellStyle(sheet, "A"+strconv.Itoa(rowIdx), "B"+strconv.Itoa(rowIdx), subHeaderStyle)
	rowIdx++
	xl.SetCellValue(sheet, "A"+strconv.Itoa(rowIdx), "Option 1")
	xl.SetCellStyle(sheet, "A"+strconv.Itoa(rowIdx), "A"+strconv.Itoa(rowIdx), labelStyle)
	rowIdx++
	xl.SetCellValue(sheet, "A"+strconv.Itoa(rowIdx), "Option 2")
	xl.SetCellStyle(sheet, "A"+strconv.Itoa(rowIdx), "A"+strconv.Itoa(rowIdx), labelStyle)
	rowIdx++

	// --- 8. GST Refund/Payable ---
	rowIdx++
	xl.SetCellValue(sheet, "A"+strconv.Itoa(rowIdx), "GST Payable or (Refund)")
	// Formula: 1A - 1B (Adjusted cells for single column layout)
	// B7 is 1A, B9 is 1B based on rowIdx incrementing above
	xl.SetCellFormula(sheet, "B"+strconv.Itoa(rowIdx), "=B7-B9")

	xl.SetCellStyle(sheet, "A"+strconv.Itoa(rowIdx), "A"+strconv.Itoa(rowIdx), totalRowStyle)
	xl.SetCellStyle(sheet, "B"+strconv.Itoa(rowIdx), "B"+strconv.Itoa(rowIdx), currencyStyle)
	xl.SetCellStyle(sheet, "B"+strconv.Itoa(rowIdx), "B"+strconv.Itoa(rowIdx), labelStyle)

	xl.SetColWidth(sheet, "A", "A", 55)
	xl.SetColWidth(sheet, "B", "B", 25)

	return xl.WriteToBuffer()
}

type PeriodInfo struct {
	From  string
	To    string
	Label string
}

func (s *service) GetPeriodDates(ctx context.Context, f *BASReportFilter) (curr, prev PeriodInfo, err error) {
	var start, end time.Time

	// 1. Get Current Range
	if f.QuarterID != nil {
		qID, _ := uuid.Parse(*f.QuarterID)
		fromStr, toStr, err := s.repo.GetQuarterDates(ctx, qID)
		if err != nil {
			return curr, prev, err
		}
		start, _ = time.Parse("2006-01-02", fromStr)
		end, _ = time.Parse("2006-01-02", toStr)
	} else if f.Month != nil {
		start, end, err = util.GetMonthRange(*f.Month)
		if err != nil {
			return curr, prev, err
		}
	}

	// 2. Custom Quarter Mapping for your project
	// Jan-Mar: Q3 | Apr-Jun: Q4 | Jul-Sep: Q1 | Oct-Dec: Q2
	getProjectQuarter := func(t time.Time) string {
		month := t.Month()
		var qNum int
		var qMonths string

		switch {
		case month >= 1 && month <= 3:
			qNum = 3
			qMonths = "January-March"
		case month >= 4 && month <= 6:
			qNum = 4
			qMonths = "April-June"
		case month >= 7 && month <= 9:
			qNum = 1
			qMonths = "July-September"
		case month >= 10 && month <= 12:
			qNum = 2
			qMonths = "October-December"
		}
		return fmt.Sprintf("Q%d (%s) %d", qNum, qMonths, t.Year())
	}

	// 3. Set Current Period
	curr.From = start.Format("02-Jan-06")
	curr.To = end.Format("02-Jan-06")
	curr.Label = getProjectQuarter(start)

	// 4. Set Previous Period (Preceding Quarter = Current Start - 3 Months)
	// Example: If current is April (Q4), prevStart becomes January (Q3)
	prevStart := start.AddDate(0, -3, 0)

	// We calculate the end of that previous quarter
	// (3 months from prevStart, then minus 1 day)
	prevEnd := prevStart.AddDate(0, 3, 0).Add(-time.Hour * 24)

	prev.From = prevStart.Format("02-Jan-06")
	prev.To = prevEnd.Format("02-Jan-06")
	prev.Label = getProjectQuarter(prevStart)

	return curr, prev, nil
}

func (s *service) GetAllQuartersInYear(ctx context.Context, quarterID uuid.UUID) ([]BASQuarterInfo, error) {
	// 1. Call the repository method to fetch all quarters in the same financial year
	quarters, err := s.repo.GetAllQuartersInYear(ctx, quarterID)
	if err != nil {
		// Log the error if you have a logger, then return
		return nil, fmt.Errorf("service: failed to fetch quarters for year: %w", err)
	}

	// 2. Return the list (it will contain Q1, Q2, Q3, Q4)
	return quarters, nil
}

func (s *service) ExportBASPreparation(data *RsBASPreparation) (*excelize.File, error) {
	f := excelize.NewFile()
	sheet := "Quarterly BAS Preparation"
	f.NewSheet(sheet)
	f.DeleteSheet("Sheet1")

	// --- STYLES ---

	// Top Headers (Q1, Q2, etc.) - Light Blue, Bold, Black Borders
	styleHeaderBlue, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Family: "Calibri", Size: 11},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#DAEEF3"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})

	// Standard Grid Style (Used for all data cells)
	styleDataGrid, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Family: "Calibri", Size: 10},
		CustomNumFmt: func() *string { s := "$#,##0.00;[Red] $#,##0.00;$0.00"; return &s }(),
		Alignment:    &excelize.Alignment{Horizontal: "left"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})

	// Standard Table Grid Style (Used for all table data cells)
	styleTableGrid, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Family: "Calibri", Size: 10},
		CustomNumFmt: func() *string { s := "$#,##0.00;[Red] $#,##0.00;$0.00"; return &s }(),
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"#DAEEF3"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})

	// Section Titles (INCOME / EXPENSES) - Bold, Underline, Large
	styleSectionTitle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Underline: "single", Family: "Calibri", Size: 12},
	})

	// Net Profit/Loss
	styleNetProfit, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Family: "Calibri", Color: "000000"},
		CustomNumFmt: func() *string { s := "$#,##0.00;$#,##0.00"; return &s }(),
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})

	// Net Profit/Loss (Green cell background)
	styleNetProfitCol, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Color: "28a745"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"#c4f0ce"}, Pattern: 1},
		CustomNumFmt: func() *string { s := "$#,##0.00;$#,##0.00"; return &s }(),
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})

	// Net GST Payable
	styleGSTTotal, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Family: "Calibri"},
		CustomNumFmt: func() *string { s := "$#,##0.00;[Red] $#,##0.00"; return &s }(),
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})

	// Net GST Payable (Red Text)
	styleGSTPayableCol, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Color: "dc3545"},
		CustomNumFmt: func() *string { s := "$#,##0.00;$#,##0.00"; return &s }(),
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})

	// --- DATA PREPARATION ---
	allCols := append(data.Columns, data.GrandTotal)

	// --- RENDER HEADERS ---
	f.SetCellValue(sheet, "A2", "Financial Year Ending: 30 June")
	f.SetCellStyle(sheet, "A2", "A2", styleHeaderBlue)

	for i := range allCols {
		cIdx := 1 + (i * 4)
		startCol, _ := excelize.ColumnNumberToName(cIdx + 1)
		midCol, _ := excelize.ColumnNumberToName(cIdx + 2)
		endCol, _ := excelize.ColumnNumberToName(cIdx + 3)

		// Top Quarter Header
		f.MergeCell(sheet, fmt.Sprintf("%s5", startCol), fmt.Sprintf("%s5", endCol))
		f.SetCellValue(sheet, fmt.Sprintf("%s5", startCol), allCols[i].Quarter.Name)
		f.SetCellStyle(sheet, fmt.Sprintf("%s5", startCol), fmt.Sprintf("%s5", endCol), styleHeaderBlue)

		// Sub Headers
		f.SetCellValue(sheet, fmt.Sprintf("%s6", startCol), "Gross")
		f.SetCellValue(sheet, fmt.Sprintf("%s6", midCol), "GST")
		f.SetCellValue(sheet, fmt.Sprintf("%s6", endCol), "Net")
		f.SetCellStyle(sheet, fmt.Sprintf("%s6", startCol), fmt.Sprintf("%s6", endCol), styleHeaderBlue)
	}

	// --- INCOME SECTION ---
	currentRow := 7
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "INCOME")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleSectionTitle)
	currentRow++

	incomeRows := s.getUniqueNamesFromSection(allCols, "income")
	for _, name := range incomeRows {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), name)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleDataGrid)

		for i := range allCols {
			cIdx := 1 + (i * 4)
			startCol, _ := excelize.ColumnNumberToName(cIdx + 1)
			endCol, _ := excelize.ColumnNumberToName(cIdx + 3)

			// Always apply borders
			f.SetCellStyle(sheet, fmt.Sprintf("%s%d", startCol, currentRow), fmt.Sprintf("%s%d", endCol, currentRow), styleTableGrid)
			s.writeFormattedAmounts(f, sheet, cIdx, currentRow, allCols[i].Sections.Income.Items, name, styleTableGrid)
		}
		currentRow++
	}

	// --- EXPENSES SECTION ---
	currentRow += 1
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "EXPENSES")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleSectionTitle)
	currentRow++

	expenseRows := s.getUniqueNamesFromSection(allCols, "expenses")
	for _, name := range expenseRows {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), name)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleDataGrid)

		for i := range allCols {
			cIdx := 1 + (i * 4)
			startCol, _ := excelize.ColumnNumberToName(cIdx + 1)
			endCol, _ := excelize.ColumnNumberToName(cIdx + 3)

			// Force $0.00 by initializing with 0
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", startCol, currentRow), 0)
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", endCol, currentRow), 0)
			f.SetCellStyle(sheet, fmt.Sprintf("%s%d", startCol, currentRow), fmt.Sprintf("%s%d", endCol, currentRow), styleTableGrid)
			s.writeFormattedAmounts(f, sheet, cIdx, currentRow, allCols[i].Sections.Expenses.Items, name, styleTableGrid)
		}
		currentRow++
	}

	// --- SUMMARY SECTION ---
	currentRow += 2

	// Net Profit Row (Green Background)
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "Net Profit/Loss")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleNetProfit)

	for i, col := range allCols {
		cIdx := 1 + (i * 4)
		startCol, _ := excelize.ColumnNumberToName(cIdx + 1)
		endCol, _ := excelize.ColumnNumberToName(cIdx + 3)

		if len(col.Sections.NetProfitLoss.Items) > 0 {
			// Force $0.00 by initializing with 0
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", startCol, currentRow), 0)
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", endCol, currentRow), 0)
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", endCol, currentRow), col.Sections.NetProfitLoss.Items[0].Amounts.Net)
			f.SetCellStyle(sheet, fmt.Sprintf("%s%d", startCol, currentRow), fmt.Sprintf("%s%d", endCol, currentRow), styleNetProfitCol)
		}
	}

	currentRow++
	currentRow++

	// Net GST Payable Row (Red Text)
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "Net GST Payable")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleGSTTotal)

	for i, col := range allCols {
		cIdx := 1 + (i * 4)
		startCol, _ := excelize.ColumnNumberToName(cIdx + 1)
		endCol, _ := excelize.ColumnNumberToName(cIdx + 3)

		f.SetCellStyle(sheet, fmt.Sprintf("%s%d", startCol, currentRow), fmt.Sprintf("%s%d", endCol, currentRow), styleGSTPayableCol)
		f.SetCellValue(sheet, fmt.Sprintf("%s%d", endCol, currentRow), col.NetGSTPayable)
	}

	// --- FILTERS ---
	lastCol, _ := excelize.ColumnNumberToName(1 + (len(allCols) * 4))
	f.AutoFilter(sheet, fmt.Sprintf("A6:%s%d", lastCol, currentRow), nil)

	// --- FINAL DIMENSIONS ---
	f.SetColWidth(sheet, "A", "A", 45)
	for col := 2; col <= 1+(len(allCols)*4); col++ {
		name, _ := excelize.ColumnNumberToName(col)
		// Check if it's a spacer column (the blank column between Qs)
		if (col-1)%4 == 0 {
			f.SetColWidth(sheet, name, name, 3) // Narrow spacer
		} else {
			f.SetColWidth(sheet, name, name, 15) // Standard data width
		}
	}

	return f, nil
}

func (s *service) writeFormattedAmounts(f *excelize.File, sheet string, startIdx, row int, items []BASLineItem, name string, styleID int) {
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

func (s *service) getUniqueNamesFromSection(allCols []BASColumn, section string) []string {
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
