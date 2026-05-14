package bs

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/auth"
	"github.com/iamarpitzala/acareca/internal/modules/business/equity"
	"github.com/iamarpitzala/acareca/internal/modules/business/invitation"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	"github.com/iamarpitzala/acareca/internal/modules/business/shared/events"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
	"github.com/xuri/excelize/v2"

	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
)

type Service interface {
	GetBalanceSheet(ctx context.Context, f *BSFilter, actorID uuid.UUID, role string, userID uuid.UUID) (*RsBalanceSheet, error)
	ExportBalanceSheet(ctx context.Context, data *RsBalanceSheet, exportType string, actorID uuid.UUID, role string, userID uuid.UUID, notifIDs []uuid.UUID, filterPractitionerID string) (interface{}, error)
}

type service struct {
	repo            Repository
	equitySvc       equity.Service
	db              sqlx.DB
	auditSvc        audit.Service
	eventsSvc       events.Service
	authRepo        auth.Repository
	invitationSvc   invitation.Service
	practitionerSvc practitioner.IService
}

func NewService(repo Repository, equitySvc equity.Service, db sqlx.DB, auditSvc audit.Service, eventsSvc events.Service, authRepo auth.Repository, invitationSvc invitation.Service, practitionerSvc practitioner.IService) Service {
	return &service{
		repo:            repo,
		equitySvc:       equitySvc,
		db:              db,
		auditSvc:        auditSvc,
		eventsSvc:       eventsSvc,
		authRepo:        authRepo,
		invitationSvc:   invitationSvc,
		practitionerSvc: practitionerSvc,
	}
}

type AccKey struct {
	Code int16
	Name string
}

