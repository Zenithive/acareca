package pl

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

type Service interface {
	GetMonthlySummary(ctx context.Context, f *PLFilter) ([]RsPLSummary, error)
	GetByAccount(ctx context.Context, f *PLFilter) ([]RsPLAccount, error)
	GetByResponsibility(ctx context.Context, f *PLFilter) ([]RsPLResponsibility, error)
	GetFYSummary(ctx context.Context, f *PLFilter) ([]RsPLFYSummary, error)
	GetReport(ctx context.Context, f *PLReportFilter) (*RsReport, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
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

func (s *service) GetReport(ctx context.Context, f *PLReportFilter) (*RsReport, error) {
	if f.ClinicID != nil {
		if _, err := uuid.Parse(*f.ClinicID); err != nil {
			return nil, fmt.Errorf("invalid clinic_id: must be a valid UUID")
		}
	}

	var from, to time.Time
	var err error
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
		// Use gross_amount consistently across all sections so that
		// income and costs are compared on the same (GST-inclusive) basis.
		// Previously COST/OTHER_COST used net_amount, which understated
		// "Gross Up" management fees that carry GST on top.
		val := r.GrossAmount

		ck := coaKey{r.SectionType, r.CoaID}
		if !coaSeen[ck] {
			coaSeen[ck] = true
			coaOrder[r.SectionType] = append(coaOrder[r.SectionType], r.CoaID)
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
