package bs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/auth"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/business/equity"
	"github.com/iamarpitzala/acareca/internal/modules/business/invitation"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	"github.com/iamarpitzala/acareca/internal/modules/business/shared/events"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
	"github.com/xuri/excelize/v2"

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
	addEquityItem(960, "Retained Earnings", totalOwnerEquity.RetainedEarnings)

	netAssets := totalAssets - totalLiabilities
	totalEquity := totalOwnerEquity.TotalEquity + totalOtherEquity
	displayEnd := formatDateForDisplay(endDate)

	result := &RsBalanceSheet{
		EndDate:           displayEnd,
		Assets:            assets,
		TotalAssets:       totalAssets,
		Liabilities:       liabilities,
		TotalLiabilities:  totalLiabilities,
		NetAssets:         math.Round(netAssets*100) / 100,
		Equity:            equitySect,
		CurrentYearProfit: totalOwnerEquity.CurrentYearProfit,
		TotalEquity:       math.Round(totalEquity*100) / 100,
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
	f := excelize.NewFile()
	sheet := "Balance Sheet"
	f.NewSheet(sheet)
	f.DeleteSheet("Sheet1")

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

	styleHeaderBlue, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Family: "Calibri", Size: 14},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#DAEEF3"}, Pattern: 1},
	})
	styleSectionTitle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Family: "Calibri", Size: 12},
	})
	styleDataLeft, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Family: "Calibri", Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "left"},
		Border:    []excelize.Border{{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1}, {Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1}},
	})
	styleDataGrid, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Family: "Calibri", Size: 10},
		CustomNumFmt: func() *string { s := "$#,##0.00;$#,##0.00;$0.00"; return &s }(),
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Border:       []excelize.Border{{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1}, {Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1}},
	})
	styleGroupTotal, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Family: "Calibri"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"#DAEEF3"}, Pattern: 1},
		CustomNumFmt: func() *string { s := "$#,##0.00;$#,##0.00;$0.00"; return &s }(),
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Border:       []excelize.Border{{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1}, {Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1}},
	})
	styleProfit, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Family: "Calibri"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"#c4f0ce"}, Pattern: 1},
		CustomNumFmt: func() *string { s := "$#,##0.00;$#,##0.00;$0.00"; return &s }(),
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 2}, {Type: "top", Color: "000000", Style: 2},
			{Type: "bottom", Color: "000000", Style: 2}, {Type: "right", Color: "000000", Style: 2},
		},
	})
	styleProfitGreen, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Family: "Calibri", Color: "28a745"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"#c4f0ce"}, Pattern: 1},
		CustomNumFmt: func() *string { s := "$#,##0.00;$#,##0.00;$0.00"; return &s }(),
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 2}, {Type: "top", Color: "000000", Style: 2},
			{Type: "bottom", Color: "000000", Style: 2}, {Type: "right", Color: "000000", Style: 2},
		},
	})
	setRichMeta := func(cell string, label string, value string) {
		f.SetCellRichText(sheet, cell, []excelize.RichTextRun{
			{Text: label, Font: &excelize.Font{Bold: true, Family: "Calibri", Size: 10}},
			{Text: " " + value, Font: &excelize.Font{Bold: false, Family: "Calibri", Size: 10}},
		})
	}

	f.SetCellValue(sheet, "A1", "Balance Sheet")
	f.MergeCell(sheet, "A1", "B1")
	f.SetCellStyle(sheet, "A1", "B1", styleHeaderBlue)

	f.MergeCell(sheet, "A2", "B2")
	setRichMeta("A2", "Exported by:", entityName)

	f.MergeCell(sheet, "A3", "B3")
	if practitionerABN != "" {
		setRichMeta("A3", "ABN:", practitionerABN)
	}

	var dateText string
	if data.EndDate != "" {
		dateText = fmt.Sprintf("As of %s", data.EndDate)
	}

	f.MergeCell(sheet, "A4", "B4")
	setRichMeta("A4", "Period:", dateText)

	currentTimeStr := time.Now().Format("02/01/2006, 3:04:05 pm")
	f.MergeCell(sheet, "A5", "B5")
	setRichMeta("A5", "Generated:", currentTimeStr)
	f.MergeCell(sheet, "A6", "B6")

	currentRow := 7

	var assetTotalCell string
	var liabTotalCell string

	renderBSSection := func(title string, accounts []RsAccount) string {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), title)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleSectionTitle)

		if len(accounts) > 0 {
			tableRange := fmt.Sprintf("A%d:A%d", currentRow, currentRow+len(accounts))
			tableName := strings.ReplaceAll(title, " ", "_") + fmt.Sprintf("_%d", currentRow)

			showHeaders := true
			f.AddTable(sheet, &excelize.Table{
				Range:         tableRange,
				Name:          tableName,
				StyleName:     "",
				ShowHeaderRow: &showHeaders,
			})
		}

		currentRow++
		dataStartRow := currentRow
		for _, acc := range accounts {
			f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), acc.Name)
			f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleDataLeft)

			f.SetCellValue(sheet, fmt.Sprintf("B%d", currentRow), acc.Balance)
			f.SetCellStyle(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("B%d", currentRow), styleDataGrid)
			currentRow++
		}
		dataEndRow := currentRow - 1

		totalCell := fmt.Sprintf("B%d", currentRow)
		f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "TOTAL "+title)

		if len(accounts) > 0 {
			formula := fmt.Sprintf("SUBTOTAL(109, B%d:B%d)", dataStartRow, dataEndRow)
			f.SetCellFormula(sheet, totalCell, formula)
		} else {
			f.SetCellValue(sheet, totalCell, 0)
		}

		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("B%d", currentRow), styleGroupTotal)
		currentRow += 2
		return totalCell
	}

	assetTotalCell = renderBSSection("ASSETS", data.Assets)
	liabTotalCell = renderBSSection("LIABILITIES", data.Liabilities)

	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "NET ASSETS")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleProfit)

	netAssetsFormula := fmt.Sprintf("=%s-%s", assetTotalCell, liabTotalCell)
	f.SetCellFormula(sheet, fmt.Sprintf("B%d", currentRow), netAssetsFormula)
	f.SetCellStyle(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("B%d", currentRow), styleProfitGreen)
	currentRow += 3

	renderBSSection("EQUITY", data.Equity)

	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "Current Year Profit")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleProfit)
	f.SetCellValue(sheet, fmt.Sprintf("B%d", currentRow), data.CurrentYearProfit)
	f.SetCellStyle(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("B%d", currentRow), styleProfitGreen)
	currentRow += 2

	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "TOTAL EQUITY")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleProfit)
	f.SetCellValue(sheet, fmt.Sprintf("B%d", currentRow), data.TotalEquity)
	f.SetCellStyle(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("B%d", currentRow), styleProfitGreen)

	f.SetColWidth(sheet, "A", "A", 45)
	f.SetColWidth(sheet, "B", "B", 20)

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
