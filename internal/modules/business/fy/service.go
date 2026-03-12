package fy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type Service interface {
	CreateFY(ctx context.Context, req *RqCreateFY) (*RsFinancialYear, error)
	UpdateFYLabel(ctx context.Context, id uuid.UUID, req *RqUpdateFYLabel) (*RsFinancialYear, error)
	GetFinancialYears(ctx context.Context) ([]RsFinancialYear, error)
	GetFinancialQuarters(ctx context.Context, financialYearID uuid.UUID) ([]RsFinancialQuarter, error)
}

type service struct {
	repo Repository
	db   *sqlx.DB
}

func NewService(repo Repository, db *sqlx.DB) Service {
	return &service{repo: repo, db: db}
}

func (s *service) CreateFY(ctx context.Context, req *RqCreateFY) (*RsFinancialYear, error) {
	// Parse fy_year (e.g., "2025-2026")
	years := strings.Split(req.FYYear, "-")
	if len(years) != 2 {
		return nil, ErrInvalidFYYearFormat
	}

	startYear := years[0]
	endYear := years[1]

	// Create start_date: 01-07-startYear
	startDate, err := time.Parse("02-01-2006", fmt.Sprintf("01-07-%s", startYear))
	if err != nil {
		return nil, ErrInvalidFYYearFormat
	}

	// Create end_date: 30-06-endYear
	endDate, err := time.Parse("02-01-2006", fmt.Sprintf("30-06-%s", endYear))
	if err != nil {
		return nil, ErrInvalidFYYearFormat
	}

	// create transection

	var createdFY *FinancialYear

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		// If is_active is true, deactivate all other financial years
		if req.IsActive {
			if err := s.repo.DeactivateAllFinancialYears(ctx, tx); err != nil {
				return fmt.Errorf("deactivate existing financial years: %w", err)
			}
		}
		// Create financial year
		fy := &FinancialYear{
			Label:     req.Label,
			IsActive:  req.IsActive,
			StartDate: startDate,
			EndDate:   endDate,
		}

		newFY, err := s.repo.CreateFinancialYear(ctx, fy, tx)
		if err != nil {
			return fmt.Errorf("create financial year: %w", err)
		}
		createdFY = newFY
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
				return fmt.Errorf("parse quarter start date: %w", err)
			}

			qEndDate, err := time.Parse("02-01-2006", fmt.Sprintf("%s-%s", q.endDate, year))
			if err != nil {
				return fmt.Errorf("parse quarter end date: %w", err)
			}

			quarter := &FinancialQuarter{
				FinancialYearID: newFY.ID,
				Label:           q.label,
				StartDate:       qStartDate,
				EndDate:         qEndDate,
			}

			if _, err := s.repo.CreateFinancialQuarter(ctx, quarter, tx); err != nil {
				return fmt.Errorf("create quarter %s: %w", q.label, err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &RsFinancialYear{
		ID:        createdFY.ID,
		Label:     createdFY.Label,
		StartDate: createdFY.StartDate,
		EndDate:   createdFY.EndDate,
	}, nil
}

func (s *service) UpdateFYLabel(ctx context.Context, id uuid.UUID, req *RqUpdateFYLabel) (*RsFinancialYear, error) {
	fy, err := s.repo.GetFinancialYearByID(ctx, id)
	if err != nil {
		return nil, err
	}

	fy.Label = req.Label
	var updatedFY *FinancialYear

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		// If is_active is provided and set to true, deactivate all other financial years
		if req.IsActive != nil && *req.IsActive {
			if err := s.repo.DeactivateAllFinancialYears(ctx, tx); err != nil {
				return fmt.Errorf("deactivate existing financial years: %w", err)
			}
			fy.IsActive = true
		} else if req.IsActive != nil {
			fy.IsActive = *req.IsActive
		}

		up, err := s.repo.UpdateFinancialYear(ctx, fy, tx)
		if err != nil {
			return fmt.Errorf("update financial year: %w", err)
		}
		updatedFY = up

		return nil
	})
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
	if _, err := s.repo.GetFinancialYearByID(ctx, financialYearID); err != nil {
		return nil, err
	}
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
