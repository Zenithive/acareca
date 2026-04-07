package bas

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

// Service defines the business-logic layer for the BAS module.
type Service interface {
	GetQuarterlySummary(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]RsBASSummary, error)
	GetByAccount(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]RsBASByAccount, error)
	GetMonthly(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]RsBASMonthly, error)
	GetReport(ctx context.Context, f *BASReportFilter) (*RsBASReport, error)
	GetBASPreparation(ctx context.Context, clinicID uuid.UUID, f *BASFilter) (*RsBASPreparation, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
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

func (s *service) GetBASPreparation(ctx context.Context, clinicID uuid.UUID, f *BASFilter) (*RsBASPreparation, error) {
	rows, err := s.repo.GetBASLineItems(ctx, clinicID, f)
	if err != nil {
		return nil, err
	}

	quarterGroups := make(map[string][]*BASLineItemRow)
	for _, r := range rows {
		k := r.PeriodQuarter.Format("2006-01-02")
		quarterGroups[k] = append(quarterGroups[k], r)
	}

	resp := &RsBASPreparation{Columns: []BASColumn{}}

	// --- Iterate over SELECTED Quarters first ---
	if len(f.QuarterIDs) > 0 {
		for _, qID := range f.QuarterIDs {
			if qID == nil {
				continue
			}

			// Get metadata by ID (Always works even if no transactions)
			qInfo, err := s.repo.GetQuarterInfoByID(ctx, *qID)
			if err != nil {
				continue
			}

			// Get data from our map (might be nil/empty)
			qRows := quarterGroups[qInfo.StartDate]

			// Map to column (mapToBASColumn handles nil/empty rows by returning $0)
			col := s.mapToBASColumn(qRows)
			col.Quarter = *qInfo
			resp.Columns = append(resp.Columns, col)
		}
	} else {
		// Fallback for when no specific quarters are selected (Show what exists)
		for key, qRows := range quarterGroups {
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
	resp.GrandTotal = s.mapToBASColumn(rows)
	resp.GrandTotal.Quarter.Name = "Total"

	return resp, nil
}

func (s *service) mapToBASColumn(rows []*BASLineItemRow) BASColumn {
	var col BASColumn
	col.Sections.Income.Items = make([]BASLineItem, 0)
	col.Sections.Expenses.Items = make([]BASLineItem, 0)

	var g3, g8, a1, b1 BASAmount
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
			// Split by BAS category only (not by GST amount)
			if BASCategory(r.BasCategory) == BASCategoryTaxable {
				g8.Gross += r.GrossAmount
				g8.GST += r.GstAmount
				g8.Net += r.NetAmount
				a1.Gross += r.GstAmount
			} else {
				g3.Gross += r.GrossAmount
				g3.GST += r.GstAmount
				g3.Net += r.NetAmount
			}
		case "COST", "OTHER_COST", "":
			// Track 1B for ALL GST on purchases (matches vw_bas_summary)
			// This includes TAXABLE and GST_FREE items with GST
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

	// Income Section
	g3 = finalize(g3)
	g8 = finalize(g8)

	totalIncome := BASAmount{
		Gross: roundToTwo(g3.Gross + g8.Gross),
		GST:   roundToTwo(g3.GST + g8.GST),
		Net:   roundToTwo(g3.Net + g8.Net),
	}
	col.Sections.Income.Items = []BASLineItem{
		{Name: "Income – GST Free (G3)", Amounts: g3},
		{Name: "Income – GST", Amounts: g8},
		{
			Name: "1A GST on Sales",
			Amounts: BASAmount{
				Gross: a1.Gross, // Use a1 (only taxable GST), not totalIncome.GST
			},
		},
	}

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
		{Name: "Subtotal (non capital purchase)", Amounts: subtotalExpenses},
		// {
		// 	Name: "G11/1B GST on Purchases",
		// 	Amounts: BASAmount{
		// 		Gross: subtotalExpenses.Gross, // (G11) Total Spent
		// 		GST:   b1.Gross,               // (1B) GST to claim
		// 	},
		// },
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
	col.NetGSTPayable = roundToTwo(a1.Gross - b1.Gross)

	return col
}

// Helper to round values after calculation
func roundToTwo(val float64) float64 {
	return math.Round(val*100) / 100
}
