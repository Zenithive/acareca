package bas

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/auth"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/business/clinic"
	"github.com/iamarpitzala/acareca/internal/modules/business/fy"
	"github.com/iamarpitzala/acareca/internal/modules/business/shared/events"
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
	GetBASPreparation(ctx context.Context, actorID uuid.UUID, role string, f *BASFilter) (*RsBASPreparation, error)
	ExportActivityStatement(ctx context.Context, quarters []QuarterData, prevDates PeriodInfo, exportType string, actorID uuid.UUID, role string, userID uuid.UUID) (*bytes.Buffer, string, error)
	GetPeriodDates(ctx context.Context, f *BASReportFilter) (curr PeriodInfo, prev PeriodInfo, err error)
	GetAllQuartersInYear(ctx context.Context, quarterID uuid.UUID) ([]BASQuarterInfo, error)
	// ExportBASPreparation(ctx context.Context, data *RsBASPreparation, actorID uuid.UUID, role string, userID uuid.UUID, filter *BASFilter) (*excelize.File, error)
	generateActivityExcelReport(ctx context.Context, quarters []QuarterData, prevDates PeriodInfo) (*bytes.Buffer, error)
	generateActivityPDFWithChrome(ctx context.Context, data interface{}) (*bytes.Buffer, error)
	ExportBASPreparation(ctx context.Context, data *RsBASPreparation, actorID uuid.UUID, role string, userID uuid.UUID, filter *BASFilter, exportType string) (interface{}, error)
}

type service struct {
	repo           Repository
	accountantRepo accountant.Repository
	auditSvc       audit.Service
	clinicRepo     clinic.Repository
	fyRepo         fy.Repository
	eventsSvc      events.Service
	authRepo       auth.Repository
}