func (s *service) GetBalanceSheet(ctx context.Context, f *BSFilter, actorID uuid.UUID, role string, userID uuid.UUID) (*RsBalanceSheet, error) {
	// Role-based Practitioner Resolution
	var targetPracIDs []uuid.UUID
	var practitionerID uuid.UUID

	if role == util.RolePractitioner {
		practitionerID = actorID
		targetPracIDs = []uuid.UUID{actorID}
	} else if role == util.RoleAccountant {
		if f.PractitionerID != nil && *f.PractitionerID != "" {
			pID, err := uuid.Parse(*f.PractitionerID)
			if err == nil {
				practitionerID = pID
				targetPracIDs = []uuid.UUID{pID}
			}
		} else {
			// Case: Accountant hasn't selected a specific practitioner
			linked, err := s.invitationSvc.GetPractitionersLinkedToAccountant(ctx, actorID)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch linked practitioners: %w", err)
			}
			if len(linked) == 0 {
				return nil, errors.New("no linked practitioners found")
			}
			// Default to the first linked practitioner for the actual data fetch
			targetPracIDs = linked

			if len(linked) > 0 {
				practitionerID = linked[0]
			}
		}
	}

	// Determine reporting range
	startDate := ""
	if f.StartDate != nil {
		startDate = *f.StartDate
	}

	endDate := time.Now().Format("2006-01-02")
	if f.EndDate != nil && *f.EndDate != "" {
		endDate = *f.EndDate
	}

	f.StartDate = &startDate
	f.EndDate = &endDate

	// Get balance sheet accounts (assets, liabilities, other equity accounts)
	rows, err := s.repo.GetBalanceSheet(ctx, targetPracIDs, f)
	if err != nil {
		return nil, err
	}

	// Get automatically calculated owner equity
	var totalOwnerEquity equity.OwnerEquityCalculation
	for _, pID := range targetPracIDs {
		pracEquity, err := s.equitySvc.CalculateOwnerEquity(ctx, pID, nil, startDate, endDate)
		if err != nil {
			return nil, fmt.Errorf("calculate owner equity: %w", err)
		}
		// Aggregate totals
		totalOwnerEquity.ShareCapital += pracEquity.ShareCapital
		totalOwnerEquity.FundsIntroduced += pracEquity.FundsIntroduced
		totalOwnerEquity.Drawings += pracEquity.Drawings
		totalOwnerEquity.RetainedEarnings += pracEquity.RetainedEarnings
		totalOwnerEquity.CurrentYearProfit += pracEquity.CurrentYearProfit
		totalOwnerEquity.TotalEquity += pracEquity.TotalEquity
	}

	// 4. Group and Summarize Assets/Liabilities/Other Equity
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
			// Skip owner fund accounts - they're handled separately
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

	// 5. Convert Maps back to Slices for the Response
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

	// 6. Append the Calculated Equity Items
	addEquityItem := func(code int16, name string, balance float64) {
		if balance == 0 {
			return
		}
		coaId, _ := s.getCoaIDByAccountCode(ctx, practitionerID, code)
		equitySect = append(equitySect, RsAccount{
			CoaId:   *coaId,
			Code:    code,
			Name:    name,
			Balance: balance,
		})
	}
	addEquityItem(970, "Owner Share Capital", totalOwnerEquity.ShareCapital)
	addEquityItem(881, "Owner Funds Introduced", totalOwnerEquity.FundsIntroduced)
	addEquityItem(880, "Owner Drawings", -totalOwnerEquity.Drawings) // Negative for drawings
	addEquityItem(960, "Retained Earnings", totalOwnerEquity.RetainedEarnings)

	// Total equity = calculated owner equity + other equity accounts
	totalEquity := totalOwnerEquity.TotalEquity + totalOtherEquity

	// Format dates for the response
	displayStart := formatDateForDisplay(startDate)
	displayEnd := formatDateForDisplay(endDate)

	result := &RsBalanceSheet{
		StartDate:                 displayStart,
		EndDate:                   displayEnd,
		Assets:                    assets,
		TotalAssets:               totalAssets,
		Liabilities:               liabilities,
		TotalLiabilities:          totalLiabilities,
		Equity:                    equitySect,
		CurrentYearProfit:         totalOwnerEquity.CurrentYearProfit,
		TotalEquity:               totalEquity,
		TotalLiabilitiesAndEquity: totalLiabilities + totalEquity,
	}

	// --- AUDIT & SHARED EVENTS LOGIC ---
	meta := auditctx.GetMetadata(ctx)

	userIDStr := userID.String()
	actorIDStr := actorID.String()

	// Trigger Audit Log
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: nil,
		UserID:     &userIDStr,
		Action:     auditctx.ActitionBalanceSheetGenerated,
		Module:     auditctx.ModuleReport,
		EntityType: strPtr(auditctx.EntityBalanceSheet),
		EntityID:   &actorIDStr,
		AfterState: map[string]interface{}{
			"report_type": "Balance Sheet",
			"start_date":  startDate,
			"end_date":    endDate,
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})

	// Trigger Shared Event
	if role == util.RoleAccountant {
		// Fetching user details
		var fullName string
		user, err := s.authRepo.FindByID(ctx, userID)
		if err == nil {
			fullName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
			var dateDescription string
			if startDate != "" && endDate != "" {
				dateDescription = fmt.Sprintf("for the period of %s to %s", formatDateForDisplay(startDate), formatDateForDisplay(endDate))
			} else if startDate != "" {
				dateDescription = fmt.Sprintf("for the period of %s to %s", formatDateForDisplay(startDate), formatDateForDisplay(endDate))
			} else if endDate != "" {
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
					Metadata:       events.JSONBMap{"report_type": "Balance Sheet", "start_date": startDate, "end_date": endDate},
					CreatedAt:      time.Now(),
				})
			}
		}
	}

	return result, nil
}

