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
	"github.com/iamarpitzala/acareca/internal/modules/business/shared/events"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
	"github.com/xuri/excelize/v2"

	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
)

type Service interface {
	GetBalanceSheet(ctx context.Context, f *BSFilter, actorID uuid.UUID, role string, userID uuid.UUID) (*RsBalanceSheet, error)
	ExportBalanceSheet(ctx context.Context, data *RsBalanceSheet, exportType string, actorID uuid.UUID, role string, userID uuid.UUID, notifIDs []uuid.UUID, filterClinicID string) (interface{}, error)
}

type service struct {
	repo          Repository
	equitySvc     equity.Service
	db            sqlx.DB
	auditSvc      audit.Service
	eventsSvc     events.Service
	authRepo      auth.Repository
	invitationSvc invitation.Service
}

func NewService(repo Repository, equitySvc equity.Service, db sqlx.DB, auditSvc audit.Service, eventsSvc events.Service, authRepo auth.Repository, invitationSvc invitation.Service) Service {
	return &service{
		repo:          repo,
		equitySvc:     equitySvc,
		db:            db,
		auditSvc:      auditSvc,
		eventsSvc:     eventsSvc,
		authRepo:      authRepo,
		invitationSvc: invitationSvc,
	}
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
			practitionerID = linked[0]
			targetPracIDs = linked
		}
	}

	// Default to today if no date specified
	asOfDate := time.Now().Format("2006-01-02")
	if f.AsOfDate != nil && *f.AsOfDate != "" {
		asOfDate = *f.AsOfDate
	}

	// Parse clinic ID if provided
	var clinicID *uuid.UUID
	if f.ClinicID != nil && *f.ClinicID != "" {
		id, err := uuid.Parse(*f.ClinicID)
		if err != nil {
			return nil, fmt.Errorf("invalid clinic_id: %w", err)
		}
		clinicID = &id
	}

	// Get balance sheet accounts (assets, liabilities, other equity accounts)
	rows, err := s.repo.GetBalanceSheet(ctx, practitionerID, f)
	if err != nil {
		return nil, err
	}

	// Get automatically calculated owner equity
	ownerEquity, err := s.equitySvc.CalculateOwnerEquity(ctx, practitionerID, clinicID, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("calculate owner equity: %w", err)
	}

	// Organize by account type
	var assets, liabilities, equity []RsAccount
	var totalAssets, totalLiabilities, totalOtherEquity float64

	for _, row := range rows {
		account := row.ToRs()

		switch row.AccountType {
		case "Asset":
			assets = append(assets, account)
			totalAssets += row.Balance
		case "Liability":
			liabilities = append(liabilities, account)
			totalLiabilities += row.Balance
		case "Equity":
			// Skip owner fund accounts (880, 881, 960, 970) - they're calculated by equity service
			if row.AccountCode != 880 && row.AccountCode != 881 &&
				row.AccountCode != 960 && row.AccountCode != 970 {
				equity = append(equity, account)
				totalOtherEquity += row.Balance
			}
		}
	}

	// Build equity section from calculated values
	if ownerEquity.ShareCapital != 0 {
		coaId, err := s.getCoaIDByAccountCode(ctx, practitionerID, 970)
		if err != nil {
			return nil, err
		}

		equity = append(equity, RsAccount{
			CoaId:   *coaId,
			Code:    970,
			Name:    "Owner A Share Capital",
			Balance: ownerEquity.ShareCapital,
		})
	}

	if ownerEquity.FundsIntroduced != 0 {
		coaId, err := s.getCoaIDByAccountCode(ctx, practitionerID, 881)
		if err != nil {
			return nil, err
		}
		equity = append(equity, RsAccount{
			CoaId:   *coaId,
			Code:    881,
			Name:    "Owner A Funds Introduced",
			Balance: ownerEquity.FundsIntroduced,
		})
	}

	if ownerEquity.Drawings != 0 {
		coaId, err := s.getCoaIDByAccountCode(ctx, practitionerID, 880)
		if err != nil {
			return nil, err
		}
		equity = append(equity, RsAccount{
			CoaId:   *coaId,
			Code:    880,
			Name:    "Owner A Drawings",
			Balance: -ownerEquity.Drawings,
		})
	}

	if ownerEquity.RetainedEarnings != 0 {
		coaId, err := s.getCoaIDByAccountCode(ctx, practitionerID, 960)
		if err != nil {
			return nil, err
		}
		equity = append(equity, RsAccount{
			CoaId:   *coaId,
			Code:    960,
			Name:    "Retained Earnings",
			Balance: ownerEquity.RetainedEarnings,
		})
	}

	// Total equity = calculated owner equity + other equity accounts
	totalEquity := ownerEquity.TotalEquity + totalOtherEquity

	result := &RsBalanceSheet{
		AsOfDate:                  asOfDate,
		Assets:                    assets,
		TotalAssets:               totalAssets,
		Liabilities:               liabilities,
		TotalLiabilities:          totalLiabilities,
		Equity:                    equity,
		CurrentYearProfit:         ownerEquity.CurrentYearProfit,
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
			"as_of_date":  asOfDate,
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
					Description:    fmt.Sprintf("Accountant %s generated Balance Sheet as of %s", fullName, asOfDate),
					Metadata:       events.JSONBMap{"report_type": "Balance Sheet", "as_of_date": asOfDate},
					CreatedAt:      time.Now(),
				})
			}
		}
	}

	return result, nil
}