func NewService(repo Repository, accountantRepo accountant.Repository, auditSvc audit.Service, clinicRepo clinic.Repository, fyRepo fy.Repository, eventsSvc events.Service, authRepo auth.Repository) Service {
	return &service{repo: repo, accountantRepo: accountantRepo, auditSvc: auditSvc, clinicRepo: clinicRepo, fyRepo: fyRepo, eventsSvc: eventsSvc, authRepo: authRepo}
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

func (s *service) GetBASPreparation(ctx context.Context, actorID uuid.UUID, role string, f *BASFilter) (*RsBASPreparation, error) {
	isAccountant := false
	if role == util.RoleAccountant {
		isAccountant = true
	}

	var ownerID uuid.UUID
	var clinicIDs []uuid.UUID

	// Convert BASFilter to common.Filter for clinic listing
	commonFilter := f.MapToFilter()

	// Use clinic_id array from BASFilter
	requestedClinicIDs := f.ParsedClinicIDs

	if isAccountant {
		// If clinic_ids are provided, verify permission for each clinic
		if len(requestedClinicIDs) > 0 {
			for _, clinicID := range requestedClinicIDs {
				permission, err := s.clinicRepo.GetAccountantPermission(ctx, actorID, clinicID)
				if err != nil {
					return nil, fmt.Errorf("permission denied for clinic %s", clinicID)
				}
				ownerID = permission.PractitionerID
				clinicIDs = append(clinicIDs, clinicID)
			}
		} else {
			// If no clinic_ids provided, get all clinics the accountant has access to
			clinics, err := s.clinicRepo.ListClinicByAccountant(ctx, actorID, commonFilter)
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
		ownerID = actorID

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

func (s *service) ExportActivityStatement(ctx context.Context, quarters []QuarterData, prevDates PeriodInfo, exportType string, actorID uuid.UUID, role string, userID uuid.UUID) (*bytes.Buffer, string, error) {
	// 1. Branching Logic
	if strings.ToLower(exportType) == "pdf" {
		// Wrap data for template
		data := struct {
			Quarters []QuarterData
			Prev     PeriodInfo
		}{
			Quarters: quarters,
			Prev:     prevDates,
		}

		buf, err := s.generateActivityPDFWithChrome(ctx, data)
		if err != nil {
			return nil, "", fmt.Errorf("failed to generate activity pdf: %w", err)
		}
		return buf, "application/pdf", nil
	}

	// 2. Default to Excel logic
	buf, err := s.generateActivityExcelReport(ctx, quarters, prevDates)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate activity excel: %w", err)
	}

	return buf, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
}
func (s *service) generateActivityExcelReport(ctx context.Context, quarters []QuarterData, prevDates PeriodInfo) (*bytes.Buffer, error) {

	xl := excelize.NewFile()
	defer xl.Close()

	sheet := "Activity Statement"
	dataSheet := "SourceData"
	xl.SetSheetName("Sheet1", sheet)
	xl.NewSheet(dataSheet)

	// parsedActorID := actorID.String()

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

	// // --- AUDIT LOG ---
	// meta := auditctx.GetMetadata(ctx)
	// var userIDStr string
	// userIDStr = userID.String()
	// s.auditSvc.LogAsync(&audit.LogEntry{
	// 	PracticeID: nil,
	// 	UserID:     &userIDStr,
	// 	Action:     auditctx.ActionActivityStatementExported,
	// 	Module:     auditctx.ModuleReport,
	// 	EntityType: strPtr(auditctx.EntityActivityStatement),
	// 	EntityID:   &parsedActorID,
	// 	AfterState: map[string]interface{}{
	// 		"report_type": "Activity Statement",
	// 		"quarters":    quarters,
	// 	},
	// 	IPAddress: meta.IPAddress,
	// 	UserAgent: meta.UserAgent,
	// })

	return xl.WriteToBuffer()
}

const activityTemplate = `
<html>
<head>
<style>
    body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; font-size: 10pt; padding: 20px; color: #000; }
    table { width: 100%; border-collapse: collapse; margin-bottom: 20px; table-layout: fixed; }
    th, td { border: 1px solid #bfbfbf; padding: 8px; word-wrap: break-word; }
    
    .header { background-color: #4EA7B3; color: white; font-weight: bold; text-align: center; }
    .sub-header { background-color: #E1F0F2; font-weight: bold; color: #2A5D63; }
    .label { font-weight: bold; width: 70%; }
    .amount { text-align: right; width: 30%; font-family: 'Courier New', Courier, monospace; font-weight: bold;}
    .total-row { background-color: #4EA7B3; color: white; font-weight: bold; }
    
    .indent { padding-left: 25px; font-weight: normal; }
</style>
</head>
<body>
    {{$q := index .Quarters 0}}
    
    <table>
        <tr>
            <td class="header">Activity Statement Information</td>
            <td class="header">BAS</td>
        </tr>
        <tr>
            <td class="label">Period start</td>
            <td>{{$q.Period.From}}</td>
        </tr>
        <tr>
            <td class="label">Period end</td>
            <td>{{$q.Period.To}}</td>
        </tr>
        <tr>
            <td class="label">Qtr</td>
            <td>{{$q.Period.Label}}</td>
        </tr>
    </table>

    <table>
        <tr class="sub-header"><td colspan="2">GST Section</td></tr>
        <tr>
            <td class="label">G1 (Total Sales)</td>
            <td class="amount">${{printf "%.2f" $q.Report.G1}}</td>
        </tr>
        <tr>
            <td class="label">1A (GST on Sales)</td>
            <td class="amount">${{printf "%.2f" $q.Report.A1}}</td>
        </tr>
        <tr>
            <td class="label">G11 (Total Purchases)</td>
            <td class="amount">${{printf "%.2f" $q.Report.G11}}</td>
        </tr>
        <tr>
            <td class="label">1B (GST on Purchases)</td>
            <td class="amount">${{printf "%.2f" $q.Report.B1}}</td>
        </tr>
    </table>

    <table>
        <tr class="sub-header"><td colspan="2">PAYG tax withheld</td></tr>
        <tr>
            <td class="label">Period start</td>
            <td>{{$q.Period.From}}</td>
        </tr>
        <tr>
            <td class="label">Period end</td>
            <td>{{$q.Period.To}}</td>
        </tr>
        <tr>
            <td class="label">W1 (Total Wages, salary and other payments)</td>
			<td>-</td>
           
        </tr>
        <tr>
            <td class="label">W2 (Amount withheld from payments shown at W1)</td>
			<td>-</td>
            
        </tr>
        <tr>
            <td class="label">W3 (Other amounts withheld)</td>
			<td>-</td>
            
        </tr>
        <tr>
            <td class="label">W4 (Amount withheld where no ABN is quoted)</td>
			<td>-</td>
            
        </tr>
        <tr>
            <td class="label">W5 (Total amounts withheld)</td>
			<td>-</td>
           
        </tr>
    </table>

    <table>
        <tr class="sub-header"><td colspan="2">PAYG instalment</td></tr>
        <tr>
            <td class="label">Option 1</td>
			<td>-</td>
            
        </tr>
        <tr>
            <td class="label">Option 2</td>
			<td>-</td>
            
        </tr>
    </table>

    <table>
        <tr class="total-row">
            <td class="label">GST Payable or (Refund)</td>
            <td class="amount">${{calcRefund $q.Report.A1 $q.Report.B1}}</td>
        </tr>
    </table>
</body>
</html>
`

func (s *service) generateActivityPDFWithChrome(ctx context.Context, data interface{}) (*bytes.Buffer, error) {
	tmpl, err := template.New("activity").Funcs(template.FuncMap{
		"calcRefund": func(a1, b1 float64) string {
			return fmt.Sprintf("%.2f", a1-b1)
		},
	}).Parse(activityTemplate)
	if err != nil {
		return nil, err
	}

	var htmlBuf bytes.Buffer
	if err := tmpl.Execute(&htmlBuf, data); err != nil {
		return nil, err
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
		chromedp.Headless,
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()
	taskCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	var pdfBuffer []byte
	err = chromedp.Run(taskCtx,
		chromedp.Navigate("about:blank"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			frameTree, _ := page.GetFrameTree().Do(ctx)
			return page.SetDocumentContent(frameTree.Frame.ID, htmlBuf.String()).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			buf, _, err := page.PrintToPDF().WithPrintBackground(true).Do(ctx)
			pdfBuffer = buf
			return err
		}),
	)

	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(pdfBuffer), nil
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

func (s *service) ExportBASPreparation(ctx context.Context, data *RsBASPreparation, actorID uuid.UUID, role string, userID uuid.UUID, filter *BASFilter, exportType string) (interface{}, error) {
	f := excelize.NewFile()
	sheet := "Quarterly BAS REPORT"
	f.NewSheet(sheet)
	f.DeleteSheet("Sheet1")

	parsedActorID := actorID.String()

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
		Font: &excelize.Font{Bold: true, Family: "Calibri", Size: 12},
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

	// Get Financial Year
	parsedID, err := uuid.Parse(*filter.FinancialYearID)
	if err != nil {
		return nil, fmt.Errorf("invalid financial year id: %w", err)
	}

	FY, err := s.fyRepo.GetFinancialYearByID(ctx, parsedID)
	if err != nil {
		return nil, err
	}

	// --- RENDER HEADERS ---
	f.SetCellValue(sheet, "A2", FY.Label)
	f.SetCellStyle(sheet, "A2", "A2", styleHeaderBlue)

	for i := range allCols {
		cIdx := 1 + (i * 4)
		startCol, _ := excelize.ColumnNumberToName(cIdx + 1)
		midCol, _ := excelize.ColumnNumberToName(cIdx + 2)
		endCol, _ := excelize.ColumnNumberToName(cIdx + 3)

		// --- Quarter Name FORMATTING ---
		headerValue := allCols[i].Quarter.Name

		// Only attempt to format if it's an actual quarter (skip for the "Total" column)
		if allCols[i].Quarter.StartDate != "" {
			// Parse the year
			t, err := time.Parse("2006-01-02", allCols[i].Quarter.StartDate)
			yearStr := ""
			if err == nil {
				yearStr = fmt.Sprintf("%d", t.Year())
			}

			// Combine to: Quarter Name (Display Range) Year
			headerValue = fmt.Sprintf("%s (%s) %s",
				allCols[i].Quarter.Name,
				allCols[i].Quarter.DisplayRange,
				yearStr,
			)
		}

		// Top Quarter Header
		f.MergeCell(sheet, fmt.Sprintf("%s5", startCol), fmt.Sprintf("%s5", endCol))
		f.SetCellValue(sheet, fmt.Sprintf("%s5", startCol), headerValue)
		f.SetCellStyle(sheet, fmt.Sprintf("%s5", startCol), fmt.Sprintf("%s5", endCol), styleHeaderBlue)

		// Sub Headers
		f.SetCellValue(sheet, fmt.Sprintf("%s6", startCol), "Gross")
		f.SetCellValue(sheet, fmt.Sprintf("%s6", midCol), "GST")
		f.SetCellValue(sheet, fmt.Sprintf("%s6", endCol), "Net")
		f.SetCellStyle(sheet, fmt.Sprintf("%s6", startCol), fmt.Sprintf("%s6", endCol), styleHeaderBlue)
	}

	// Helper to track range for dynamic calculations
	type SectionMeta struct {
		StartRow int
		EndRow   int
	}
	var incomeMeta, expenseMeta SectionMeta

	// --- INCOME SECTION ---
	currentRow := 7
	incomeHeaderRow := currentRow
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "INCOME")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleSectionTitle)
	currentRow++
	incomeMeta.StartRow = currentRow

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
	incomeMeta.EndRow = currentRow - 1

	// Create Income Table for Filtering
	if len(incomeRows) > 0 {
		// CHANGE: Range is now only Column A (A7 to A9)
		tblRange := fmt.Sprintf("A%d:A%d", incomeHeaderRow, incomeMeta.EndRow)
		showH := true

		f.AddTable(sheet, &excelize.Table{
			Range:         tblRange,
			Name:          "IncomeTable",
			StyleName:     "",
			ShowHeaderRow: &showH,
		})
	}

	// --- EXPENSES SECTION ---
	currentRow += 1
	expenseHeaderRow := currentRow
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "EXPENSES")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleSectionTitle)
	currentRow++
	expenseMeta.StartRow = currentRow

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
	expenseMeta.EndRow = currentRow - 1

	// Create Expenses Table for Filtering
	if len(expenseRows) > 0 {
		// CHANGE: Range is now only Column A
		tblRange := fmt.Sprintf("A%d:A%d", expenseHeaderRow, expenseMeta.EndRow)
		showH := true

		f.AddTable(sheet, &excelize.Table{
			Range:         tblRange,
			Name:          "ExpenseTable",
			StyleName:     "",
			ShowHeaderRow: &showH,
		})
	}

	// --- SUMMARY SECTION ---
	currentRow += 2
	netProfitRow := currentRow
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
	netGSTRow := currentRow
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "Net GST Payable")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleGSTTotal)

	// Apply Dynamic Formulas for each Quarter Column
	for i := range allCols {
		cIdx := 1 + (i * 4)
		gstCol, _ := excelize.ColumnNumberToName(cIdx + 2) // GST column
		netCol, _ := excelize.ColumnNumberToName(cIdx + 3) // Net column

		// Net Profit Formula (Net Income - Net Expenses)
		incomeSum := fmt.Sprintf("SUBTOTAL(109, %s%d:%s%d)", netCol, incomeMeta.StartRow, netCol, incomeMeta.EndRow)
		expenseSum := fmt.Sprintf("SUBTOTAL(109, %s%d:%s%d)", netCol, expenseMeta.StartRow, netCol, expenseMeta.EndRow)
		f.SetCellFormula(sheet, fmt.Sprintf("%s%d", netCol, netProfitRow), fmt.Sprintf("%s-%s", incomeSum, expenseSum))
		f.SetCellStyle(sheet, fmt.Sprintf("%s%d", netCol, netProfitRow), fmt.Sprintf("%s%d", netCol, netProfitRow), styleNetProfitCol)

		// Net GST Payable Formula (GST Income - GST Expenses)
		incomeGST := fmt.Sprintf("SUBTOTAL(109, %s%d:%s%d)", gstCol, incomeMeta.StartRow, gstCol, incomeMeta.EndRow)
		expenseGST := fmt.Sprintf("SUBTOTAL(109, %s%d:%s%d)", gstCol, expenseMeta.StartRow, gstCol, expenseMeta.EndRow)
		f.SetCellFormula(sheet, fmt.Sprintf("%s%d", netCol, netGSTRow), fmt.Sprintf("%s-%s", incomeGST, expenseGST))
		f.SetCellStyle(sheet, fmt.Sprintf("%s%d", netCol, netGSTRow), fmt.Sprintf("%s%d", netCol, netGSTRow), styleGSTPayableCol)
	}

	// --- FINAL DIMENSIONS ---
	f.SetColWidth(sheet, "A", "A", 45)
	for col := 2; col <= 1+(len(allCols)*4); col++ {
		name, _ := excelize.ColumnNumberToName(col)
		if (col-1)%4 == 0 {
			f.SetColWidth(sheet, name, name, 3)
		} else {
			f.SetColWidth(sheet, name, name, 15)
		}
	}

	// --- AUDIT LOG ---
	meta := auditctx.GetMetadata(ctx)
	var userIDStr string
	userIDStr = userID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: nil,
		UserID:     &userIDStr,
		Action:     auditctx.ActionBASReportExported,
		Module:     auditctx.ModuleReport,
		EntityType: strPtr(auditctx.EntityBASReport),
		EntityID:   &parsedActorID,
		AfterState: map[string]interface{}{
			"report_type":    "Quarterly BAS Report",
			"financial_year": filter.FinancialYearID,
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})

	if role == util.RoleAccountant {
		// Fetching user details
		var fullName string
		user, err := s.authRepo.FindByID(ctx, userID)
		if err == nil {
			fullName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
		}

		// Record the Shared Event
		err = s.eventsSvc.Record(ctx, events.SharedEvent{
			ID: uuid.New(),
			// PractitionerID: practitionerID,
			AccountantID: actorID,
			ActorID:      actorID,
			ActorName:    &fullName,
			ActorType:    role,
			EventType:    "bas_report.exported",
			EntityType:   "REPORT",
			EntityID:     actorID,
			Description:  fmt.Sprintf("Accountant %s exported BAS Report", fullName),
			Metadata:     events.JSONBMap{"report_type": "Quarterly BAS Report", "financial_year": filter.FinancialYearID},
			CreatedAt:    time.Now(),
		})
	}

	if exportType == "pdf" {
		return s.convertExcelToPDF(f, sheet, data, FY.Label)
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

func strPtr(s string) *string {
	return &s
}

// Helper to convert the Excel file to PDF bytes using Chromedp
func (s *service) convertExcelToPDF(f *excelize.File, sheetName string, data *RsBASPreparation, FYLabel string) ([]byte, error) {
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer
	b.WriteString("<html><head><style>")
	b.WriteString(`
		@page { size: A3 landscape; margin: 0.5cm; }
		body { font-family: 'Calibri', sans-serif; margin: 0; padding: 10px; }
		table { border-collapse: collapse; table-layout: fixed; width: 100%; border: 1.2pt solid #000; }
		td { border: 1px solid #000; padding: 4px 2px; font-size: 8.5pt; height: 22px; text-align: center; }
		.header-blue { background-color: #DAEEF3 !important; font-weight: bold; }
		.fy-title { font-size: 16pt; font-weight: bold; background-color: #DAEEF3 !important; padding: 15px; border: 1.2pt solid #000; }
		.section-title { font-weight: bold; font-size: 11pt; border: none; padding-top: 12px; text-align: left; }
		.data-left { text-align: left; border: 1px solid #000; }
		.text-right { text-align: right; }
		.profit-green { background-color: #c4f0ce !important; font-weight: bold; color: #28a745; text-align: right; }
		.gst-red { font-weight: bold; color: #dc3545; text-align: right; }
	`)
	b.WriteString("</style></head><body><table>")

	// 16 columns: 1 Label + (4 Quarters * 3 Cols) + (1 Total * 3 Cols)
	b.WriteString("<colgroup><col style='width: 16%;'>")
	for i := 0; i < 15; i++ {
		b.WriteString("<col style='width: 5.6%;'>")
	}
	b.WriteString("</colgroup>")

	formatCurr := func(v float64) string { return fmt.Sprintf("$%.2f", v) }

	for rIdx, row := range rows {
		rowNum := rIdx + 1
		if len(row) == 0 {
			continue
		}

		// --- ROW 1: FINANCIAL YEAR ---
		if rowNum == 1 {
			// Use the passed FYLabel explicitly
			b.WriteString(fmt.Sprintf("<tr><td colspan='16' class='section-title'>%s</td></tr>", FYLabel))
			continue
		}

		// --- ROW 5: QUARTER NAMES ---
		if rowNum == 5 {
			b.WriteString("<tr><td class='header-blue'></td>") // Column A spacer
			// We iterate through the columns and look specifically for the Quarter names
			quarters := []string{"Q1 (Jul-Sep)", "Q2 (Oct-Dec)", "Q3 (Jan-Mar)", "Q4 (Apr-Jun)", "Grand Total"}
			for _, qName := range quarters {
				b.WriteString(fmt.Sprintf("<td class='header-blue' colspan='3' style='font-size:10pt;'>%s</td>", qName))
			}
			b.WriteString("</tr>")
			continue
		}

		// --- ROW 6: SUB-HEADERS (Gross, GST, Net) ---
		if rowNum == 6 {
			b.WriteString("<td class='header-blue'>Particulars</td>")
			for i := 0; i < 5; i++ {
				b.WriteString("<td class='header-blue'>Gross</td><td class='header-blue'>GST</td><td class='header-blue'>Net</td>")
			}
			b.WriteString("</tr>")
			continue
		}

		// --- DATA ROWS ---
		valA := row[0]

		classA := "data-left"
		if valA == "INCOME" || valA == "EXPENSES" {
			classA = "section-title"
			b.WriteString(fmt.Sprintf("<td colspan='16' class='%s'>%s</td></tr>", classA, valA))
			continue
		}

		b.WriteString(fmt.Sprintf("<td class='%s'>%s</td>", classA, valA))

		// Combine data columns (4 quarters + 1 grand total)
		allColumns := append(data.Columns, data.GrandTotal)

		for _, col := range allColumns {
			var g, gst, n float64
			found := false

			// Match Account from API Data
			for _, item := range append(col.Sections.Income.Items, col.Sections.Expenses.Items...) {
				if item.Name == valA {
					g, gst, n = item.Amounts.Gross, item.Amounts.GST, item.Amounts.Net
					found = true
					break
				}
			}

			// Handle Special Rows
			if valA == "Net Profit/Loss" && len(col.Sections.NetProfitLoss.Items) > 0 {
				item := col.Sections.NetProfitLoss.Items[0]
				g, gst, n = item.Amounts.Gross, item.Amounts.GST, item.Amounts.Net
				found = true
			} else if valA == "Net GST Payable" {
				gst = col.NetGSTPayable
				found = true
			}

			cellClass := "text-right"
			if valA == "Net Profit/Loss" {
				cellClass += " profit-green"
			}
			if valA == "Net GST Payable" {
				cellClass += " gst-red"
			}

			if found {
				b.WriteString(fmt.Sprintf("<td class='%s'>%s</td><td class='%s'>%s</td><td class='%s'>%s</td>",
					cellClass, formatCurr(g), cellClass, formatCurr(gst), cellClass, formatCurr(n)))
			} else {
				for i := 0; i < 3; i++ {
					b.WriteString("<td class='text-right'>$0.00</td>")
				}
			}
		}
		b.WriteString("</tr>")
	}

	b.WriteString("</table></body></html>")

	// Render using Chromedp
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	var buf []byte
	err = chromedp.Run(ctx,
		chromedp.Navigate("about:blank"),
		// Set viewport wide enough to capture all columns at once
		chromedp.EmulateViewport(1920, 1080),
		chromedp.ActionFunc(func(ctx context.Context) error {
			frameTree, _ := page.GetFrameTree().Do(ctx)
			return page.SetDocumentContent(frameTree.Frame.ID, b.String()).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			buf, _, err = page.PrintToPDF().
				WithPrintBackground(true).
				WithLandscape(true).
				WithPaperWidth(16.5). // A3 size
				WithPaperHeight(11.7).
				Do(ctx)
			return err
		}),
	)
	return buf, err
}
