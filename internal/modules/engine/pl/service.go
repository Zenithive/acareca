package pl

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/business/clinic"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type Service interface {
	GetMonthlySummary(ctx context.Context, f *PLFilter) ([]RsPLSummary, error)
	GetByAccount(ctx context.Context, f *PLFilter) ([]RsPLAccount, error)
	GetByResponsibility(ctx context.Context, f *PLFilter) ([]RsPLResponsibility, error)
	GetFYSummary(ctx context.Context, f *PLFilter) ([]RsPLFYSummary, error)
	GetReport(ctx context.Context, actorID uuid.UUID, f *PLReportFilter) (*RsReport, error)
}

type service struct {
	repo           Repository
	clinicRepo     clinic.Repository
	accountantRepo accountant.Repository

	practitionerSvc practitioner.IService
}

func NewService(repo Repository, clinicRepo clinic.Repository, accountantRepo accountant.Repository, practitionerSvc practitioner.IService) Service {
	return &service{repo: repo, clinicRepo: clinicRepo, accountantRepo: accountantRepo, practitionerSvc: practitionerSvc}
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

func (s *service) GetReport(ctx context.Context, actorID uuid.UUID, f *PLReportFilter) (*RsReport, error) {

	meta := auditctx.GetMetadata(ctx)

	isAccountant := false
	if meta.UserType != nil {
		isAccountant = strings.EqualFold(*meta.UserType, util.RoleAccountant)
	}

	accProfile, err := s.accountantRepo.GetAccountantByUserID(ctx, actorID.String())
	if err == nil && accProfile != nil {
		isAccountant = true
	}

	var finalOwnerID uuid.UUID

	if isAccountant {
		if accProfile == nil {
			return nil, fmt.Errorf("access denied: accountant profile not found")
		}

		if f.ClinicID != nil && *f.ClinicID != "" {
			clinicUUID, err := uuid.Parse(*f.ClinicID)
			if err != nil {
				return nil, fmt.Errorf("invalid clinic_id format")
			}
			permission, err := s.clinicRepo.GetAccountantPermission(ctx, accProfile.ID, clinicUUID)
			if err != nil {
				return nil, fmt.Errorf("permission denied: you are not associated with this clinic")
			}
			finalOwnerID = permission.PractitionerID
		} else {
			// Case B: Practice-wide
			if f.PractitionerID == "" {
				targetPracID, err := s.clinicRepo.GetPractitionerForAccountant(ctx, accProfile.ID)
				if err != nil {
					return nil, fmt.Errorf("no linked practitioner found: please provide a practitioner_id")
				}
				finalOwnerID = *targetPracID
			} else {
				targetPracID, err := uuid.Parse(f.PractitionerID)
				if err != nil {
					return nil, fmt.Errorf("invalid practitioner_id format")
				}
				isLinked, err := s.clinicRepo.IsAccountantInvitedByPractitioner(ctx, accProfile.ID, targetPracID)
				if err != nil || !isLinked {
					return nil, fmt.Errorf("permission denied: no association with this practitioner")
				}
				finalOwnerID = targetPracID
			}
		}
	} else {

		pracProfile, err := s.practitionerSvc.GetPractitionerByUserID(ctx, actorID.String())
		if err != nil {
			return nil, fmt.Errorf("access denied: practitioner profile not found")
		}
		finalOwnerID = pracProfile.ID

		// Verify clinic ownership if a specific one is requested
		if f.ClinicID != nil && *f.ClinicID != "" {
			clinicUUID, err := uuid.Parse(*f.ClinicID)
			if err != nil {
				return nil, fmt.Errorf("invalid clinic_id format")
			}

			_, err = s.clinicRepo.GetClinicByIDAndPractitioner(ctx, clinicUUID, finalOwnerID)
			if err != nil {
				return nil, fmt.Errorf("access denied: clinic not found or ownership mismatch")
			}
		}
	}

	// 3. APPLY VERIFIED PRACTITIONER ID (OUTSIDE all if/else blocks)
	f.PractitionerID = finalOwnerID.String()

	/*
		if f.ClinicID != nil {
			if _, err := uuid.Parse(*f.ClinicID); err != nil {
				return nil, fmt.Errorf("invalid clinic_id: must be a valid UUID")
			}
		}*/
	var from, to time.Time
	//var err error
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
		// Treat NULL section_type as 'COST' (operating expenses)
		sectionType := "COST"
		if r.SectionType != nil {
			sectionType = *r.SectionType
		}

		// Use net_amount consistently across all sections for P&L reporting.
		// P&L should show revenue and expenses on a GST-exclusive basis:
		// - Income: NET (actual revenue earned, GST is collected for government)
		// - Costs: NET (actual expenses, GST can be claimed back)
		// This aligns with standard accounting practice where GST is a pass-through.
		val := r.NetAmount

		ck := coaKey{sectionType, r.CoaID}
		if !coaSeen[ck] {
			coaSeen[ck] = true
			coaOrder[sectionType] = append(coaOrder[sectionType], r.CoaID)
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
