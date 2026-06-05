package bs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/auth"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/business/equity"
	"github.com/iamarpitzala/acareca/internal/modules/business/invitation"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	"github.com/iamarpitzala/acareca/internal/modules/business/shared/events"
	"github.com/iamarpitzala/acareca/internal/shared/export"
	bsexport "github.com/iamarpitzala/acareca/internal/shared/export/bs"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"

	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
)

type Service interface {
	GetBalanceSheet(ctx context.Context, f *BSFilter, actorID uuid.UUID, role string, userID uuid.UUID) (*RsBalanceSheet, error)
	ExportBalanceSheet(ctx context.Context, data *RsBalanceSheet, exportType string, actorID uuid.UUID, role string, userID uuid.UUID, notifIDs []uuid.UUID, filterPractitionerID string) (*ExportBalanceSheetResponse, error)
}

type service struct {
	repo            Repository
	equitySvc       equity.Service
	db              *sqlx.DB
	auditSvc        audit.Service
	eventsSvc       events.Service
	authRepo        auth.Repository
	invitationSvc   invitation.Service
	accountantRepo  accountant.Repository
	practitionerSvc practitioner.IService
}

func NewService(repo Repository, equitySvc equity.Service, db *sqlx.DB, auditSvc audit.Service, eventsSvc events.Service, authRepo auth.Repository, invitationSvc invitation.Service, accountantRepo accountant.Repository, practitionerSvc practitioner.IService) Service {
	return &service{
		repo:            repo,
		equitySvc:       equitySvc,
		db:              db,
		auditSvc:        auditSvc,
		eventsSvc:       eventsSvc,
		authRepo:        authRepo,
		invitationSvc:   invitationSvc,
		accountantRepo:  accountantRepo,
		practitionerSvc: practitionerSvc,
	}
}

type AccKey struct {
	Code int16
	Name string
}

func (s *service) GetBalanceSheet(ctx context.Context, f *BSFilter, actorID uuid.UUID, role string, userID uuid.UUID) (*RsBalanceSheet, error) {
	targetPracIDs, practitionerID, err := s.resolveTargetPractitioners(ctx, actorID, role, f)
	if err != nil {
		return nil, err
	}

	endDate := s.resolveEndDate(f)

	rows, err := s.repo.GetBalanceSheet(ctx, targetPracIDs, f)
	if err != nil {
		return nil, err
	}

	totalOwnerEquity, err := s.calculateTotalEquity(ctx, targetPracIDs, endDate)
	if err != nil {
		return nil, err
	}

	result := s.buildBalanceSheet(ctx, rows, totalOwnerEquity, practitionerID, endDate)
	
	s.logBalanceSheetGeneration(ctx, actorID, userID, role, endDate, targetPracIDs)

	return result, nil
}

func (s *service) resolveTargetPractitioners(ctx context.Context, actorID uuid.UUID, role string, f *BSFilter) ([]uuid.UUID, uuid.UUID, error) {
	switch role {
	case util.RolePractitioner:
		return []uuid.UUID{actorID}, actorID, nil

	case util.RoleAccountant:
		if f.PractitionerID != nil && *f.PractitionerID != "" {
			pID, err := uuid.Parse(*f.PractitionerID)
			if err != nil {
				return nil, uuid.Nil, err
			}
			return []uuid.UUID{pID}, pID, nil
		}
		return []uuid.UUID{}, uuid.Nil, nil

	default:
		linked, err := s.invitationSvc.GetPractitionersLinkedToAccountant(ctx, actorID)
		if err != nil {
			return nil, uuid.Nil, fmt.Errorf("failed to fetch linked practitioners: %w", err)
		}
		if len(linked) == 0 {
			return nil, uuid.Nil, errors.New("no linked practitioners found")
		}
		return linked, linked[0], nil
	}
}

func (s *service) resolveEndDate(f *BSFilter) string {
	if f.EndDate != nil && *f.EndDate != "" {
		return *f.EndDate
	}
	return time.Now().Format("2006-01-02")
}

