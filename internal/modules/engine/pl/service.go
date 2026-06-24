package pl

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/auth"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/business/admin"
	"github.com/iamarpitzala/acareca/internal/modules/business/clinic"
	"github.com/iamarpitzala/acareca/internal/modules/business/invitation"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	"github.com/iamarpitzala/acareca/internal/modules/notification"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/export"
	plexport "github.com/iamarpitzala/acareca/internal/shared/export/pl"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
	"github.com/xuri/excelize/v2"
)

type Service interface {
	GetMonthlySummary(ctx context.Context, f *PLFilter) ([]RsPLSummary, error)
	GetByAccount(ctx context.Context, f *PLFilter) ([]RsPLAccount, error)
	GetByResponsibility(ctx context.Context, f *PLFilter) ([]RsPLResponsibility, error)
	GetFYSummary(ctx context.Context, f *PLFilter) ([]RsPLFYSummary, error)
	GetReport(ctx context.Context, actorID uuid.UUID, f *PLReportFilter, role string, targetNotifIDs []uuid.UUID, userID uuid.UUID) (*RsReport, error)
	ExportPLReport(ctx context.Context, data []*RsReport, exportType string, actorID uuid.UUID, role string, userID uuid.UUID, notifIDs []uuid.UUID, filterPractitionerID string) (*excelize.File, error)
}

type service struct {
	repo            Repository
	clinicRepo      clinic.Repository
	accountantRepo  accountant.Repository
	practitionerSvc practitioner.IService
	authRepo        auth.Repository
	auditSvc        audit.Service
	notificationPub *sharednotification.Publisher
	invitationRepo  invitation.Repository
	authSvc         auth.Service
}

