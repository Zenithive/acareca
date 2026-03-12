package fy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service interface {
	CreateFY(ctx context.Context, req *RqCreateFY) (*RsFinancialYear, error)
	UpdateFYLabel(ctx context.Context, id uuid.UUID, req *RqUpdateFYLabel) (*RsFinancialYear, error)
	GetFinancialYears(ctx context.Context) ([]RsFinancialYear, error)
	GetFinancialQuarters(ctx context.Context, financialYearID uuid.UUID) ([]RsFinancialQuarter, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) CreateFY(ctx context.Context, req *RqCreateFY) (*RsFinancialYear, error) {
	// Parse fy_year (e.g., "2025-2026")
	years := strings.Split(req.FYYear, "-")
	if len(years) != 2 {
		return nil, errors.New("invalid fy_year format, expected format: YYYY-YYYY")
	}

	startYear := years[0]
	endYear := years[1]

	// Create start_date: 01-07-startYear
	startDate, err := time.Parse("02-01-2006", fmt.Sprintf("01-07-%s", startYear))
	if err != nil {
		return nil, fmt.Errorf("invalid start year: %w", err)
	}

	// Create end_date: 30-06-endYear
	endDate, err := time.Parse("02-01-2006", fmt.Sprintf("30-06-%s", endYear))
	if err != nil {
		return nil, fmt.Errorf("invalid end year: %w", err)
	}

	// If is_active is true, deactivate all other financial years
	if req.IsActive {
		if err := s.repo.DeactivateAllFinancialYears(ctx); err != nil {
			return nil, fmt.Errorf("deactivate existing financial years: %w", err)
		}
	}

	// Create financial year
	fy := &FinancialYear{
		Label:     req.Label,
		IsActive:  req.IsActive,
		StartDate: startDate,
		EndDate:   endDate,
	}

	createdFY, err := s.repo.CreateFinancialYear(ctx, fy)
	if err != nil {
		return nil, err
	}

	// Create 4 quarters
	quarters := []struct {
		label      string
		startDate  string
		endDate    string
		useEndYear bool
	}{
		{"Q1", "01-07", "30-09", false},
		{"Q2", "01-10", "31-12", false},
		{"Q3", "01-01", "31-03", true},
		{"Q4", "01-04", "30-06", true},
	}

	for _, q := range quarters {
		year := startYear
		if q.useEndYear {
			year = endYear
		}

		qStartDate, err := time.Parse("02-01-2006", fmt.Sprintf("%s-%s", q.startDate, year))
		if err != nil {
			return nil, fmt.Errorf("parse quarter start date: %w", err)
		}

		qEndDate, err := time.Parse("02-01-2006", fmt.Sprintf("%s-%s", q.endDate, year))
		if err != nil {
			return nil, fmt.Errorf("parse quarter end date: %w", err)
		}

		quarter := &FinancialQuarter{
			FinancialYearID: createdFY.ID,
			Label:           q.label,
			StartDate:       qStartDate,
			EndDate:         qEndDate,
		}

		if _, err := s.repo.CreateFinancialQuarter(ctx, quarter); err != nil {
			return nil, fmt.Errorf("create quarter %s: %w", q.label, err)
		}
	}

	return &RsFinancialYear{
		ID:        createdFY.ID,
		Label:     createdFY.Label,
		StartDate: createdFY.StartDate,
		EndDate:   createdFY.EndDate,
	}, nil
}

func (s *service) UpdateFYLabel(ctx context.Context, id uuid.UUID, req *RqUpdateFYLabel) (*RsFinancialYear, error) {
	// Validate that at least one field is provided
	hasLabel := req.Label != nil && strings.TrimSpace(*req.Label) != ""
	hasIsActive := req.IsActive != nil

	if !hasLabel && !hasIsActive {
		return nil, errors.New("label --or-- is_active is required in payload")
	}

	fy, err := s.repo.GetFinancialYearByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Update label only if provided and not empty after trimming
	if hasLabel {
		fy.Label = strings.TrimSpace(*req.Label)
	}

	// If is_active is provided and set to true, deactivate all other financial years
	if req.IsActive != nil && *req.IsActive {
		if err := s.repo.DeactivateAllFinancialYears(ctx); err != nil {
			return nil, fmt.Errorf("deactivate existing financial years: %w", err)
		}
		fy.IsActive = true
	} else if req.IsActive != nil {
		fy.IsActive = *req.IsActive
	}

	updatedFY, err := s.repo.UpdateFinancialYear(ctx, fy)
	if err != nil {
		return nil, err
	}

	return &RsFinancialYear{
		ID:        updatedFY.ID,
		Label:     updatedFY.Label,
		StartDate: updatedFY.StartDate,
		EndDate:   updatedFY.EndDate,
	}, nil
}

func (s *service) GetFinancialYears(ctx context.Context) ([]RsFinancialYear, error) {
	years, err := s.repo.GetFinancialYears(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]RsFinancialYear, 0, len(years))
	for _, year := range years {
		result = append(result, RsFinancialYear{
			ID:        year.ID,
			Label:     year.Label,
			StartDate: year.StartDate,
			EndDate:   year.EndDate,
		})
	}

	return result, nil
}

func (s *service) GetFinancialQuarters(ctx context.Context, financialYearID uuid.UUID) ([]RsFinancialQuarter, error) {
	quarters, err := s.repo.GetFinancialQuarters(ctx, financialYearID)
	if err != nil {
		return nil, err
	}

	result := make([]RsFinancialQuarter, 0, len(quarters))
	for _, quarter := range quarters {
		result = append(result, RsFinancialQuarter{
			ID:        quarter.ID,
			Label:     quarter.Label,
			StartDate: quarter.StartDate,
			EndDate:   quarter.EndDate,
		})
	}

	return result, nil
}