func (s *service) calculateTotalEquity(ctx context.Context, practitionerIDs []uuid.UUID, endDate string) (equity.OwnerEquityCalculation, error) {
	var total equity.OwnerEquityCalculation
	for _, pID := range practitionerIDs {
		pracEquity, err := s.equitySvc.CalculateOwnerEquity(ctx, pID, nil, "", endDate)
		if err != nil {
			return total, fmt.Errorf("calculate owner equity: %w", err)
		}
		total.ShareCapital += pracEquity.ShareCapital
		total.FundsIntroduced += pracEquity.FundsIntroduced
		total.Drawings += pracEquity.Drawings
		total.RetainedEarnings += pracEquity.RetainedEarnings
		total.CurrentYearProfit += pracEquity.CurrentYearProfit
		total.TotalEquity += pracEquity.TotalEquity
	}
	return total, nil
}

func (s *service) buildBalanceSheet(ctx context.Context, rows []*BSRow, equity equity.OwnerEquityCalculation, practitionerID uuid.UUID, endDate string) *RsBalanceSheet {
	assetMap := make(map[AccKey]RsAccount)
	liabMap := make(map[AccKey]RsAccount)
	otherEquityMap := make(map[AccKey]RsAccount)

	var totalAssets, totalLiabilities, totalOtherEquity float64

	for _, row := range rows {
		key := AccKey{Code: row.AccountCode, Name: row.AccountName}

		switch row.AccountType {
		case "Asset":
			acc := assetMap[key]
			acc.Code, acc.Name, acc.CoaId = row.AccountCode, row.AccountName, row.CoaID
			acc.Balance += row.Balance
			assetMap[key] = acc
			totalAssets += row.Balance

		case "Liability":
			acc := liabMap[key]
			acc.Code, acc.Name, acc.CoaId = row.AccountCode, row.AccountName, row.CoaID
			acc.Balance += row.Balance
			liabMap[key] = acc
			totalLiabilities += row.Balance

		case "Equity":
			if !s.isOwnerEquityAccount(row.AccountCode) {
				acc := otherEquityMap[key]
				acc.Code, acc.Name, acc.CoaId = row.AccountCode, row.AccountName, row.CoaID
				acc.Balance += row.Balance
				otherEquityMap[key] = acc
				totalOtherEquity += row.Balance
			}
		}
	}

	equitySect := s.buildEquitySection(ctx, otherEquityMap, equity, practitionerID)
	totalEquity := equity.TotalEquity + totalOtherEquity

	return &RsBalanceSheet{
		EndDate:                   formatDateForDisplay(endDate),
		Assets:                    mapToSlice(assetMap),
		TotalAssets:               totalAssets,
		Liabilities:               mapToSlice(liabMap),
		TotalLiabilities:          totalLiabilities,
		NetAssets:                 math.Round((totalAssets - totalLiabilities) * 100) / 100,
		Equity:                    equitySect,
		CurrentYearProfit:         equity.CurrentYearProfit,
		TotalEquity:               math.Round(totalEquity * 100) / 100,
		TotalLiabilitiesAndEquity: math.Round((totalLiabilities + totalEquity) * 100) / 100,
	}
}

func (s *service) isOwnerEquityAccount(code int16) bool {
	return code == 880 || code == 881 || code == 960 || code == 970
}

func (s *service) buildEquitySection(ctx context.Context, otherEquity map[AccKey]RsAccount, equity equity.OwnerEquityCalculation, practitionerID uuid.UUID) []RsAccount {
	equitySect := mapToSlice(otherEquity)
	
	addEquityItem := func(code int16, name string, balance float64) {
		if balance == 0 {
			return
		}
		coaID, err := s.getCoaIDByAccountCode(ctx, practitionerID, code)
		if err == nil && coaID != nil {
			equitySect = append(equitySect, RsAccount{
				CoaId:   *coaID,
				Code:    code,
				Name:    name,
				Balance: balance,
			})
		}
	}

	addEquityItem(970, "Owner Share Capital", equity.ShareCapital)
	addEquityItem(881, "Owner Funds Introduced", equity.FundsIntroduced)
	addEquityItem(880, "Owner Drawings", -equity.Drawings)
	addEquityItem(960, "Retained Earnings", equity.RetainedEarnings+equity.CurrentYearProfit)

	return equitySect
}

