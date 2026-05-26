package pl

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/auth"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/business/clinic"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	"github.com/iamarpitzala/acareca/internal/modules/business/shared/events"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
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
	ExportPLReport(ctx context.Context, data *RsReport, exportType string, actorID uuid.UUID, role string, userID uuid.UUID, notifIDs []uuid.UUID, filterPractitionerID string) (interface{}, error)
}

type service struct {
	repo            Repository
	clinicRepo      clinic.Repository
	accountantRepo  accountant.Repository
	practitionerSvc practitioner.IService
	authRepo        auth.Repository
	auditSvc        audit.Service
	eventsSvc       events.Service
}

func NewService(repo Repository, clinicRepo clinic.Repository, accountantRepo accountant.Repository, practitionerSvc practitioner.IService, authRepo auth.Repository, auditSvc audit.Service, eventsSvc events.Service) Service {
	return &service{repo: repo, clinicRepo: clinicRepo, accountantRepo: accountantRepo, practitionerSvc: practitionerSvc, authRepo: authRepo, auditSvc: auditSvc, eventsSvc: eventsSvc}
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

func (s *service) GetReport(ctx context.Context, actorID uuid.UUID, f *PLReportFilter, role string, targetNotifIDs []uuid.UUID, userID uuid.UUID) (*RsReport, error) {
	meta := auditctx.GetMetadata(ctx)
	isAccountant := strings.EqualFold(role, util.RoleAccountant)

	var targetPracIDs []uuid.UUID
	var rows []*PLReportRow
	var summary *PLSummaryRow

	err := util.RunInTransaction(ctx, s.repo.(*repository).db, func(ctx context.Context, tx *sqlx.Tx) error {
		var innerErr error

		if isAccountant {
			// Determine Scope: Specific Clinic, Specific Practitioner, or All Linked
			if f.ClinicID != nil && *f.ClinicID != "" {
				// Scenario: Specific Clinic
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
				// Scenario: Specific Practitioner (Accountant picked one from a dropdown)
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
				// Scenario: Aggregation (Accountant viewing ALL linked practitioners)
				if len(targetNotifIDs) == 0 {
					return fmt.Errorf("no linked practitioners found for aggregation")
				}
				targetPracIDs = targetNotifIDs
			}
		} else {
			// Scenario: User is the Practitioner
			// If practitioner selects a clinic, verify ownership
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

		// Sync Filter state for Repo/Audit (use the first ID as a representative if aggregating)
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

		// Record the Shared Event within the safe transaction timeline
		if isAccountant && len(targetNotifIDs) > 0 {
			var fullName string
			user, innerErr := s.authRepo.FindByID(ctx, userID) // Fallback using transactional version if authRepo supports it
			if innerErr == nil {
				fullName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
			}

			fmt.Printf("\nName: %s\n", fullName)

			for _, pID := range targetNotifIDs {
				innerErr = s.eventsSvc.Record(ctx, events.SharedEvent{
					ID:             uuid.New(),
					PractitionerID: pID,
					AccountantID:   actorID,
					ActorID:        userID,
					ActorName:      &fullName,
					ActorType:      role,
					EventType:      "pl_report.generated",
					EntityType:     "REPORT",
					Description:    fmt.Sprintf("Accountant %s generated Profit and Loss Report", fullName),
					CreatedAt:      time.Now(),
					Metadata:       events.JSONBMap{"report_type": "Profit and Loss Report"},
				})
				if innerErr != nil {
					return fmt.Errorf("failed to write shared audit transaction record: %w", innerErr)
				}
			}
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

	// --- AUDIT LOG
	var userIDStr string
	userIDStr = userID.String()
	parsedActorID := actorID.String()

	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: nil,
		UserID:     &userIDStr,
		Action:     auditctx.ActionPLReportGenerated,
		Module:     auditctx.ModuleReport,
		EntityType: lo.ToPtr(auditctx.EntityPLReport),
		EntityID:   &parsedActorID,
		AfterState: map[string]interface{}{
			"report_type": "Profit and Loss Report",
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})

	return buildReport(f, rows, summary), nil
}

// buildReport assembles a flat P&L report aggregated across all clinics/forms,
// grouped by COA account within each P&L section.
func buildReport(f *PLReportFilter, rows []*PLReportRow, summary *PLSummaryRow) *RsReport {
	// coaKey → accumulated total per P&L section
	type coaKey struct {
		plSection string
		coaID     string
	}
	coaOrder := map[string][]string{} // plSection → ordered coaIDs
	coaSeen := map[coaKey]bool{}
	coaNames := map[coaKey]string{}
	coaTotals := map[coaKey]float64{}

	for _, r := range rows {
		// Use pl_section for proper categorization based on account type
		plSection := r.PLSection
		if plSection == "" {
			// Fallback to Other Expenses if somehow empty
			plSection = "3. Other Expenses"
		}

		// Use net_amount consistently across all sections for P&L reporting.
		// P&L should show revenue and expenses on a GST-exclusive basis:
		// - Income: NET (actual revenue earned, GST is collected for government)
		// - Costs: NET (actual expenses, GST can be claimed back)
		// This aligns with standard accounting practice where GST is a pass-through.
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

	// grossProfit := round2(income.GroupTotal - cos.GroupTotal)
	// netProfit := round2(grossProfit - other.GroupTotal)

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

func (s *service) ExportPLReport(ctx context.Context, data *RsReport, exportType string, actorID uuid.UUID, role string, userID uuid.UUID, notifIDs []uuid.UUID, filterPractitionerID string) (interface{}, error) {
	// --- FETCH METADATA ---
	var fullName string
	user, err := s.authRepo.FindByID(ctx, userID)
	if err == nil {
		fullName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
	}

	var entityName string
	var practitionerABN string
	// If a practitioner is filtered, resolve that specific profile for the ABN.
	// Otherwise, fallback to the actor context.
	targetID := ""
	if filterPractitionerID != "" {
		targetID = filterPractitionerID
	} else if role == util.RolePractitioner {
		targetID = actorID.String()
	}

	if targetID != "" {
		prac, err := s.practitionerSvc.GetPractitioner(ctx, uuid.MustParse(targetID))
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
			{
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
	}

	f := excelize.NewFile()
	sheet := "Profit and Loss"
	f.NewSheet(sheet)
	f.DeleteSheet("Sheet1")

	// --- STYLES ---

	// Main Header
	styleHeaderBlue, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Family: "Calibri", Size: 14},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#DAEEF3"}, Pattern: 1},
	})

	// Section Title
	styleSectionTitle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Family: "Calibri", Size: 12},
	})

	// Style for Particulars/Names (Left Aligned)
	styleDataLeft, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Family: "Calibri", Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "left"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})

	// Data Cell Grid (Currency)
	styleDataGrid, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Family: "Calibri", Size: 10},
		CustomNumFmt: func() *string { s := "$#,##0.00;$#,##0.00;$0.00"; return &s }(),
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})

	// Group Total Style
	styleGroupTotal, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Family: "Calibri"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"#DAEEF3"}, Pattern: 1},
		CustomNumFmt: func() *string { s := "$#,##0.00;$#,##0.00;$0.00"; return &s }(),
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})

	// Final Profit Style
	styleProfit, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Family: "Calibri"},
		CustomNumFmt: func() *string { s := "$#,##0.00;$#,##0.00;$0.00"; return &s }(),
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"#c4f0ce"}, Pattern: 1},
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

	setMetaRow := func(cell string, label string, value string) {
		f.SetCellRichText(sheet, cell, []excelize.RichTextRun{
			{Text: label, Font: &excelize.Font{Bold: true, Family: "Calibri", Size: 10}},
			{Text: " " + value, Font: &excelize.Font{Bold: false, Family: "Calibri", Size: 10}},
		})
	}

	// --- RENDER HEADERS ---
	f.SetCellValue(sheet, "A1", "Profit and Loss Report")
	f.MergeCell(sheet, "A1", "B1")
	f.SetCellStyle(sheet, "A1", "B1", styleHeaderBlue)

	currentRow := 2 // Default start if no date

	setMetaRow(fmt.Sprintf("A%d", currentRow), "Exported by:", entityName)
	currentRow++

	// ABN Row (Only if exists)
	if practitionerABN != "" {
		setMetaRow(fmt.Sprintf("A%d", currentRow), "ABN:", practitionerABN)
		currentRow++
	}

	// Period Row
	if data.ReportMetadata.DateFrom != "" && data.ReportMetadata.DateUntil != "" {
		setMetaRow(fmt.Sprintf("A%d", currentRow), "Period:", fmt.Sprintf("%s to %s", formatDateStr(data.ReportMetadata.DateFrom), formatDateStr(data.ReportMetadata.DateUntil)))
		currentRow++
	}

	currentTimeStr := time.Now().Format("02/01/2006, 3:04:05 pm")
	setMetaRow(fmt.Sprintf("A%d", currentRow), "Generated:", currentTimeStr)
	currentRow++

	currentRow++ // Spacer row

	var totalIncomeCell, totalCOSCell, totalOtherCostsCell string

	// Helper closure to render sections
	renderGroup := func(title string, group RsReportGroup) string {

		f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), title)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleSectionTitle)

		// Set the table filter
		if len(group.Accounts) > 0 {
			tableRange := fmt.Sprintf("A%d:A%d", currentRow, currentRow+len(group.Accounts))
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
		for _, acc := range group.Accounts {
			// Column A: Account Name
			f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), acc.CoaName)
			f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleDataLeft)

			// Column B: Total Value
			f.SetCellValue(sheet, fmt.Sprintf("B%d", currentRow), acc.TotalValue)
			f.SetCellStyle(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("B%d", currentRow), styleDataGrid)
			currentRow++
		}
		dataEndRow := currentRow - 1

		totalCell := fmt.Sprintf("B%d", currentRow)
		f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "TOTAL "+title)

		if len(group.Accounts) > 0 {
			formula := fmt.Sprintf("SUBTOTAL(109, B%d:B%d)", dataStartRow, dataEndRow)
			f.SetCellFormula(sheet, totalCell, formula)
		} else {
			f.SetCellValue(sheet, totalCell, 0)
		}

		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("B%d", currentRow), styleGroupTotal)
		currentRow += 2

		return totalCell
	}

	// --- DATA SECTIONS ---
	totalIncomeCell = renderGroup("INCOME", data.Income)
	totalCOSCell = renderGroup("COST OF SALES", data.CostOfSales)

	// --- GROSS PROFIT (Dynamic) ---
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "GROSS PROFIT")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleProfit)

	// Formula: Total Income - Cost of Sales
	f.SetCellFormula(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("%s-%s", totalIncomeCell, totalCOSCell))
	f.SetCellStyle(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("B%d", currentRow), styleProfitGreen)
	grossProfitCell := fmt.Sprintf("B%d", currentRow)
	currentRow += 2

	totalOtherCostsCell = renderGroup("OTHER COSTS", data.OtherCosts)

	// --- NET PROFIT (Dynamic) ---
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "NET PROFIT")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleProfit)

	// Formula: Gross Profit - Other Costs
	f.SetCellFormula(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("%s-%s", grossProfitCell, totalOtherCostsCell))
	f.SetCellStyle(sheet, fmt.Sprintf("B%d", currentRow), fmt.Sprintf("B%d", currentRow), styleProfitGreen)

	// --- FORMATTING ---
	f.SetColWidth(sheet, "A", "A", 45)
	f.SetColWidth(sheet, "B", "B", 20)
	f.UpdateLinkedValue()

	// --- NOTIFICATION LOGIC ---
	finalNotifIDs := notifIDs
	if filterPractitionerID != "" {
		finalNotifIDs = []uuid.UUID{uuid.MustParse(filterPractitionerID)}
	}

	// --- AUDIT LOG ---
	meta := auditctx.GetMetadata(ctx)
	var userIDStr string
	userIDStr = userID.String()
	parsedActorID := actorID.String()

	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: nil,
		UserID:     &userIDStr,
		Action:     auditctx.ActionPLReportExported,
		Module:     auditctx.ModuleReport,
		EntityType: lo.ToPtr(auditctx.EntityPLReport),
		EntityID:   &parsedActorID,
		AfterState: map[string]interface{}{
			"report_type": "Profit and Loss Report",
			"export_type": exportType,
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})

	// Record the Shared Event — only for accountants, never for practitioners.
	if role == util.RoleAccountant && len(finalNotifIDs) > 0 {
		for _, pID := range finalNotifIDs {
			_ = s.eventsSvc.Record(ctx, events.SharedEvent{
				ID:             uuid.New(),
				PractitionerID: pID,
				AccountantID:   actorID,
				ActorID:        userID,
				ActorName:      &fullName,
				ActorType:      role,
				EventType:      "pl_report.exported",
				EntityType:     "REPORT",
				Description:    fmt.Sprintf("Accountant %s exported Profit and Loss Report", fullName),
				CreatedAt:      time.Now(),
				Metadata:       events.JSONBMap{"report_type": "Profit and Loss Report", "export_type": exportType},
			})
		}
	}

	return f, nil
}

// Helper to format date strings from YYYY-MM-DD to DD-MM-YYYY
func formatDateStr(dateStr string) string {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return dateStr
	}
	return t.Format("02-01-2006")
}