func (s *service) ExportBalanceSheet(ctx context.Context, data *RsBalanceSheet, exportType string, actorID uuid.UUID, role string, userID uuid.UUID, notifIDs []uuid.UUID, filterPractitionerID string) (interface{}, error) {
	f := excelize.NewFile()
	sheet := "Balance Sheet"
	f.NewSheet(sheet)
	f.DeleteSheet("Sheet1")

	// --- FETCH METADATA ---
	var fullName string
	user, err := s.authRepo.FindByID(ctx, userID)
	if err == nil {
		fullName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
	}

	var practitionerABN string
	targetID := filterPractitionerID
	if targetID == "" && role == util.RolePractitioner {
		targetID = actorID.String()
	}

	if targetID != "" {
		pracUUID, err := uuid.Parse(targetID)
		if err == nil {
			prac, err := s.practitionerSvc.GetPractitioner(ctx, pracUUID)
			if err == nil && prac.ABN != nil {
				practitionerABN = *prac.ABN
			}
		}
	}

	// --- STYLES ---
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

	// --- RENDER HEADERS ---
	f.SetCellValue(sheet, "A1", "Balance Sheet")
	f.MergeCell(sheet, "A1", "B1")
	f.SetCellStyle(sheet, "A1", "B1", styleHeaderBlue)

	f.MergeCell(sheet, "A2", "B2")
	setRichMeta("A2", "Exported by:", fullName)

	f.MergeCell(sheet, "A3", "B3")
	if practitionerABN != "" {
		setRichMeta("A3", "ABN:", practitionerABN)
	}

	var dateText string
	if data.StartDate != "" && data.EndDate != "" {
		dateText = fmt.Sprintf("%s to %s", data.StartDate, data.EndDate)
	} else if data.EndDate != "" {
		dateText = fmt.Sprintf("As of %s", data.EndDate)
	}
	f.MergeCell(sheet, "A4", "B4")
	setRichMeta("A4", "Period:", dateText)

	// --- BLANK ROW AFTER METADATA ---
	f.MergeCell(sheet, "A5", "B5")

	currentRow := 6

	// Helper to render sections with Excel Filters
	renderBSSection := func(title string, accounts []RsAccount, total float64) string {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), title)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleSectionTitle)

		// --- APPLY TABLE FILTER ---
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

	// --- RENDER SECTIONS ---
	renderBSSection("ASSETS", data.Assets, data.TotalAssets)
	renderBSSection("LIABILITIES", data.Liabilities, data.TotalLiabilities)
	renderBSSection("EQUITY", data.Equity, data.TotalEquity)

	// --- CURRENT YEAR PROFIT ---
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "Current Year Profit")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleProfit)
	f.SetCellValue(sheet, fmt.Sprintf("B%d", currentRow), data.CurrentYearProfit)
	f.SetCellStyle(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("B%d", currentRow), styleProfitGreen)
	currentRow += 2

	// --- FINAL TOTALS ---
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "TOTAL LIABILITIES & EQUITY")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleProfit)
	f.SetCellValue(sheet, fmt.Sprintf("B%d", currentRow), data.TotalLiabilitiesAndEquity)
	f.SetCellStyle(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("B%d", currentRow), styleProfitGreen)

	f.SetColWidth(sheet, "A", "A", 45)
	f.SetColWidth(sheet, "B", "B", 20)

	// --- AUDIT & SHARED EVENTS  ---
	meta := auditctx.GetMetadata(ctx)
	userIDStr := userID.String()
	parsedActorID := actorID.String()

	s.auditSvc.LogAsync(&audit.LogEntry{
		Action:     auditctx.ActionBalanceSheetExported,
		Module:     auditctx.ModuleReport,
		EntityType: strPtr(auditctx.EntityBalanceSheet),
		EntityID:   &parsedActorID,
		UserID:     &userIDStr,
		AfterState: map[string]interface{}{"report_type": "Balance Sheet", "export_type": exportType, "start_date": data.StartDate, "end_date": data.EndDate},
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	if role == util.RoleAccountant && len(notifIDs) > 0 {
		var dateDescription string
		if data.StartDate != "" && data.EndDate != "" {
			dateDescription = fmt.Sprintf("for the period of %s to %s", formatDateForDisplay(data.StartDate), formatDateForDisplay(data.EndDate))
		} else if data.StartDate != "" {
			dateDescription = fmt.Sprintf("for the period of %s to %s", formatDateForDisplay(data.StartDate), formatDateForDisplay(data.EndDate))
		} else if data.EndDate != "" {
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
				Metadata:       events.JSONBMap{"report_type": "Balance Sheet", "export_type": exportType, "start_date": data.StartDate, "end_date": data.EndDate},
				CreatedAt:      time.Now(),
			})
		}
	}
	f.UpdateLinkedValue()

	if exportType == "pdf" {
		return s.generateBSHTMLString(f, sheet, data, fullName, practitionerABN)
	}
	return f, nil
}