func (s *service) getCoaIDByAccountCode(ctx context.Context, practitionerID uuid.UUID, accountCode int16) (*uuid.UUID, error) {
	var coaID uuid.UUID
	query := `SELECT id FROM tbl_chart_of_accounts 
	          WHERE practitioner_id = $1 AND code = $2 AND deleted_at IS NULL LIMIT 1`
	
	err := s.db.QueryRowContext(ctx, query, practitionerID, accountCode).Scan(&coaID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("COA %d not found for practitioner %s", accountCode, practitionerID)
		}
		return nil, fmt.Errorf("get COA ID: %w", err)
	}
	return &coaID, nil
}

func (s *service) getUserFullName(ctx context.Context, userID uuid.UUID) string {
	user, err := s.authRepo.FindByID(ctx, userID)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s %s", user.FirstName, user.LastName)
}

func (s *service) formatDateDescription(endDate string) string {
	if endDate == "" {
		return ""
	}
	return fmt.Sprintf("as of %s", formatDateForDisplay(endDate))
}

func (s *service) recordAccountantEvents(ctx context.Context, actorID, userID uuid.UUID, eventType, action, endDate string, targetPracIDs []uuid.UUID, metadata events.JSONBMap) {
	fullName := s.getUserFullName(ctx, userID)
	if fullName == "" {
		return
	}

	dateDesc := s.formatDateDescription(endDate)
	description := fmt.Sprintf("Accountant %s %s %s", fullName, action, dateDesc)

	for _, pID := range targetPracIDs {
		_ = s.eventsSvc.Record(ctx, events.SharedEvent{
			ID:             uuid.New(),
			PractitionerID: pID,
			AccountantID:   actorID,
			ActorID:        userID,
			ActorName:      &fullName,
			ActorType:      util.RoleAccountant,
			EventType:      eventType,
			EntityType:     "REPORT",
			Description:    description,
			Metadata:       metadata,
			CreatedAt:      time.Now(),
		})
	}
}

func (s *service) logAudit(ctx context.Context, actorID, userID uuid.UUID, action string, afterState map[string]interface{}) {
	meta := auditctx.GetMetadata(ctx)
	userIDStr := userID.String()
	actorIDStr := actorID.String()

	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: nil,
		UserID:     &userIDStr,
		Action:     action,
		Module:     auditctx.ModuleReport,
		EntityType: lo.ToPtr(auditctx.EntityBalanceSheet),
		EntityID:   &actorIDStr,
		AfterState: afterState,
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})
}

func (s *service) logBalanceSheetGeneration(ctx context.Context, actorID, userID uuid.UUID, role, endDate string, targetPracIDs []uuid.UUID) {
	s.logAudit(ctx, actorID, userID, auditctx.ActitionBalanceSheetGenerated, map[string]interface{}{
		"report_type": "Balance Sheet",
		"end_date":    endDate,
	})

	if role == util.RoleAccountant {
		s.recordAccountantEvents(ctx, actorID, userID, "balance_sheet.generated", "generated Balance Sheet", endDate, targetPracIDs, 
			events.JSONBMap{"report_type": "Balance Sheet", "end_date": endDate})
	}
}

func (s *service) logBalanceSheetExport(ctx context.Context, actorID, userID uuid.UUID, role, exportType, endDate string, notifIDs []uuid.UUID) {
	s.logAudit(ctx, actorID, userID, auditctx.ActionBalanceSheetExported, map[string]interface{}{
		"report_type": "Balance Sheet",
		"export_type": exportType,
		"end_date":    endDate,
	})

	if role == util.RoleAccountant && len(notifIDs) > 0 {
		action := fmt.Sprintf("exported Balance Sheet (%s)", exportType)
		s.recordAccountantEvents(ctx, actorID, userID, "balance_sheet.exported", action, endDate, notifIDs,
			events.JSONBMap{"report_type": "Balance Sheet", "export_type": exportType, "end_date": endDate})
	}
}

