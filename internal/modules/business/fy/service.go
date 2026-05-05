package fy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type Service interface {
	CreateFY(ctx context.Context, req *RqCreateFY) (*RsFinancialYear, error)
	UpdateFY(ctx context.Context, id uuid.UUID, req *RqUpdateFY) (*RsFinancialYear, error)
	GetFinancialYears(ctx context.Context) ([]RsFY, error)
	GetFinancialQuarters(ctx context.Context, financialYearID uuid.UUID) ([]RsFinancialQuarter, error)
	ActivateFY(ctx context.Context, id uuid.UUID) (*RsFinancialYear, error)
	GetFinancialYearByID(ctx context.Context, id uuid.UUID) (*RsFY, error)
}

type service struct {
	repo     Repository
	db       *sqlx.DB
	auditSvc audit.Service
}

func NewService(repo Repository, db *sqlx.DB, auditSvc audit.Service) Service {
	return &service{repo: repo, db: db, auditSvc: auditSvc}
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
			if err := s.repo.DeactivateAllFinancialYearsExcept(ctx, tx, uuid.Nil); err != nil {
				return fmt.Errorf("deactivate existing financial years: %w", err)
			}
		} else {
			// If no financial Year is active, set this one active by default
			count, _ := s.repo.CountActiveFY(ctx, tx)
			if count == 0 {
				req.IsActive = true
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
		s.auditSvc.LogSystemIssue(ctx, auditctx.ActionSystemError, "fy.creation_failed",
			err, "", req.Label, auditctx.EntityFinancialYear, auditctx.ModuleBusiness)
		return nil, err
	}

	result := &RsFinancialYear{
		ID:        createdFY.ID,
		Label:     createdFY.Label,
		StartDate: createdFY.StartDate,
		EndDate:   createdFY.EndDate,
	}

	// Audit log: FY created
	meta := auditctx.GetMetadata(ctx)
	idStr := createdFY.ID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: meta.PracticeID,
		UserID:     meta.UserID,
		Action:     auditctx.ActionFYCreated,
		Module:     auditctx.ModuleBusiness,
		EntityType: strPtr(auditctx.EntityFinancialYear),
		EntityID:   &idStr,
		AfterState: result,
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	return result, nil
}

