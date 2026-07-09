package bs

import (
	"context"
	"database/sql"
	"errors"
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
	"github.com/iamarpitzala/acareca/internal/modules/business/equity"
	"github.com/iamarpitzala/acareca/internal/modules/business/fy"
	"github.com/iamarpitzala/acareca/internal/modules/business/invitation"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	"github.com/iamarpitzala/acareca/internal/modules/notification"
	"github.com/iamarpitzala/acareca/internal/shared/export"
	bsexport "github.com/iamarpitzala/acareca/internal/shared/export/bs"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"

	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
)

type Service interface {
	GetBalanceSheet(ctx context.Context, f *BSFilter, actorID uuid.UUID, role string, userID uuid.UUID) (*RsBalanceSheet, error)
	ExportBalanceSheet(ctx context.Context, data []*RsBalanceSheet, exportType string, actorID uuid.UUID, role string, userID uuid.UUID, notifIDs []uuid.UUID, filterPractitionerID string) (*ExportBalanceSheetResponse, error)
}

type service struct {
	repo            Repository
	equitySvc       equity.Service
	db              *sqlx.DB
	auditSvc        audit.Service
	authRepo        auth.Repository
	invitationSvc   invitation.Service
	accountantRepo  accountant.Repository
	practitionerSvc practitioner.IService
	notificationPub *sharednotification.Publisher
	invitationRepo  invitation.Repository
	authSvc         auth.Service
	fySvc           fy.Service
}

func NewService(repo Repository, equitySvc equity.Service, db *sqlx.DB, auditSvc audit.Service, authRepo auth.Repository, invitationSvc invitation.Service, accountantRepo accountant.Repository, practitionerSvc practitioner.IService, invitationRepo invitation.Repository, authSvc auth.Service, notificationSvc notification.Service, adminRepo admin.Repository, fySvc fy.Service) Service {
	return &service{
		repo:            repo,
		equitySvc:       equitySvc,
		db:              db,
		auditSvc:        auditSvc,
		authRepo:        authRepo,
		invitationSvc:   invitationSvc,
		accountantRepo:  accountantRepo,
		practitionerSvc: practitionerSvc,
		notificationPub: sharednotification.NewPublisher(notification.NewServiceAdapter(notificationSvc), adminRepo),
		invitationRepo:  invitationRepo,
		authSvc:         authSvc,
		fySvc:           fySvc,
	}
}

type AccKey struct {
	Code int16
	Name string
}