func mapToSlice(m map[AccKey]RsAccount) []RsAccount {
	result := make([]RsAccount, 0, len(m))
	for _, v := range m {
		result = append(result, v)
	}
	return result
}

func formatDateForDisplay(dateStr string) string {
	if dateStr == "" {
		return ""
	}
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return dateStr
	}
	return t.Format("02-01-2006")
}

func (s *service) ExportBalanceSheet(ctx context.Context, data *RsBalanceSheet, exportType string, actorID uuid.UUID, role string, userID uuid.UUID, notifIDs []uuid.UUID, filterPractitionerID string) (*ExportBalanceSheetResponse, error) {
	fullName := s.getUserFullName(ctx, userID)
	entityName, practitionerABN := s.resolveEntityDetails(ctx, actorID, userID, role, filterPractitionerID, fullName)
	
	config := export.ExportConfig{
		EntityName:     entityName,
		EntityABN:      practitionerABN,
		Period:         s.formatPeriodText(data.EndDate),
		ExportType:     exportType,
		ExportedByName: fullName,
		GeneratedTime:  time.Now().Format("02/01/2006, 3:04:05 pm"),
	}

	exportData := s.prepareExportData(data)

	f, err := bsexport.GenerateExcelReport(exportData, config)
	if err != nil {
		return nil, fmt.Errorf("generate excel: %w", err)
	}

	s.logBalanceSheetExport(ctx, actorID, userID, role, exportType, data.EndDate, notifIDs)

	f.UpdateLinkedValue()
	return &ExportBalanceSheetResponse{Result: f}, nil
}

func (s *service) resolveEntityDetails(ctx context.Context, actorID, userID uuid.UUID, role, filterPractitionerID, fullName string) (string, string) {
	targetID := filterPractitionerID
	if targetID == "" && role == util.RolePractitioner {
		targetID = actorID.String()
	}

	if targetID != "" {
		if pracUUID, err := uuid.Parse(targetID); err == nil {
			if prac, err := s.practitionerSvc.GetPractitioner(ctx, pracUUID); err == nil {
				entityName := lo.FromPtrOr(prac.EntityName, fullName)
				return entityName, lo.FromPtrOr(prac.ABN, "")
			}
		}
	}

	if role != util.RolePractitioner {
		if acc, err := s.accountantRepo.GetAccountantByUserID(ctx, userID.String()); err == nil {
			entityName := lo.FromPtrOr(acc.EntityName, fullName)
			return entityName, lo.FromPtrOr(acc.ABN, "")
		}
	}

	return fullName, ""
}

func (s *service) formatPeriodText(endDate string) string {
	if endDate == "" {
		return ""
	}
	return fmt.Sprintf("As of %s", endDate)
}

func (s *service) prepareExportData(data *RsBalanceSheet) *bsexport.RsBalanceSheet {
	convertAccount := func(acc RsAccount) bsexport.RsAccount {
		return bsexport.RsAccount{CoaId: acc.CoaId, Code: acc.Code, Name: acc.Name, Balance: acc.Balance}
	}

	exportData := &bsexport.RsBalanceSheet{
		EndDate:                   data.EndDate,
		Assets:                    make([]bsexport.RsAccount, len(data.Assets)),
		TotalAssets:               data.TotalAssets,
		Liabilities:               make([]bsexport.RsAccount, len(data.Liabilities)),
		TotalLiabilities:          data.TotalLiabilities,
		Equity:                    make([]bsexport.RsAccount, len(data.Equity)),
		CurrentYearProfit:         data.CurrentYearProfit,
		TotalEquity:               data.TotalEquity,
		TotalLiabilitiesAndEquity: data.TotalLiabilities + data.TotalEquity,
	}

	for i, acc := range data.Assets {
		exportData.Assets[i] = convertAccount(acc)
	}
	for i, acc := range data.Liabilities {
		exportData.Liabilities[i] = convertAccount(acc)
	}
	for i, acc := range data.Equity {
		exportData.Equity[i] = convertAccount(acc)
	}

	return exportData
}