func (s *service) ExportBalanceSheet(ctx context.Context, data *RsBalanceSheet, exportType string, actorID uuid.UUID, role string, userID uuid.UUID, notifIDs []uuid.UUID, filterClinicID string) (interface{}, error) {
	f := excelize.NewFile()
	sheet := "Balance Sheet"
	f.NewSheet(sheet)
	f.DeleteSheet("Sheet1")

	// --- STYLES ---
	styleHeaderBlue, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Family: "Calibri", Size: 14},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#DAEEF3"}, Pattern: 1},
	})
	styleItalic, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Italic: true,
			Family: "Calibri",
			Size:   10,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "left",
		},
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

	// --- RENDER HEADERS ---
	f.SetCellValue(sheet, "A1", "Balance Sheet")
	f.MergeCell(sheet, "A1", "B1")
	f.SetCellStyle(sheet, "A1", "B1", styleHeaderBlue)
	f.SetCellValue(sheet, "A2", fmt.Sprintf("As of Date: %s", data.AsOfDate))
	f.SetCellStyle(sheet, "A2", "A2", styleItalic)

	currentRow := 4

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
		AfterState: map[string]interface{}{"report_type": "Balance Sheet", "export_type": exportType, "as_of_date": data.AsOfDate},
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	if role == util.RoleAccountant && len(notifIDs) > 0 {
		user, _ := s.authRepo.FindByID(ctx, userID)
		fullName := fmt.Sprintf("%s %s", user.FirstName, user.LastName)
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
				Description:    fmt.Sprintf("Accountant %s exported Balance Sheet (%s)", fullName, exportType),
				Metadata:       events.JSONBMap{"report_type": "Balance Sheet", "export_type": exportType, "as_of_date": data.AsOfDate},
				CreatedAt:      time.Now(),
			})
		}
	}
	f.UpdateLinkedValue()

	if exportType == "pdf" {
		return s.generateBSHTMLString(f, sheet, data)
	}
	return f, nil
}

func (s *service) generateBSHTMLString(f *excelize.File, sheetName string, data *RsBalanceSheet) (string, error) {
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
	`)
	b.WriteString("</style></head><body>")

	// Print button that only shows on screen, not on the PDF/Printout
	b.WriteString(`<div class="no-print" style="width:100%;text-align:right;margin-bottom:15px;">
	<button onclick="window.print()" style="padding:10px 20px;background:#DAEEF3;color:#000;border:1.2pt solid #000;border-radius:4px;cursor:pointer;font-weight:bold;font-family:sans-serif;">Print to PDF</button>
	<style>@media print{.no-print{display:none}}</style></div>`)

	b.WriteString("<table><colgroup><col style='width: 70%;'><col style='width: 30%;'></colgroup>")

	for rIdx, row := range rows {
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

		case strings.HasPrefix(valA, "As of Date"):
			b.WriteString(fmt.Sprintf("<tr><td colspan='2' style='border:none;font-style:italic;'>%s</td></tr>", valA))
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
