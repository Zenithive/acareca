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

	endDate := time.Now().Format("2006-01-02")
	if f.EndDate != nil && *f.EndDate != "" {
		endDate = *f.EndDate
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
			if row.AccountCode != 880 && row.AccountCode != 881 &&
				row.AccountCode != 960 && row.AccountCode != 970 {
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
	addEquityItem(970, "Owner Share Capital", totalOwnerEquity.ShareCapital)
	addEquityItem(881, "Owner Funds Introduced", totalOwnerEquity.FundsIntroduced)
	addEquityItem(880, "Owner Drawings", -totalOwnerEquity.Drawings)
	// Net profit (current year) is rolled into COA 960 - Retained Earnings
	addEquityItem(960, "Retained Earnings", totalOwnerEquity.RetainedEarnings+totalOwnerEquity.CurrentYearProfit)

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

	meta := auditctx.GetMetadata(ctx)
	userIDStr := userID.String()
	actorIDStr := actorID.String()

	s.auditSvc.LogAsync(&audit.LogEntry{
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
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})

	if role == util.RoleAccountant {
		var fullName string
		user, err := s.authRepo.FindByID(ctx, userID)
		if err == nil {
			fullName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
			var dateDescription string
			if endDate != "" {
				dateDescription = fmt.Sprintf("as of %s", formatDateForDisplay(endDate))
			}

			description := fmt.Sprintf("Accountant %s generated Balance Sheet %s", fullName, dateDescription)
			for _, pID := range targetPracIDs {
				_ = s.eventsSvc.Record(ctx, events.SharedEvent{
					ID:             uuid.New(),
					PractitionerID: pID,
					AccountantID:   actorID,
					ActorID:        userID,
					ActorName:      &fullName,
					ActorType:      role,
					EventType:      "balance_sheet.generated",
					EntityType:     "REPORT",
					Description:    description,
					Metadata:       events.JSONBMap{"report_type": "Balance Sheet", "end_date": endDate},
					CreatedAt:      time.Now(),
				})
			}
		}
	}

	return result, nil
}

func (s *service) ExportBalanceSheet(ctx context.Context, data *RsBalanceSheet, exportType string, actorID uuid.UUID, role string, userID uuid.UUID, notifIDs []uuid.UUID, filterPractitionerID string) (*ExportBalanceSheetResponse, error) {
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
	if data.EndDate != "" {
		dateText = fmt.Sprintf("As of %s", data.EndDate)
	}

	config := export.ExportConfig{
		EntityName:     entityName,
		EntityABN:      practitionerABN,
		Period:         dateText,
		ExportType:     exportType,
		ExportedByName: fullName,
		GeneratedTime:  time.Now().Format("02/01/2006, 3:04:05 pm"),
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
		exportData.Assets[i] = bsexport.RsAccount{
			CoaId:   acc.CoaId,
			Code:    acc.Code,
			Name:    acc.Name,
			Balance: acc.Balance,
		}
	}
	for i, acc := range data.Liabilities {
		exportData.Liabilities[i] = bsexport.RsAccount{
			CoaId:   acc.CoaId,
			Code:    acc.Code,
			Name:    acc.Name,
			Balance: acc.Balance,
		}
	}
	for i, acc := range data.Equity {
		exportData.Equity[i] = bsexport.RsAccount{
			CoaId:   acc.CoaId,
			Code:    acc.Code,
			Name:    acc.Name,
			Balance: acc.Balance,
		}
	}

	f, err := bsexport.GenerateExcelReport(exportData, config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate balance sheet excel: %w", err)
	}

	meta := auditctx.GetMetadata(ctx)
	userIDStr := userID.String()
	parsedActorID := actorID.String()

	s.auditSvc.LogAsync(&audit.LogEntry{
		Action:     auditctx.ActionBalanceSheetExported,
		Module:     auditctx.ModuleReport,
		EntityType: lo.ToPtr(auditctx.EntityBalanceSheet),
		EntityID:   &parsedActorID,
		UserID:     &userIDStr,
		AfterState: map[string]interface{}{"report_type": "Balance Sheet", "export_type": exportType, "end_date": data.EndDate},
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	if role == util.RoleAccountant && len(notifIDs) > 0 {
		var dateDescription string
		if data.EndDate != "" {
			dateDescription = fmt.Sprintf("as of %s", formatDateForDisplay(data.EndDate))
		}

		description := fmt.Sprintf("Accountant %s exported Balance Sheet (%s) %s", fullName, exportType, dateDescription)
		for _, pID := range notifIDs {
			_ = s.eventsSvc.Record(ctx, events.SharedEvent{
				ID:             uuid.New(),
				PractitionerID: pID,
				AccountantID:   actorID,
				ActorID:        userID,
				ActorName:      &fullName,
				ActorType:      role,
				EventType:      "balance_sheet.exported",
				EntityType:     "REPORT",
				Description:    description,
				Metadata:       events.JSONBMap{"report_type": "Balance Sheet", "export_type": exportType, "end_date": data.EndDate},
				CreatedAt:      time.Now(),
			})
		}
	}
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