func (s *service) UpdateFY(ctx context.Context, id uuid.UUID, req *RqUpdateFY) (*RsFinancialYear, error) {
	fy, err := s.repo.GetFinancialYearByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Handle Label Update
	if req.Label != nil && strings.TrimSpace(*req.Label) != "" {
		fy.Label = strings.TrimSpace(*req.Label)
	}

	// Handle FYYear (Dates) Update
	var startYear, endYear string
	datesChanged := false
	if req.FYYear != "" {
		years := strings.Split(req.FYYear, "-")
		if len(years) != 2 {
			return nil, ErrInvalidFYYearFormat
		}
		startYear, endYear = years[0], years[1]

		fy.StartDate, _ = time.Parse("02-01-2006", fmt.Sprintf("01-07-%s", startYear))
		fy.EndDate, _ = time.Parse("02-01-2006", fmt.Sprintf("30-06-%s", endYear))
		datesChanged = true
	}

	var updatedFY *FinancialYear
	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		// Only 1 Active Financial Year is allowed
		if req.IsActive != nil {
			if *req.IsActive {
				// Deactivate others if this one is being set to active
				if err := s.repo.DeactivateAllFinancialYearsExcept(ctx, tx, id); err != nil {
					return err
				}
				fy.IsActive = true
			} else {
				// Check if we are trying to deactivate the ONLY active FY
				count, err := s.repo.CountActiveFY(ctx, tx)
				if err != nil {
					return err
				}
				if fy.IsActive && count <= 1 {
					return errors.New("cannot deactivate the only active financial year; at least one must be active")
				}
				fy.IsActive = false
			}
		}

		// Update the FY record
		updated, err := s.repo.UpdateFinancialYear(ctx, fy, tx)
		if err != nil {
			return err
		}
		updatedFY = updated

		// If dates changed, we must recreate the quarters
		if datesChanged {
			if err := s.repo.DeleteQuartersByFYID(ctx, id, tx); err != nil {
				return err
			}

			quarters := []struct {
				label  string
				sD     string
				eD     string
				useEnd bool
			}{
				{"Q1", "01-07", "30-09", false},
				{"Q2", "01-10", "31-12", false},
				{"Q3", "01-01", "31-03", true},
				{"Q4", "01-04", "30-06", true},
			}

			for _, q := range quarters {
				year := startYear
				if q.useEnd {
					year = endYear
				}
				qS, _ := time.Parse("02-01-2006", fmt.Sprintf("%s-%s", q.sD, year))
				qE, _ := time.Parse("02-01-2006", fmt.Sprintf("%s-%s", q.eD, year))

				if _, err := s.repo.CreateFinancialQuarter(ctx, &FinancialQuarter{
					FinancialYearID: id,
					Label:           q.label,
					StartDate:       qS,
					EndDate:         qE,
				}, tx); err != nil {
					return err
				}
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	result := &RsFinancialYear{
		ID:        updatedFY.ID,
		Label:     updatedFY.Label,
		StartDate: updatedFY.StartDate,
		EndDate:   updatedFY.EndDate,
	}

	// Audit log: FY updated
	meta := auditctx.GetMetadata(ctx)
	idStr := id.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: meta.PracticeID,
		UserID:     meta.UserID,
		Action:     auditctx.ActionFYUpdated,
		Module:     auditctx.ModuleBusiness,
		EntityType: strPtr(auditctx.EntityFinancialYear),
		EntityID:   &idStr,
		AfterState: result,
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	return result, nil
}

func (s *service) GetFinancialYears(ctx context.Context) ([]RsFY, error) {
	years, err := s.repo.GetFinancialYears(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]RsFY, 0, len(years))
	for _, year := range years {
		status := "Inactive"
		if year.IsActive {
			status = "Active"
		}
		result = append(result, RsFY{
			ID:        year.ID,
			Label:     year.Label,
			FYYear:    fmt.Sprintf("%d-%d", year.StartDate.Year(), year.EndDate.Year()),
			Status:    status,
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

func (s *service) ActivateFY(ctx context.Context, id uuid.UUID) (*RsFinancialYear, error) {
	var updatedFY *FinancialYear

	err := util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		// Fetch the target FY to ensure it exists
		fy, err := s.repo.GetFinancialYearByID(ctx, id)
		if err != nil {
			return err
		}

		// Deactivate everything currently active
		if err := s.repo.DeactivateAllFinancialYearsExcept(ctx, tx, id); err != nil {
			s.auditSvc.LogSystemIssue(ctx, auditctx.ActionSystemError, "fy.activation_failed",
				err, "", id.String(), auditctx.EntityFinancialYear, auditctx.ModuleBusiness)
			return err
		}

		// Set to active and update
		fy.IsActive = true
		updated, err := s.repo.UpdateFinancialYear(ctx, fy, tx)
		if err != nil {
			return err
		}
		updatedFY = updated
		return nil
	})

	if err != nil {
		return nil, err
	}

	result := &RsFinancialYear{
		ID:        updatedFY.ID,
		Label:     updatedFY.Label,
		StartDate: updatedFY.StartDate,
		EndDate:   updatedFY.EndDate,
	}

	// Success Audit Log
	meta := auditctx.GetMetadata(ctx)
	idStr := id.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: meta.PracticeID,
		UserID:     meta.UserID,
		Action:     auditctx.ActionFYActivated,
		Module:     auditctx.ModuleBusiness,
		EntityType: strPtr(auditctx.EntityFinancialYear),
		EntityID:   &idStr,
		AfterState: result,
	})

	return result, nil
}

func (s *service) GetFinancialYearByID(ctx context.Context, id uuid.UUID) (*RsFY, error) {
	fy, err := s.repo.GetFinancialYearByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Calculate fy_year string: e.g., "2025-2026"
	fyYearStr := fmt.Sprintf("%d-%d", fy.StartDate.Year(), fy.EndDate.Year())

	// Map boolean to status string
	status := "Inactive"
	if fy.IsActive {
		status = "Active"
	}

	return &RsFY{
		ID:        fy.ID,
		Label:     fy.Label,
		FYYear:    fyYearStr,
		Status:    status,
		StartDate: fy.StartDate,
		EndDate:   fy.EndDate,
	}, nil
}
func strPtr(s string) *string { return &s }
