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

// buildReport assembles the flat P&L report structure from DB rows, grouped by clinic.
func buildReport(f *PLReportFilter, rows []*PLReportRow) *RsReport {
	clinicOrder := []string{}
	clinicSeen := map[string]bool{}
	clinicNames := map[string]string{}

	// clinic+section → line items & totals (aggregated across all forms)
	itemsMap := map[sectionKey][]RsReportLineItem{}
	totalsMap := map[sectionKey]float64{}

	for _, r := range rows {
		if !clinicSeen[r.ClinicID] {
			clinicOrder = append(clinicOrder, r.ClinicID)
			clinicSeen[r.ClinicID] = true
			clinicNames[r.ClinicID] = r.ClinicName
		}

		sk := sectionKey{r.ClinicID, r.SectionType}

		var fieldTotal float64
		if r.SectionType == "COLLECTION" {
			fieldTotal = round2(r.GrossAmount)
		} else {
			fieldTotal = round2(r.NetAmount)
		}

		itemsMap[sk] = append(itemsMap[sk], RsReportLineItem{
			CoaID:      r.CoaID,
			CoaName:    r.AccountName,
			FieldID:    r.FormFieldID,
			FieldName:  r.FieldLabel,
			FieldTotal: fieldTotal,
			TaxType:    r.TaxName,
			TaxTypeID:  r.TaxName,
			TaxAmount:  round2(r.GstAmount),
		})
		totalsMap[sk] += fieldTotal
	}

	var overallNet float64
	clinics := make([]RsReportClinic, 0, len(clinicOrder))

	for _, cid := range clinicOrder {
		sections := make([]RsReportSection, 0, 3)
		totalIncome := 0.0
		totalCOS := 0.0
		totalOther := 0.0

		for _, m := range sectionMeta {
			sk := sectionKey{cid, m.sectionType}
			items, ok := itemsMap[sk]
			if !ok {
				continue
			}
			total := round2(totalsMap[sk])

			switch m.sectionType {
			case "COLLECTION":
				totalIncome = total
			case "COST":
				totalCOS = total
			case "OTHER_COST":
				totalOther = total
			}

			sections = append(sections, RsReportSection{
				SectionType:  m.displayType,
				SectionLabel: m.sectionLabel,
				SectionTotal: total,
				Items:        items,
			})
		}

		grossProfit := round2(totalIncome - totalCOS)
		netProfit := round2(grossProfit - totalOther)
		overallNet += netProfit

		clinics = append(clinics, RsReportClinic{
			ClinicID:           cid,
			ClinicName:         clinicNames[cid],
			TotalIncome:        totalIncome,
			TotalCostOfSales:   totalCOS,
			GrossProfit:        grossProfit,
			TotalOtherExpenses: totalOther,
			NetProfit:          netProfit,
			Sections:           sections,
		})
	}

	dateRange := ""
	if f.DateFrom != nil && f.DateUntil != nil {
		dateRange = *f.DateFrom + " to " + *f.DateUntil
	}

	return &RsReport{
		ReportMetadata: RsReportMetadata{
			DateRange:        dateRange,
			OverallNetProfit: round2(overallNet),
		},
		Clinics: clinics,
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

type sectionKey struct{ formID, sectionType string }