func (s *service) GetBalanceSheet(ctx context.Context, f *BSFilter, actorID uuid.UUID, role string, userID uuid.UUID) (*RsBalanceSheet, error) {
	var targetPracIDs []uuid.UUID
	var practitionerID uuid.UUID

	switch role {
	case util.RolePractitioner:
		practitionerID = actorID
		targetPracIDs = []uuid.UUID{actorID}

	case util.RoleAccountant:
		if f.PractitionerID != nil && *f.PractitionerID != "" {
			pID, err := uuid.Parse(*f.PractitionerID)
			if err == nil {
				practitionerID = pID
				targetPracIDs = []uuid.UUID{pID}
			}
		}

	default:
		linked, err := s.invitationSvc.GetPractitionersLinkedToAccountant(ctx, actorID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch linked practitioners: %w", err)
		}
		if len(linked) == 0 {
			return nil, errors.New("no linked practitioners found")
		}
		targetPracIDs = linked

		if len(linked) > 0 {
			practitionerID = linked[0]
		}
	}

	endDate := ""
	if f.EndDate != nil && *f.EndDate != "" {
		endDate = *f.EndDate
	} else {
		fys, err := s.fySvc.GetFinancialYears(ctx)
		if err == nil && fys != nil {
			for _, item := range fys {
				if *item.IsActive {
					if parsedTime, parseErr := time.Parse(time.RFC3339, item.EndDate.String()); parseErr == nil {
						endDate = parsedTime.Format("2006-01-02")
						break
					} else {
						if len(item.EndDate.String()) >= 10 {
							endDate = item.EndDate.String()[:10]
							break
						}
					}
				}
			}
		}

		// Fallback to todays date if nothing matched
		if endDate == "" {
			endDate = time.Now().Format("2006-01-02")
		}
	}
	f.EndDate = &endDate

	rows, err := s.repo.GetBalanceSheet(ctx, targetPracIDs, f)
	if err != nil {
		return nil, err
	}

	var totalOwnerEquity equity.OwnerEquityCalculation
	for _, pID := range targetPracIDs {
		pracEquity, err := s.equitySvc.CalculateOwnerEquity(ctx, pID, nil, "", endDate)
		if err != nil {
			return nil, fmt.Errorf("calculate owner equity: %w", err)
		}
		totalOwnerEquity.ShareCapital += pracEquity.ShareCapital
		totalOwnerEquity.FundsIntroduced += pracEquity.FundsIntroduced
		totalOwnerEquity.Drawings += pracEquity.Drawings
		totalOwnerEquity.RetainedEarnings += pracEquity.RetainedEarnings
		totalOwnerEquity.CurrentYearProfit += pracEquity.CurrentYearProfit
		totalOwnerEquity.TotalEquity += pracEquity.TotalEquity
	}

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
			// 🚀 COA FIX: Filter using your new COA template system codes
			if row.AccountCode != 900 && row.AccountCode != 910 &&
				row.AccountCode != 920 && row.AccountCode != 960 &&
				row.AccountCode != 970 && row.AccountCode != 980 {
				acc := otherEquityMap[key]
				acc.Code, acc.Name, acc.CoaId = row.AccountCode, row.AccountName, row.CoaID
				acc.Balance += row.Balance
				otherEquityMap[key] = acc
				totalOtherEquity += row.Balance
			}
		}
	}

	assets := []RsAccount{}
	for _, v := range assetMap {
		assets = append(assets, v)
	}

	liabilities := []RsAccount{}
	for _, v := range liabMap {
		liabilities = append(liabilities, v)
	}

	equitySect := []RsAccount{}
	for _, v := range otherEquityMap {
		equitySect = append(equitySect, v)
	}

	addEquityItem := func(code int16, name string, balance float64) {
		if balance == 0 {
			return
		}
		var coaId *uuid.UUID
		err = util.RunInTransaction(ctx, s.db, func(Ctx context.Context, tx *sqlx.Tx) error {
			var err error
			coaId, err = s.getCoaIDByAccountCode(Ctx, tx, practitionerID, code)
			return err
		})
		if err != nil {
			return
		}

		if coaId != nil {
			equitySect = append(equitySect, RsAccount{
				CoaId:   *coaId,
				Code:    code,
				Name:    name,
				Balance: balance,
			})
		}
	}
	addEquityItem(970, "Owner A Share Capital", totalOwnerEquity.ShareCapital)
	addEquityItem(920, "Owner A Funds Introduced", totalOwnerEquity.FundsIntroduced)
	addEquityItem(900, "Owner A Drawings", -totalOwnerEquity.Drawings)
	addEquityItem(960, "Retained Earnings", totalOwnerEquity.RetainedEarnings)

	netAssets := totalAssets - totalLiabilities
	totalEquity := totalOwnerEquity.TotalEquity + totalOtherEquity
	totalLiabilitiesAndEquity := totalLiabilities + totalEquity
	displayEnd := formatDateForDisplay(endDate)

	result := &RsBalanceSheet{
		EndDate:                   displayEnd,
		Assets:                    assets,
		TotalAssets:               totalAssets,
		Liabilities:               liabilities,
		TotalLiabilities:          totalLiabilities,
		NetAssets:                 math.Round(netAssets*100) / 100,
		Equity:                    equitySect,
		CurrentYearProfit:         totalOwnerEquity.CurrentYearProfit,
		TotalEquity:               math.Round(totalEquity*100) / 100,
		TotalLiabilitiesAndEquity: math.Round(totalLiabilitiesAndEquity*100) / 100,
	}

	userIDStr := userID.String()
	actorIDStr := actorID.String()

	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		PracticeID: nil,
		UserID:     &userIDStr,
		Action:     auditctx.ActitionBalanceSheetGenerated,
		Module:     auditctx.ModuleReport,
		EntityType: lo.ToPtr(auditctx.EntityBalanceSheet),
		EntityID:   &actorIDStr,
		AfterState: map[string]interface{}{
			"report_type": "Balance Sheet",
			"end_date":    endDate,
		},
	})

	// Send notification for report generation
	if err := s.notifyReportExport(ctx, actorID, util.ActorType(role), util.EventBalanceSheetGenerated, "Balance Sheet", targetPracIDs); err != nil {
		log.Printf("[WARN] failed to send balance sheet generation notification: %v", err)
	}

	return result, nil
}