func NewService(repo Repository, clinicRepo clinic.Repository, accountantRepo accountant.Repository, practitionerSvc practitioner.IService, authRepo auth.Repository, auditSvc audit.Service, invitationRepo invitation.Repository, authSvc auth.Service, notificationSvc notification.Service, adminRepo admin.Repository) Service {
	return &service{
		repo:            repo,
		clinicRepo:      clinicRepo,
		accountantRepo:  accountantRepo,
		practitionerSvc: practitionerSvc,
		authRepo:        authRepo,
		auditSvc:        auditSvc,
		notificationPub: sharednotification.NewPublisher(notification.NewServiceAdapter(notificationSvc), adminRepo),
		invitationRepo:  invitationRepo,
		authSvc:         authSvc,
	}
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

const dateLayout = "2006-01-02"

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

func (s *service) GetReport(ctx context.Context, actorID uuid.UUID, f *PLReportFilter, role string, targetNotifIDs []uuid.UUID, userID uuid.UUID) (*RsReport, error) {
	isAccountant := strings.EqualFold(role, util.RoleAccountant)

	var targetPracIDs []uuid.UUID
	var rows []*PLReportRow
	var summary *PLSummaryRow

	err := util.RunInTransaction(ctx, s.repo.(*repository).db, func(ctx context.Context, tx *sqlx.Tx) error {
		var innerErr error

		if isAccountant {
			if f.ClinicID != nil && *f.ClinicID != "" {
				clinicUUID, innerErr := uuid.Parse(*f.ClinicID)
				if innerErr != nil {
					return fmt.Errorf("invalid clinic_id format")
				}
				permission, innerErr := s.clinicRepo.GetAccountantPermission(ctx, actorID, clinicUUID)
				if innerErr != nil {
					return fmt.Errorf("permission denied: not associated with this clinic")
				}
				targetPracIDs = []uuid.UUID{permission.PractitionerID}
			} else if f.PractitionerID != "" {
				pracUUID, innerErr := uuid.Parse(f.PractitionerID)
				if innerErr != nil {
					return fmt.Errorf("invalid practitioner_id format")
				}
				isLinked, innerErr := s.clinicRepo.IsAccountantInvitedByPractitioner(ctx, actorID, pracUUID)
				if innerErr != nil || !isLinked {
					return fmt.Errorf("permission denied: no association with this practitioner")
				}
				targetPracIDs = []uuid.UUID{pracUUID}
			} else {
				if len(targetNotifIDs) == 0 {
					return fmt.Errorf("no linked practitioners found for aggregation")
				}
				targetPracIDs = targetNotifIDs
			}
		} else {
			if f.ClinicID != nil && *f.ClinicID != "" {
				clinicUUID, innerErr := uuid.Parse(*f.ClinicID)
				if innerErr == nil {
					_, innerErr = s.clinicRepo.GetClinicByIDAndPractitioner(ctx, tx, clinicUUID, actorID)
					if innerErr != nil {
						return fmt.Errorf("access denied: clinic mismatch")
					}
				}
			}
			targetPracIDs = []uuid.UUID{actorID}
		}

		if len(targetPracIDs) > 0 {
			f.PractitionerID = targetPracIDs[0].String()
		}

		var from, to time.Time
		if f.DateFrom != nil {
			if from, innerErr = time.Parse(dateLayout, *f.DateFrom); innerErr != nil {
				return fmt.Errorf("invalid date_from: use YYYY-MM-DD format")
			}
		}
		if f.DateUntil != nil {
			if to, innerErr = time.Parse(dateLayout, *f.DateUntil); innerErr != nil {
				return fmt.Errorf("invalid date_until: use YYYY-MM-DD format")
			}
		}
		if f.DateFrom != nil && f.DateUntil != nil && from.After(to) {
			return fmt.Errorf("date_from must not be after date_until")
		}

		rows, innerErr = s.repo.GetReport(ctx, targetPracIDs, f)
		if innerErr != nil {
			return innerErr
		}

		summary, innerErr = s.repo.GetPLSummary(ctx, targetPracIDs, f)
		if innerErr != nil {
			return innerErr
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	userIDStr := userID.String()
	parsedActorID := actorID.String()

	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		PracticeID: nil,
		UserID:     &userIDStr,
		Action:     auditctx.ActionPLReportGenerated,
		Module:     auditctx.ModuleReport,
		EntityType: lo.ToPtr(auditctx.EntityPLReport),
		EntityID:   &parsedActorID,
		AfterState: map[string]interface{}{
			"report_type": "Profit and Loss Report",
		},
	})

	// Send notification for report generation
	if err := s.notifyReportExport(ctx, actorID, util.ActorType(role), util.EventPLReportGenerated, "P&L Report", targetPracIDs); err != nil {
		log.Printf("[WARN] failed to send P&L report generation notification: %v", err)
	}

	return buildReport(f, rows, summary), nil
}

func buildReport(f *PLReportFilter, rows []*PLReportRow, summary *PLSummaryRow) *RsReport {
	type coaKey struct {
		plSection string
		coaID     string
	}
	coaOrder := map[string][]string{}
	coaSeen := map[coaKey]bool{}
	coaNames := map[coaKey]string{}
	coaTotals := map[coaKey]float64{}

	for _, r := range rows {
		plSection := r.PLSection
		if plSection == "" {
			plSection = "3. Other Expenses"
		}

		val := r.NetAmount
		ck := coaKey{plSection, r.CoaID}
		if !coaSeen[ck] {
			coaSeen[ck] = true
			coaOrder[plSection] = append(coaOrder[plSection], r.CoaID)
			coaNames[ck] = r.AccountName
		}
		coaTotals[ck] += val
	}

	buildGroup := func(plSections ...string) RsReportGroup {
		accounts := make([]RsReportAccount, 0)
		var total float64
		for _, section := range plSections {
			for _, cid := range coaOrder[section] {
				ck := coaKey{section, cid}
				total += coaTotals[ck]
				accounts = append(accounts, RsReportAccount{
					CoaID:      cid,
					CoaName:    coaNames[ck],
					TotalValue: round2(coaTotals[ck]),
				})
			}
		}
		return RsReportGroup{GroupTotal: round2(total), Accounts: accounts}
	}

	income := buildGroup("1. Income")
	cos := buildGroup("2. Cost of Sales")
	otherCosts := buildGroup("3. Other Costs")

	grossProfit := round2(summary.GrossProfitNet)
	netProfit := round2(summary.NetProfitNet)

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
		OtherCosts:  otherCosts,
		NetProfit:   netProfit,
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func (s *service) ExportPLReport(ctx context.Context, data []*RsReport, exportType string, actorID uuid.UUID, role string, userID uuid.UUID, notifIDs []uuid.UUID, filterPractitionerID string) (*excelize.File, error) {
	var fullName string
	user, err := s.authRepo.FindByID(ctx, userID)
	if err == nil {
		fullName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
	}

	var entityName string
	var practitionerABN string
	targetID := filterPractitionerID
	if targetID == "" && role == util.RolePractitioner {
		targetID = actorID.String()
	}

	if targetID != "" {
		if prac, err := s.practitionerSvc.GetPractitioner(ctx, uuid.MustParse(targetID)); err == nil {
			if prac.EntityName != nil {
				entityName = *prac.EntityName
			} else {
				entityName = fullName
			}
			if prac.ABN != nil {
				practitionerABN = *prac.ABN
			}
		}
	} else {
		if acc, err := s.accountantRepo.GetAccountantByUserID(ctx, userID.String()); err == nil {
			if acc.EntityName != nil {
				entityName = *acc.EntityName
			} else {
				entityName = fullName
			}
			if acc.ABN != nil {
				practitionerABN = *acc.ABN
			}
		}
	}

	periodText := ""
	baseline := data[0]
	if len(data) > 1 {
		if baseline.ReportMetadata.DateUntil != "" {
			periodText = fmt.Sprintf("As of %s (with %d Comparative Periods)", formatDateStr(baseline.ReportMetadata.DateUntil), len(data))
		}
	} else {
		if baseline.ReportMetadata.DateFrom != "" && baseline.ReportMetadata.DateUntil != "" {
			periodText = fmt.Sprintf("%s to %s", formatDateStr(baseline.ReportMetadata.DateFrom), formatDateStr(baseline.ReportMetadata.DateUntil))
		}
	}

	config := export.ExportConfig{
		EntityName:     entityName,
		EntityABN:      practitionerABN,
		Period:         periodText,
		ExportType:     exportType,
		ExportedByName: fullName,
		GeneratedTime:  time.Now().Format("02/01/2006, 3:04:05 pm"),
	}

	exportSlice := make([]*plexport.RsReport, len(data))
	for i, d := range data {
		exportSlice[i] = &plexport.RsReport{
			ReportMetadata: plexport.RsReportMetadata{
				DateFrom:         d.ReportMetadata.DateFrom,
				DateUntil:        d.ReportMetadata.DateUntil,
				OverallNetProfit: d.ReportMetadata.OverallNetProfit,
			},
			Income: plexport.RsReportGroup{
				GroupTotal: d.Income.GroupTotal,
				Accounts:   make([]plexport.RsReportAccount, len(d.Income.Accounts)),
			},
			CostOfSales: plexport.RsReportGroup{
				GroupTotal: d.CostOfSales.GroupTotal,
				Accounts:   make([]plexport.RsReportAccount, len(d.CostOfSales.Accounts)),
			},
			GrossProfit: d.GrossProfit,
			OtherCosts: plexport.RsReportGroup{
				GroupTotal: d.OtherCosts.GroupTotal,
				Accounts:   make([]plexport.RsReportAccount, len(d.OtherCosts.Accounts)),
			},
			NetProfit: d.NetProfit,
		}

		for j, acc := range d.Income.Accounts {
			exportSlice[i].Income.Accounts[j] = plexport.RsReportAccount{
				CoaID:      acc.CoaID,
				CoaName:    acc.CoaName,
				TotalValue: acc.TotalValue,
			}
		}
		for j, acc := range d.CostOfSales.Accounts {
			exportSlice[i].CostOfSales.Accounts[j] = plexport.RsReportAccount{
				CoaID:      acc.CoaID,
				CoaName:    acc.CoaName,
				TotalValue: acc.TotalValue,
			}
		}
		for j, acc := range d.OtherCosts.Accounts {
			exportSlice[i].OtherCosts.Accounts[j] = plexport.RsReportAccount{
				CoaID:      acc.CoaID,
				CoaName:    acc.CoaName,
				TotalValue: acc.TotalValue,
			}
		}
	}

	// Send notification
	if err := s.notifyReportExport(ctx, actorID, util.ActorType(role), util.EventPLReportExport, "P&L Report", notifIDs); err != nil {
		log.Printf("[WARN] failed to send P&L report notification: %v", err)
	}

	return plexport.GenerateExcelReport(exportSlice, config)
}

func formatDateStr(dateStr string) string {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return dateStr
	}
	return t.Format("02-01-2006")
}

// notifyReportExport sends notifications to linked users about report export
// targetPractitionerIDs: optional list of specific practitioners for whom the report was generated
// If nil or empty when actor is accountant, notifies all linked practitioners
func (s *service) notifyReportExport(ctx context.Context, entityID uuid.UUID, actorType util.ActorType, eventType util.EventType, reportName string, targetPractitionerIDs []uuid.UUID) error {
	if s.notificationPub == nil {
		return fmt.Errorf("notification publisher is nil")
	}

	user, err := s.authSvc.GetUserByID(ctx, entityID, actorType)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}

	senderName := user.FirstName + " " + user.LastName
	senderType := actorType

	recipients := []sharednotification.RecipientWithPreferences{}

	var action string

	switch eventType {
	case util.EventPLReportGenerated:
		action = "Generated"
	case util.EventPLReportExport:
		action = "Exported"
	default:
		action = "Updated"
	}

	switch actorType {
	case util.ActorPractitioner:
		// Notify linked accountants
		accountants, err := s.invitationRepo.GetAccountantsLinkedToPractitioner(ctx, entityID)
		if err != nil {
			log.Printf("[WARN] failed to get linked accountants for practitioner %s: %v", entityID, err)
			return nil
		}

		for _, acc := range accountants {
			recipients = append(recipients, sharednotification.RecipientWithPreferences{
				RecipientID:   acc.AccountantID,
				RecipientType: util.ActorAccountant,
				UserID:        acc.UserID,
			})
		}

	case util.ActorAccountant:
		// Notify only specific practitioners if targetPractitionerIDs is provided
		var practitionerIDs []uuid.UUID
		
		if len(targetPractitionerIDs) > 0 {
			// Use the specific practitioners for whom the report was generated
			practitionerIDs = targetPractitionerIDs
		} else {
			// Fallback: notify all linked practitioners
			var err error
			practitionerIDs, err = s.invitationRepo.GetPractitionersLinkedToAccountant(ctx, entityID)
			if err != nil {
				log.Printf("[WARN] failed to get practitioners for accountant %s: %v", entityID, err)
				return nil
			}
		}

		for _, practitionerID := range practitionerIDs {
			practitionerUserID, err := s.invitationRepo.GetPractitionerUserIDByID(ctx, practitionerID)
			if err != nil {
				log.Printf("[WARN] failed to get user ID for practitioner %s: %v", practitionerID, err)
				continue
			}

			recipients = append(recipients, sharednotification.RecipientWithPreferences{
				RecipientID:   practitionerID,
				RecipientType: util.ActorPractitioner,
				UserID:        practitionerUserID,
			})
		}

	default:
		return fmt.Errorf("unsupported actor type: %s", actorType)
	}

	if len(recipients) == 0 {
		log.Printf("[INFO] no recipients found for report notification")
		return nil
	}

	return s.notificationPub.Publish(ctx, sharednotification.PublishRequest{
		Recipients: recipients,
		SenderID:   entityID,
		SenderType: senderType,
		SenderName: senderName,
		EventType:  eventType,
		EntityType: util.EntityReport,
		EntityID:   entityID,
		EntityKey:  "report_id",
		Title:      fmt.Sprintf("%s %s", reportName, action),
		Body:       fmt.Sprintf("%s %s by %s", reportName, strings.ToLower(action), senderName),
		ExtraData:  map[string]interface{}{"report_name": reportName},
	})
}
