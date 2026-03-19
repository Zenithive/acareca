package bas

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

// Service defines the business-logic layer for the BAS module.
type Service interface {
	GetQuarterlySummary(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]RsBASSummary, error)
	GetByAccount(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]RsBASByAccount, error)
	GetMonthly(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]RsBASMonthly, error)
	GetReport(ctx context.Context, f *BASReportFilter) (*RsBASReport, error)
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