func (s *service) ExportBalanceSheet(ctx context.Context, data []*RsBalanceSheet, exportType string, actorID uuid.UUID, role string, userID uuid.UUID, notifIDs []uuid.UUID, filterPractitionerID string) (*ExportBalanceSheetResponse, error) {
	// If no data was passed, prevent panic
	if len(data) == 0 {
		return nil, errors.New("no balance sheet data provided for export")
	}

	baselineData := data[0]

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
		pracUUID, err := uuid.Parse(targetID)
		if err == nil {
			prac, err := s.practitionerSvc.GetPractitioner(ctx, pracUUID)
			if err == nil {
				if prac.EntityName != nil {
					entityName = *prac.EntityName
				} else {
					entityName = fullName
				}
				if prac.ABN != nil {
					practitionerABN = *prac.ABN
				}
			}
		}
	} else {
		if role == util.RolePractitioner {
			prac, err := s.practitionerSvc.GetPractitioner(ctx, uuid.MustParse(targetID))
			entityName = fullName
			if err == nil {
				if prac.ABN != nil {
					practitionerABN = *prac.ABN
				}
			}
		} else {
			acc, err := s.accountantRepo.GetAccountantByUserID(ctx, userID.String())
			if err == nil {
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
	}

	var dateText string
	if baselineData.EndDate != "" {
		dateText = fmt.Sprintf("As at %s", baselineData.EndDate)
	}

	config := export.ExportConfig{
		EntityName:     entityName,
		EntityABN:      practitionerABN,
		Period:         dateText,
		ExportType:     exportType,
		ExportedByName: fullName,
		GeneratedTime:  time.Now().Format("02/01/2006, 3:04:05 pm"),
	}

	exportDataList := make([]*bsexport.RsBalanceSheet, len(data))

	for yearIdx, yearData := range data {
		exportData := &bsexport.RsBalanceSheet{
			EndDate:                   yearData.EndDate,
			Assets:                    make([]bsexport.RsAccount, len(yearData.Assets)),
			TotalAssets:               yearData.TotalAssets,
			Liabilities:               make([]bsexport.RsAccount, len(yearData.Liabilities)),
			TotalLiabilities:          yearData.TotalLiabilities,
			Equity:                    make([]bsexport.RsAccount, len(yearData.Equity)),
			CurrentYearProfit:         yearData.CurrentYearProfit,
			TotalEquity:               yearData.TotalEquity,
			TotalLiabilitiesAndEquity: yearData.TotalLiabilities + yearData.TotalEquity,
		}

		for i, acc := range yearData.Assets {
			exportData.Assets[i] = bsexport.RsAccount{
				CoaId:   acc.CoaId,
				Code:    acc.Code,
				Name:    acc.Name,
				Balance: acc.Balance,
			}
		}
		for i, acc := range yearData.Liabilities {
			exportData.Liabilities[i] = bsexport.RsAccount{
				CoaId:   acc.CoaId,
				Code:    acc.Code,
				Name:    acc.Name,
				Balance: acc.Balance,
			}
		}
		for i, acc := range yearData.Equity {
			exportData.Equity[i] = bsexport.RsAccount{
				CoaId:   acc.CoaId,
				Code:    acc.Code,
				Name:    acc.Name,
				Balance: acc.Balance,
			}
		}

		exportDataList[yearIdx] = exportData
	}

	f, err := bsexport.GenerateExcelReport(exportDataList, config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate balance sheet excel: %w", err)
	}

	// Send notification
	if err := s.notifyReportExport(ctx, actorID, util.ActorType(role), util.EventBalanceSheetExport, "Balance Sheet", notifIDs); err != nil {
		log.Printf("[WARN] failed to send balance sheet notification: %v", err)
	}

	userIDStr := userID.String()
	parsedActorID := actorID.String()

	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		Action:     auditctx.ActionBalanceSheetExported,
		Module:     auditctx.ModuleReport,
		EntityType: lo.ToPtr(auditctx.EntityBalanceSheet),
		EntityID:   &parsedActorID,
		UserID:     &userIDStr,
		AfterState: map[string]interface{}{"report_type": "Balance Sheet", "export_type": exportType, "end_date": baselineData.EndDate},
	})

	f.UpdateLinkedValue()

	return &ExportBalanceSheetResponse{Result: f}, nil
}

func (s *service) getCoaIDByAccountCode(ctx context.Context, tx *sqlx.Tx, practitionerID uuid.UUID, accountCode int16) (*uuid.UUID, error) {
	query := `
		SELECT id
		FROM tbl_chart_of_accounts
		WHERE practitioner_id = $1
		  AND code = $2
		  AND deleted_at IS NULL
		LIMIT 1
	`
	args := []interface{}{practitionerID, accountCode}

	var coaID uuid.UUID
	err := tx.QueryRowContext(ctx, query, args...).Scan(&coaID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("COA account with code %d not found for practitioner %s", accountCode, practitionerID)
		}
		return nil, fmt.Errorf("get coa_id for account code %d: %w", accountCode, err)
	}
	return &coaID, nil
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
	var action string

	switch eventType {
	case util.EventPLReportGenerated:
		action = "Generated"
	case util.EventPLReportExport:
		action = "Exported"
	default:
		action = "Generated"
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
