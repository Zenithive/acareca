package pl

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Service interface {
	GetMonthlySummary(ctx context.Context, f *PLFilter) ([]RsPLSummary, error)
	GetByAccount(ctx context.Context, f *PLFilter) ([]RsPLAccount, error)
	GetByResponsibility(ctx context.Context, f *PLFilter) ([]RsPLResponsibility, error)
	GetFYSummary(ctx context.Context, f *PLFilter) ([]RsPLFYSummary, error)
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