func (s *service) generateBSHTMLString(f *excelize.File, sheetName string, data *RsBalanceSheet, fullName string, practitionerABN string) (string, error) {
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return "", err
	}

	var b bytes.Buffer
	b.WriteString("<html><head><style>")
	b.WriteString(`
		@page { size: A4; margin: 1cm; }
		body { font-family: 'Calibri', sans-serif; padding: 20px; color: #333; }
		table { width: 100%; border-collapse: collapse; }
		td { padding: 6px 8px; font-size: 10pt; border: 0.5pt solid #000; }
		.header-blue { background-color: #DAEEF3; font-weight: bold; font-size: 14pt; text-align: center; }
		// .section-title { font-weight: bold; font-size: 12pt; background-color: #f9f9f9; border: none; padding-top: 15px; }
		.section-title { font-weight: bold; font-size: 12pt; padding-top: 15px; border: none; }
		.group-total { background-color: #DAEEF3; font-weight: bold; text-align: right; }
		.final-total-label { background-color: #c4f0ce !important; font-weight: bold; border: 1.5pt solid #000; }
		.final-total-value { background-color: #c4f0ce !important; font-weight: bold; color: #28a745; text-align: right; border: 1.5pt solid #000; }
		.data-grid { text-align: right; }
		.spacer { height: 10px; border: none; }
		.meta-item { font-size: 10pt; margin-bottom: 6px; text-align: left; }
		.meta-label { font-weight: bold; }
	`)
	b.WriteString("</style></head><body>")

	// Print button that only shows on screen, not on the PDF/Printout
	b.WriteString(`<div class="no-print" style="width:100%;text-align:right;margin-bottom:15px;">
	<button onclick="window.print()" style="padding:10px 20px;background:#DAEEF3;color:#000;border:1.2pt solid #000;border-radius:4px;cursor:pointer;font-weight:bold;font-family:sans-serif;">Print to PDF</button>
	<style>@media print{.no-print{display:none}}</style></div>`)

	b.WriteString(fmt.Sprintf("<div class='meta-item'><span class='meta-label'>Exported by:</span> %s</div>", fullName))
	if practitionerABN != "" {
		b.WriteString(fmt.Sprintf("<div class='meta-item'><span class='meta-label'>ABN:</span> %s</div>", practitionerABN))
	}

	var dateText string
	if data.StartDate != "" && data.EndDate != "" {
		dateText = fmt.Sprintf("%s to %s", data.StartDate, data.EndDate)
	} else if data.EndDate != "" {
		dateText = fmt.Sprintf("As of %s", data.EndDate)
	} else if data.StartDate != "" {
		dateText = fmt.Sprintf("From %s onwards", data.StartDate)
	}

	if dateText != "" {
		b.WriteString(fmt.Sprintf("<div class='meta-item'><span class='meta-label'>Period:</span> %s</div>", dateText))
	}

	b.WriteString("<div style='margin-bottom: 20px;'></div>") // Spacer after metadata

	b.WriteString("<table><colgroup><col style='width: 70%;'><col style='width: 30%;'></colgroup>")

	for rIdx, row := range rows {
		if rIdx >= 1 && rIdx <= 4 {
			continue
		}

		if len(row) == 0 {
			b.WriteString("<tr><td colspan='2' class='spacer'></td></tr>")
			continue
		}

		valA := row[0]
		valB := ""
		if len(row) > 1 {
			valB = row[1]
		}

		formatCurr := func(v float64) string {
			return fmt.Sprintf("$%.2f", v)
		}

		classA, classB := "data-left", "data-grid"

		switch {
		case rIdx == 0:
			b.WriteString(fmt.Sprintf("<tr><td colspan='2' class='header-blue'>%s</td></tr>", valA))
			continue

		case valA == "ASSETS" || valA == "LIABILITIES" || valA == "EQUITY":
			classA = "section-title"
			b.WriteString(fmt.Sprintf("<tr><td colspan='2' class='%s'>%s</td></tr>", classA, valA))
			continue

		case valA == "TOTAL ASSETS":
			classA, classB = "group-total", "group-total"
			valB = formatCurr(data.TotalAssets)

		case valA == "TOTAL LIABILITIES":
			classA, classB = "group-total", "group-total"
			valB = formatCurr(data.TotalLiabilities)

		case valA == "TOTAL EQUITY":
			classA, classB = "group-total", "group-total"
			valB = formatCurr(data.TotalEquity)

		case valA == "Current Year Profit":
			classA, classB = "final-total-label", "final-total-value"
			valB = formatCurr(data.CurrentYearProfit)

		case valA == "TOTAL LIABILITIES & EQUITY":
			classA, classB = "final-total-label", "final-total-value"
			valB = formatCurr(data.TotalLiabilitiesAndEquity)
		}

		b.WriteString(fmt.Sprintf("<tr><td class='%s'>%s</td><td class='%s'>%s</td></tr>", classA, valA, classB, valB))
	}

	b.WriteString("</table></body></html>")
	return b.String(), nil
}

// getCoaIDByAccountCode retrieves the coa_id for a given account code
func (s *service) getCoaIDByAccountCode(ctx context.Context, practitionerID uuid.UUID, accountCode int16) (*uuid.UUID, error) {
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
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&coaID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("COA account with code %d not found for practitioner %s", accountCode, practitionerID)
		}
		return nil, fmt.Errorf("get coa_id for account code %d: %w", accountCode, err)
	}
	return &coaID, nil
}

// Helper function for audit logging
func strPtr(s string) *string { return &s }

func formatDateForDisplay(dateStr string) string {
	if dateStr == "" {
		return ""
	}
	// Parse the database format
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return dateStr // Return original if parsing fails to avoid losing data
	}
	// Return the display format
	return t.Format("02-01-2006")
}
