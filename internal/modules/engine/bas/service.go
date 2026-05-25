package bas

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/auth"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/business/clinic"
	"github.com/iamarpitzala/acareca/internal/modules/business/fy"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	"github.com/iamarpitzala/acareca/internal/modules/business/shared/events"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
	"github.com/xuri/excelize/v2"
)

// Service defines the business-logic layer for the BAS module.
type Service interface {
	GetQuarterlySummary(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]RsBASSummary, error)
	GetByAccount(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]RsBASByAccount, error)
	GetMonthly(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]RsBASMonthly, error)
	GetReport(ctx context.Context, f *BASReportFilter, PracIDs []uuid.UUID, userID uuid.UUID, actorID uuid.UUID, role string) (*RsBASReport, error)
	GetBASPreparation(ctx context.Context, actorID uuid.UUID, role string, f *BASFilter, userID uuid.UUID) (*RsBASPreparation, error)
	ExportActivityStatement(ctx context.Context, quarters []QuarterData, prevDates PeriodInfo, exportType string, actorID uuid.UUID, role string, userID uuid.UUID, practitionerIDs []uuid.UUID, filterPractitionerID string) (interface{}, string, error)
	GetPeriodDates(ctx context.Context, f *BASReportFilter) (curr PeriodInfo, prev PeriodInfo, err error)
	GetAllQuartersInYear(ctx context.Context, quarterID uuid.UUID) ([]BASQuarterInfo, error)
	generateActivityExcelReport(ctx context.Context, quarters []QuarterData, prevDates PeriodInfo, entityName string, practitionerABN string) (*bytes.Buffer, error)
	generateActivityHTML(data activityHTMLData, fullName string, practitionerABN string) (string, error)
	ExportBASPreparation(ctx context.Context, data *RsBASPreparation, actorID uuid.UUID, role string, userID uuid.UUID, filter *BASFilter, exportType string, PracIDs []uuid.UUID, filterPractitionerID string) (interface{}, error)
	GetBASAnalytics(ctx context.Context, targetPracIDs []uuid.UUID, f *BASAnalyticsFilter) (*RsBASAnalytics, error)
}

type service struct {
	repo            Repository
	accountantRepo  accountant.Repository
	auditSvc        audit.Service
	clinicRepo      clinic.Repository
	fyRepo          fy.Repository
	eventsSvc       events.Service
	authRepo        auth.Repository
	practitionerSvc practitioner.IService
}

func NewService(repo Repository, accountantRepo accountant.Repository, auditSvc audit.Service, clinicRepo clinic.Repository, fyRepo fy.Repository, eventsSvc events.Service, authRepo auth.Repository, practitionerSvc practitioner.IService) Service {
	return &service{repo: repo, accountantRepo: accountantRepo, auditSvc: auditSvc, clinicRepo: clinicRepo, fyRepo: fyRepo, eventsSvc: eventsSvc, authRepo: authRepo, practitionerSvc: practitionerSvc}
}

func (s *service) GetQuarterlySummary(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]RsBASSummary, error) {
	if err := validateDateFilter(f); err != nil {
		return nil, err
	}
	if err := validateFYID(f); err != nil {
		return nil, err
	}

	rows, err := s.repo.GetQuarterlySummary(ctx, clinicID, f)
	if err != nil {
		return nil, err
	}

	out := make([]RsBASSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ToRs())
	}
	return out, nil
}

func (s *service) GetByAccount(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]RsBASByAccount, error) {
	if err := validateDateFilter(f); err != nil {
		return nil, err
	}
	if err := validateFYID(f); err != nil {
		return nil, err
	}

	rows, err := s.repo.GetByAccount(ctx, clinicID, f)
	if err != nil {
		return nil, err
	}

	out := make([]RsBASByAccount, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ToRs())
	}
	return out, nil
}

func (s *service) GetMonthly(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]RsBASMonthly, error) {
	if err := validateDateFilter(f); err != nil {
		return nil, err
	}

	rows, err := s.repo.GetMonthly(ctx, clinicID, f)
	if err != nil {
		return nil, err
	}

	out := make([]RsBASMonthly, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ToRs())
	}
	return out, nil
}

func validateFYID(f *BASFilter) error {
	if f.FinancialYearID != nil {
		if _, err := parseUUID(*f.FinancialYearID); err != nil {
			return fmt.Errorf("invalid financial_year_id: must be a valid UUID")
		}
	}
	return nil
}

func parseUUID(s string) ([16]byte, error) {
	var id [16]byte
	parsed, err := uuid.Parse(s)
	if err != nil {
		return id, err
	}
	return parsed, nil
}

func (s *service) GetReport(ctx context.Context, f *BASReportFilter, PracIDs []uuid.UUID, userID uuid.UUID, actorID uuid.UUID, role string) (*RsBASReport, error) {
	pracID, err := uuid.Parse(f.PractitionerID)
	if err != nil {
		return nil, fmt.Errorf("invalid practitioner_id")
	}

	var from, to string

	switch {
	case f.QuarterID != nil:
		qID, err := uuid.Parse(*f.QuarterID)
		if err != nil {
			return nil, fmt.Errorf("invalid quarter_id: must be a valid UUID")
		}
		from, to, err = s.repo.GetQuarterDates(ctx, qID)
		if err != nil {
			return nil, err
		}

	case f.Month != nil:
		start, end, err := util.GetMonthRange(*f.Month)
		if err != nil {
			return nil, fmt.Errorf("invalid month: use full month name e.g. January")
		}
		from = start.Format("2006-01-02")
		to = end.Format("2006-01-02")

	default:
		return nil, fmt.Errorf("provide either quarter_id or month filter")
	}

	row, err := s.repo.GetReport(ctx, pracID, from, to)
	if err != nil {
		return nil, err
	}

	// --- AUDIT LOG ---
	meta := auditctx.GetMetadata(ctx)
	var userIDStr string
	userIDStr = userID.String()
	parsedActorID := actorID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: nil,
		UserID:     &userIDStr,
		Action:     auditctx.ActionActivityStatementGenerated,
		Module:     auditctx.ModuleReport,
		EntityType: strPtr(auditctx.EntityActivityStatement),
		EntityID:   &parsedActorID,
		AfterState: map[string]interface{}{
			"report_type": "Activity Statement",
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})

	// Shared Events: only fire for accountants, using the PracIDs passed from the handler.
	// NOTE: When called from ExportBASReport's loop, PracIDs is passed but events are
	// intentionally suppressed there — ExportActivityStatement fires its own events.
	// Here we only fire if this is a direct GetReport call (PracIDs non-empty and role is accountant).
	if role == util.RoleAccountant && len(PracIDs) > 0 {
		var fullName string
		user, err := s.authRepo.FindByID(ctx, userID)
		if err == nil {
			fullName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
		}

		for _, pID := range PracIDs {
			_ = s.eventsSvc.Record(ctx, events.SharedEvent{
				ID:             uuid.New(),
				PractitionerID: pID,
				AccountantID:   actorID,
				ActorID:        userID,
				ActorName:      &fullName,
				ActorType:      role,
				EventType:      "activity_statement.generated",
				EntityType:     "REPORT",
				EntityID:       actorID,
				Description:    fmt.Sprintf("Accountant %s generated Activity Statement", fullName),
				Metadata:       events.JSONBMap{"report_type": "Activity Statement"},
				CreatedAt:      time.Now(),
			})
		}
	}

	return &RsBASReport{
		G1:  row.G1TotalSalesGross,
		A1:  row.Label1AGSTOnSales,
		G11: row.G11TotalPurchasesGross,
		B1:  row.Label1BGSTOnPurchases,
	}, nil
}

func (s *service) GetBASPreparation(ctx context.Context, actorID uuid.UUID, role string, f *BASFilter, userID uuid.UUID) (*RsBASPreparation, error) {
	isAccountant := false
	if role == util.RoleAccountant {
		isAccountant = true
	}

	// Track unique practitioners to notify
	practitionerMap := make(map[uuid.UUID]bool)

	var targetPracIDs []uuid.UUID

	if isAccountant {
		// For accountants, get all practitioners they have access to
		// This is done by fetching clinics and extracting unique practitioner IDs
		commonFilter := f.MapToFilter()
		clinics, err := s.clinicRepo.ListClinicByAccountant(ctx, actorID, commonFilter)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch accessible practitioners: %w", err)
		}

		// Extract unique practitioner IDs from clinics
		for _, clinic := range clinics {
			practitionerMap[clinic.PractitionerID] = true
		}

		// Convert map to slice
		for pID := range practitionerMap {
			targetPracIDs = append(targetPracIDs, pID)
		}

		// If no practitioners found, return empty result instead of error
		if len(targetPracIDs) == 0 {
			return &RsBASPreparation{Columns: []BASColumn{}}, nil
		}
	} else {
		// For practitioners, use their own ID
		targetPracIDs = []uuid.UUID{actorID}
		practitionerMap[actorID] = true
	}

	// Fetch BAS line items without clinic filtering
	var rawRows []*BASLineItemRow
	nilClinic := uuid.Nil
	rows, err := s.repo.GetBASLineItems(ctx, targetPracIDs, &nilClinic, f)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch BAS items: %w", err)
	}
	rawRows = append(rawRows, rows...)

	// rowKey identifies a unique account slot per (coa_id, section_type).
	// We intentionally exclude bas_category so that multiple DB rows for the
	// same coa_id (e.g. TAXABLE + GST_FREE entries) are treated as the same
	// account and all flow into mapToBASColumn for correct summing.
	rowKey := func(r *BASLineItemRow) string {
		sec := ""
		if r.SectionType != nil {
			sec = *r.SectionType
		}
		return fmt.Sprintf("%s-%s", r.CoaID, sec)
	}

	// masterAccounts tracks which account slots exist across all quarters.
	// We store a slice of rows per key so every bas_category variant is preserved.
	masterAccounts := make(map[string][]*BASLineItemRow)
	for _, r := range rawRows {
		key := rowKey(r)
		masterAccounts[key] = append(masterAccounts[key], r)
	}

	// quarterGroups indexes rows by their period_quarter date string so we can
	// look them up when iterating over the requested quarter IDs.
	quarterGroups := make(map[string][]*BASLineItemRow)
	for _, r := range rawRows {
		k := r.PeriodQuarter.Format("2006-01-02")
		quarterGroups[k] = append(quarterGroups[k], r)
	}

	resp := &RsBASPreparation{Columns: []BASColumn{}}
	var finalizedRowsForTotal []*BASLineItemRow

	// --- STEP 3: ITERATE SELECTED QUARTERS & NORMALIZE ---
	if len(f.ParsedQuarterIDs) > 0 {
		for _, qID := range f.ParsedQuarterIDs {
			qInfo, err := s.repo.GetQuarterInfoByID(ctx, qID)
			if err != nil {
				continue
			}

			// Index this quarter's rows by their account key for O(1) lookup.
			quarterRowIndex := make(map[string][]*BASLineItemRow)
			for _, qr := range quarterGroups[qInfo.StartDate] {
				key := rowKey(qr)
				quarterRowIndex[key] = append(quarterRowIndex[key], qr)
			}

			normalizedRows := make([]*BASLineItemRow, 0)
			for key := range masterAccounts {
				foundRows, exists := quarterRowIndex[key]
				if !exists {
					continue
				}
				// Include all rows for this account (all bas_category variants).
				for _, foundRow := range foundRows {
					if foundRow.GrossAmount != 0 || foundRow.GstAmount != 0 || foundRow.NetAmount != 0 {
						normalizedRows = append(normalizedRows, foundRow)
					}
				}
			}

			finalizedRowsForTotal = append(finalizedRowsForTotal, normalizedRows...)

			col := s.mapToBASColumn(normalizedRows)
			col.Quarter = *qInfo
			resp.Columns = append(resp.Columns, col)
		}
	}

	// --- CRITICAL SORTING STEP ---
	// This ensures Q1 comes before Q2, even if Q3 is missing.
	sort.Slice(resp.Columns, func(i, j int) bool {
		return resp.Columns[i].Quarter.StartDate < resp.Columns[j].Quarter.StartDate
	})

	// Build Grand Total last
	resp.GrandTotal = s.mapToBASColumn(finalizedRowsForTotal)
	resp.GrandTotal.Quarter.Name = "Total"

	// --- AUDIT LOG ---
	meta := auditctx.GetMetadata(ctx)
	var userIDStr string
	userIDStr = userID.String()
	parsedActorID := actorID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: nil,
		UserID:     &userIDStr,
		Action:     auditctx.ActionBASReportGenerated,
		Module:     auditctx.ModuleReport,
		EntityType: strPtr(auditctx.EntityBASReport),
		EntityID:   &parsedActorID,
		AfterState: map[string]interface{}{
			"report_type":    "Quarterly BAS Report",
			"financial_year": f.FinancialYearID,
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})

	if isAccountant {
		// Fetching user details
		var fullName string
		user, err := s.authRepo.FindByID(ctx, userID)
		if err == nil {
			fullName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
		}

		// Record the Shared Event
		for pID := range practitionerMap {
			err = s.eventsSvc.Record(ctx, events.SharedEvent{
				ID:             uuid.New(),
				PractitionerID: pID,
				AccountantID:   actorID,
				ActorID:        userID,
				ActorName:      &fullName,
				ActorType:      role,
				EventType:      "bas_report.generated",
				EntityType:     "REPORT",
				EntityID:       actorID,
				Description:    fmt.Sprintf("Accountant %s generated BAS Report", fullName),
				Metadata:       events.JSONBMap{"report_type": "Quarterly BAS Report", "financial_year": f.FinancialYearID},
				CreatedAt:      time.Now(),
			})
		}
	}
	return resp, nil
}

func (s *service) mapToBASColumn(rows []*BASLineItemRow) BASColumn {
	type accGroup struct {
		Name    string
		Amounts BASAmount
	}

	var col BASColumn
	col.Sections.Income.Items = make([]BASLineItem, 0)
	col.Sections.Expenses.Items = make([]BASLineItem, 0)

	incomeOrder := []string{}
	incomeAccounts := map[string]*accGroup{}

	expenseOrder := []string{}
	expenseAccounts := map[string]*accGroup{}

	var b1 BASAmount
	var mgtFee, labWork BASAmount // Only keep these two as separate totals

	for _, r := range rows {
		if BASCategory(r.BasCategory) == BASCategoryBASExcluded {
			continue
		}

		sectionType := ""
		if r.SectionType != nil {
			sectionType = strings.ToUpper(*r.SectionType)
		}

		if sectionType == "COLLECTION" {
			// Use coa_id as the key to prevent duplicate accounts with same name
			if _, seen := incomeAccounts[r.CoaID]; !seen {
				incomeOrder = append(incomeOrder, r.CoaID)
				incomeAccounts[r.CoaID] = &accGroup{Name: r.AccountName}
			}

			// For GST-free items, don't add GST amount
			gstToAdd := r.GstAmount
			if BASCategory(r.BasCategory) == BASCategoryGSTFree {
				gstToAdd = 0
			}

			incomeAccounts[r.CoaID].Amounts.Gross += r.GrossAmount
			incomeAccounts[r.CoaID].Amounts.GST += gstToAdd
			incomeAccounts[r.CoaID].Amounts.Net += r.NetAmount
			continue
		}

		// --- Process All Expenses ---
		// For GST-free items, don't add GST amount
		gstToAdd := r.GstAmount
		if BASCategory(r.BasCategory) == BASCategoryGSTFree {
			gstToAdd = 0
		}

		b1.Gross += gstToAdd
		accNameLower := strings.ToLower(r.AccountName)

		switch {
		case strings.Contains(accNameLower, "management"):
			mgtFee.Gross += r.GrossAmount
			mgtFee.GST += gstToAdd
			mgtFee.Net += r.NetAmount
		case strings.Contains(accNameLower, "lab"):
			labWork.Gross += r.GrossAmount
			labWork.GST += gstToAdd
			labWork.Net += r.NetAmount
		default:
			// Treat everything else as an individual line item
			// Use a composite key with coa_id to prevent duplicate accounts with same name
			compositeKey := fmt.Sprintf("%s-%s", r.CoaID, r.AccountName)
			if _, seen := expenseAccounts[compositeKey]; !seen {
				expenseOrder = append(expenseOrder, compositeKey)
				expenseAccounts[compositeKey] = &accGroup{Name: r.AccountName}
			}
			expenseAccounts[compositeKey].Amounts.Gross += r.GrossAmount
			expenseAccounts[compositeKey].Amounts.GST += gstToAdd
			expenseAccounts[compositeKey].Amounts.Net += r.NetAmount
		}
	}

	finalize := func(amt BASAmount) BASAmount {
		return BASAmount{
			Gross: roundToTwo(amt.Gross),
			GST:   roundToTwo(amt.GST),
			Net:   roundToTwo(amt.Net),
		}
	}

	// --- Income ---
	var totalIncome BASAmount
	for _, cid := range incomeOrder {
		acc := incomeAccounts[cid]
		fAmts := finalize(acc.Amounts)

		// Add all income items (filtering already done during normalization)
		col.Sections.Income.Items = append(col.Sections.Income.Items, BASLineItem{Name: acc.Name, Amounts: fAmts})

		totalIncome.Gross += fAmts.Gross
		totalIncome.GST += fAmts.GST
		totalIncome.Net += fAmts.Net
	}
	totalIncome = finalize(totalIncome)

	// --- Expenses ---
	mgtFee = finalize(mgtFee)
	labWork = finalize(labWork)

	// Only add Management Fee and Lab Work if they have non-zero values
	col.Sections.Expenses.Items = []BASLineItem{}

	if mgtFee.Gross != 0 || mgtFee.GST != 0 || mgtFee.Net != 0 {
		col.Sections.Expenses.Items = append(col.Sections.Expenses.Items, BASLineItem{
			Name:    "Management Fee (Gross Up)",
			Amounts: mgtFee,
		})
	}

	if labWork.Gross != 0 || labWork.GST != 0 || labWork.Net != 0 {
		col.Sections.Expenses.Items = append(col.Sections.Expenses.Items, BASLineItem{
			Name:    "Laboratory Work (GST Free)",
			Amounts: labWork,
		})
	}

	tGross := mgtFee.Gross + labWork.Gross
	tGST := mgtFee.GST + labWork.GST
	tNet := mgtFee.Net + labWork.Net

	for _, name := range expenseOrder {
		acc := expenseAccounts[name]
		fAmts := finalize(acc.Amounts)

		// Only add expense items with non-zero values
		if fAmts.Gross != 0 || fAmts.GST != 0 || fAmts.Net != 0 {
			col.Sections.Expenses.Items = append(col.Sections.Expenses.Items, BASLineItem{
				Name:    acc.Name,
				Amounts: fAmts,
			})
		}

		tGross += fAmts.Gross
		tGST += fAmts.GST
		tNet += fAmts.Net
	}

	subtotalExpenses := BASAmount{
		Gross: roundToTwo(tGross),
		GST:   roundToTwo(tGST),
		Net:   roundToTwo(tNet),
	}

	// --- GST Payable ---
	col.NetGSTPayable = roundToTwo(totalIncome.GST - subtotalExpenses.GST)

	return col
}

// Helper to round values after calculation
func roundToTwo(val float64) float64 {
	return math.Round(val*100) / 100
}

func ptrString(s string) *string {
	return &s
}

type QuarterData struct {
	Period PeriodInfo
	Report *RsBASReport
}

func (s *service) ExportActivityStatement(ctx context.Context, quarters []QuarterData, prevDates PeriodInfo, exportType string, actorID uuid.UUID, role string, userID uuid.UUID, practitionerIDs []uuid.UUID, filterPractitionerID string) (interface{}, string, error) {
	parsedActorID := actorID.String()

	// --- FETCH METADATA ---
	var fullName string
	user, err := s.authRepo.FindByID(ctx, userID)
	if err == nil {
		fullName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
	}

	var entityName string
	var practitionerABN string
	targetID := ""
	// Use the practitioner ID in filter
	if filterPractitionerID != "" {
		targetID = filterPractitionerID
	} else if role == util.RolePractitioner {
		// If role is practitioner, use actorID
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

	var result interface{}
	var contentType string

	// 1. Branching Logic
	if strings.ToLower(exportType) == "pdf" {
		data := activityHTMLData{
			Quarters: quarters,
			Prev:     prevDates,
		}
		result, err = s.generateActivityHTML(data, fullName, practitionerABN)
		if err != nil {
			return "", "", fmt.Errorf("failed to generate activity html: %w", err)
		}
		contentType = "text/html"
	} else {
		// 2. Default to Excel logic
		result, err = s.generateActivityExcelReport(ctx, quarters, prevDates, entityName, practitionerABN)
		if err != nil {
			return "", "", fmt.Errorf("failed to generate activity excel: %w", err)
		}
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	}

	// --- AUDIT LOG ---
	meta := auditctx.GetMetadata(ctx)
	var userIDStr string
	userIDStr = userID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: nil,
		UserID:     &userIDStr,
		Action:     auditctx.ActionActivityStatementExported,
		Module:     auditctx.ModuleReport,
		EntityType: strPtr(auditctx.EntityActivityStatement),
		EntityID:   &parsedActorID,
		AfterState: map[string]interface{}{
			"report_type": "Activity Statement",
			"export_type": exportType,
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})

	// Record the Shared Event — only for accountants, never for practitioners.
	if role == util.RoleAccountant && len(practitionerIDs) > 0 {
		for _, pID := range practitionerIDs {
			_ = s.eventsSvc.Record(ctx, events.SharedEvent{
				ID:             uuid.New(),
				PractitionerID: pID,
				AccountantID:   actorID,
				ActorID:        userID,
				ActorName:      &fullName,
				ActorType:      role,
				EventType:      "activity_statement.exported",
				EntityType:     "REPORT",
				EntityID:       actorID,
				Description:    fmt.Sprintf("Accountant %s exported Activity Statement", fullName),
				Metadata:       events.JSONBMap{"report_type": "Activity Statement", "export_type": exportType},
				CreatedAt:      time.Now(),
			})
		}
	}

	return result, contentType, nil
}

func (s *service) generateActivityExcelReport(ctx context.Context, quarters []QuarterData, prevDates PeriodInfo, entityName string, practitionerABN string) (*bytes.Buffer, error) {

	xl := excelize.NewFile()
	defer xl.Close()

	sheet := "Activity Statement"
	dataSheet := "SourceData"
	xl.SetSheetName("Sheet1", sheet)
	xl.NewSheet(dataSheet)

	// --- 1. Populate Hidden Data Sheet (The Lookup Table) ---
	headers := []string{"Label", "G1", "1A", "G11", "1B", "Start", "End"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		xl.SetCellValue(dataSheet, cell, h)
	}

	for i, q := range quarters {
		row := i + 2
		g1, a1, g11, b1 := 0.0, 0.0, 0.0, 0.0
		if q.Report != nil {
			g1, a1, g11, b1 = q.Report.G1, q.Report.A1, q.Report.G11, q.Report.B1
		}

		xl.SetCellValue(dataSheet, fmt.Sprintf("A%d", row), q.Period.Label)
		xl.SetCellValue(dataSheet, fmt.Sprintf("B%d", row), g1)
		xl.SetCellValue(dataSheet, fmt.Sprintf("C%d", row), a1)
		xl.SetCellValue(dataSheet, fmt.Sprintf("D%d", row), g11)
		xl.SetCellValue(dataSheet, fmt.Sprintf("E%d", row), b1)
		xl.SetCellValue(dataSheet, fmt.Sprintf("F%d", row), q.Period.From)
		xl.SetCellValue(dataSheet, fmt.Sprintf("G%d", row), q.Period.To)
	}
	xl.SetSheetVisible(dataSheet, false)

	// --- 2. Styles ---
	headerStyle, _ := xl.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#4EA7B3"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	subHeaderStyle, _ := xl.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E1F0F2"}, Pattern: 1},
	})
	labelStyle, _ := xl.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	currencyStyle, _ := xl.NewStyle(&excelize.Style{CustomNumFmt: ptrString("$#,##0.00")})
	totalRowStyle, _ := xl.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#4EA7B3"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})

	// --- 3. Main Header ---
	xl.SetCellValue(sheet, "A1", "Activity Statement Information")
	xl.SetCellValue(sheet, "B1", "BAS")
	xl.SetCellStyle(sheet, "A1", "B1", headerStyle)

	// Helper to format dates to DD-MM-YYYY
	formatDate := func(d string) string {
		t, err := time.Parse("2006-01-02", d)
		if err != nil {
			return d
		}
		return t.Format("02-01-2006")
	}

	setRichMeta := func(cell string, label string, value string) {
		xl.SetCellRichText(sheet, cell, []excelize.RichTextRun{
			{Text: label, Font: &excelize.Font{Bold: true, Family: "Calibri", Size: 10}},
			{Text: " " + value, Font: &excelize.Font{Bold: false, Family: "Calibri", Size: 10}},
		})
	}

	// --- Metadata Rows ---
	rowOffset := 2
	setRichMeta(fmt.Sprintf("A%d", rowOffset), "Exported by:", entityName)
	rowOffset++

	if practitionerABN != "" {
		setRichMeta(fmt.Sprintf("A%d", rowOffset), "ABN:", practitionerABN)
		rowOffset++
	}

	if len(quarters) > 0 {
		periodRange := fmt.Sprintf("%s (%s to %s)",
			quarters[0].Period.Label,
			formatDate(quarters[0].Period.From),
			formatDate(quarters[len(quarters)-1].Period.To),
		)
		setRichMeta(fmt.Sprintf("A%d", rowOffset), "Period:", periodRange)
		rowOffset++
	}

	currentTimeStr := time.Now().Format("02/01/2006, 3:04:05 pm")
	setRichMeta(fmt.Sprintf("A%d", rowOffset), "Generated:", currentTimeStr)
	rowOffset++

	rowOffset++ // Spacer

	// Quarter dropdown row
	qtrRow := rowOffset
	var qLabels []string
	for _, q := range quarters {
		qLabels = append(qLabels, q.Period.Label)
	}
	dv := excelize.NewDataValidation(true)
	dv.Sqref = fmt.Sprintf("B%d", qtrRow)
	dv.SetDropList(qLabels)
	xl.AddDataValidation(sheet, dv)
	xl.SetCellValue(sheet, fmt.Sprintf("A%d", qtrRow), "Qtr")
	xl.SetCellStyle(sheet, fmt.Sprintf("A%d", qtrRow), fmt.Sprintf("A%d", qtrRow), labelStyle)
	if len(qLabels) > 0 {
		xl.SetCellValue(sheet, fmt.Sprintf("B%d", qtrRow), qLabels[0])
	}
	rowOffset++

	// Period start / end (VLOOKUP from hidden data sheet)
	xl.SetCellValue(sheet, fmt.Sprintf("A%d", rowOffset), "Period start")
	xl.SetCellFormula(sheet, fmt.Sprintf("B%d", rowOffset), fmt.Sprintf("=VLOOKUP(B%d, %s!A:G, 6, FALSE)", qtrRow, dataSheet))
	xl.SetCellStyle(sheet, fmt.Sprintf("A%d", rowOffset), fmt.Sprintf("A%d", rowOffset), labelStyle)
	periodStartCell := fmt.Sprintf("B%d", rowOffset)
	rowOffset++

	xl.SetCellValue(sheet, fmt.Sprintf("A%d", rowOffset), "Period end")
	xl.SetCellFormula(sheet, fmt.Sprintf("B%d", rowOffset), fmt.Sprintf("=VLOOKUP(B%d, %s!A:G, 7, FALSE)", qtrRow, dataSheet))
	xl.SetCellStyle(sheet, fmt.Sprintf("A%d", rowOffset), fmt.Sprintf("A%d", rowOffset), labelStyle)
	periodEndCell := fmt.Sprintf("B%d", rowOffset)
	rowOffset++

	rowOffset++ // spacer before GST section

	// --- 5. GST Section ---
	gstFields := []struct {
		Label string
		Col   int
	}{
		{"G1 (Total Sales)", 2},
		{"1A (GST on Sales)", 3},
		{"G11 (Total Purchases)", 4},
		{"1B (GST on Purchases)", 5},
	}

	gstStartRow := rowOffset
	rowIdx := rowOffset
	for _, f := range gstFields {
		xl.SetCellValue(sheet, "A"+strconv.Itoa(rowIdx), f.Label)
		xl.SetCellFormula(sheet, "B"+strconv.Itoa(rowIdx), fmt.Sprintf("=VLOOKUP(B%d, %s!A:G, %d, FALSE)", qtrRow, dataSheet, f.Col))
		xl.SetCellStyle(sheet, "B"+strconv.Itoa(rowIdx), "B"+strconv.Itoa(rowIdx), currencyStyle)
		rowIdx++
	}
	rowOffset = rowIdx

	// Cells for GST Payable formula (1A is row gstStartRow+1, 1B is row gstStartRow+3)
	cell1A := fmt.Sprintf("B%d", gstStartRow+1)
	cell1B := fmt.Sprintf("B%d", gstStartRow+3)

	// --- 6. PAYG Tax Withheld Section ---
	rowOffset++ // spacer
	xl.SetCellValue(sheet, "A"+strconv.Itoa(rowOffset), "PAYG tax withheld")
	xl.SetCellStyle(sheet, "A"+strconv.Itoa(rowOffset), "B"+strconv.Itoa(rowOffset), subHeaderStyle)
	rowOffset++

	paygWithheld := []string{
		"Period start",
		"Period end",
		"W1 (Total Wages, salary and other payments)",
		"W2 (Amount withheld from payments shown at W1)",
		"W3 (Other amounts withheld)",
		"W4 (Amount withheld where no ABN is quoted)",
		"W5 (Total amounts withheld)",
	}

	for _, label := range paygWithheld {
		xl.SetCellValue(sheet, "A"+strconv.Itoa(rowOffset), label)
		if label == "Period start" {
			xl.SetCellFormula(sheet, "B"+strconv.Itoa(rowOffset), periodStartCell)
		} else if label == "Period end" {
			xl.SetCellFormula(sheet, "B"+strconv.Itoa(rowOffset), periodEndCell)
		}
		rowOffset++
	}

	// --- 7. PAYG Instalment Section ---
	rowOffset++
	xl.SetCellValue(sheet, "A"+strconv.Itoa(rowOffset), "PAYG instalment")
	xl.SetCellStyle(sheet, "A"+strconv.Itoa(rowOffset), "B"+strconv.Itoa(rowOffset), subHeaderStyle)
	rowOffset++
	xl.SetCellValue(sheet, "A"+strconv.Itoa(rowOffset), "Option 1")
	xl.SetCellStyle(sheet, "A"+strconv.Itoa(rowOffset), "A"+strconv.Itoa(rowOffset), labelStyle)
	rowOffset++
	xl.SetCellValue(sheet, "A"+strconv.Itoa(rowOffset), "Option 2")
	xl.SetCellStyle(sheet, "A"+strconv.Itoa(rowOffset), "A"+strconv.Itoa(rowOffset), labelStyle)
	rowOffset++

	// --- 8. GST Refund/Payable ---
	rowOffset++
	xl.SetCellValue(sheet, "A"+strconv.Itoa(rowOffset), "GST Payable or (Refund)")
	xl.SetCellFormula(sheet, "B"+strconv.Itoa(rowOffset), fmt.Sprintf("=%s-%s", cell1A, cell1B))
	xl.SetCellStyle(sheet, "A"+strconv.Itoa(rowOffset), "A"+strconv.Itoa(rowOffset), totalRowStyle)
	xl.SetCellStyle(sheet, "B"+strconv.Itoa(rowOffset), "B"+strconv.Itoa(rowOffset), currencyStyle)

	xl.SetColWidth(sheet, "A", "A", 55)
	xl.SetColWidth(sheet, "B", "B", 25)

	return xl.WriteToBuffer()
}

const activityTemplate = `
<html>
<head>
<style>
    body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; font-size: 10pt; padding: 20px; color: #000; }
    table { width: 100%; border-collapse: collapse; margin-bottom: 20px; table-layout: fixed; }
    th, td { border: 1px solid #bfbfbf; padding: 8px; word-wrap: break-word; }
    .header { background-color: #4EA7B3; color: white; font-weight: bold; text-align: center; }
    .sub-header { background-color: #E1F0F2; font-weight: bold; color: #2A5D63; }
    .label { font-weight: bold; width: 70%; }
    .amount { text-align: right; width: 30%; font-family: 'Courier New', Courier, monospace; font-weight: bold;}
    .total-row { background-color: #4EA7B3; color: white; font-weight: bold; }
    .meta td { border: none; padding: 3px 0; font-size: 10pt; }
    .indent { padding-left: 25px; font-weight: normal; }
</style>
</head>
<body>
    {{$q := index .Quarters 0}}

    <table>
        <tr>
            <td class="header">Activity Statement Information</td>
            <td class="header">BAS</td>
        </tr>
    </table>
    <table class="meta">
        <tr><td><b>Exported by:</b> {{.FullName}}</td></tr>
        {{if .PractitionerABN}}<tr><td><b>ABN:</b> {{.PractitionerABN}}</td></tr>{{end}}
        <tr><td><b>Period:</b> {{$q.Period.Label}} ({{$q.Period.From}} to {{$q.Period.To}})</td></tr>
    </table>

    <table>
        <tr>
            <td class="label">Period start</td>
            <td>{{$q.Period.From}}</td>
        </tr>
        <tr>
            <td class="label">Period end</td>
            <td>{{$q.Period.To}}</td>
        </tr>
        <tr>
            <td class="label">Qtr</td>
            <td>{{$q.Period.Label}}</td>
        </tr>
    </table>

    <table>
        <tr class="sub-header"><td colspan="2">GST Section</td></tr>
        <tr>
            <td class="label">G1 (Total Sales)</td>
            <td class="amount">${{printf "%.2f" $q.Report.G1}}</td>
        </tr>
        <tr>
            <td class="label">1A (GST on Sales)</td>
            <td class="amount">${{printf "%.2f" $q.Report.A1}}</td>
        </tr>
        <tr>
            <td class="label">G11 (Total Purchases)</td>
            <td class="amount">${{printf "%.2f" $q.Report.G11}}</td>
        </tr>
        <tr>
            <td class="label">1B (GST on Purchases)</td>
            <td class="amount">${{printf "%.2f" $q.Report.B1}}</td>
        </tr>
    </table>

    <table>
        <tr class="sub-header"><td colspan="2">PAYG tax withheld</td></tr>
        <tr><td class="label">Period start</td><td>{{$q.Period.From}}</td></tr>
        <tr><td class="label">Period end</td><td>{{$q.Period.To}}</td></tr>
        <tr><td class="label">W1 (Total Wages, salary and other payments)</td><td>-</td></tr>
        <tr><td class="label">W2 (Amount withheld from payments shown at W1)</td><td>-</td></tr>
        <tr><td class="label">W3 (Other amounts withheld)</td><td>-</td></tr>
        <tr><td class="label">W4 (Amount withheld where no ABN is quoted)</td><td>-</td></tr>
        <tr><td class="label">W5 (Total amounts withheld)</td><td>-</td></tr>
    </table>

    <table>
        <tr class="sub-header"><td colspan="2">PAYG instalment</td></tr>
        <tr><td class="label">Option 1</td><td>-</td></tr>
        <tr><td class="label">Option 2</td><td>-</td></tr>
    </table>

    <table>
        <tr class="total-row">
            <td class="label">GST Payable or (Refund)</td>
            <td class="amount">${{calcRefund $q.Report.A1 $q.Report.B1}}</td>
        </tr>
    </table>
</body>
</html>
`

func (s *service) generateActivityHTML(data activityHTMLData, fullName string, practitionerABN string) (string, error) {
	// Wrap the incoming data with metadata fields the template needs
	type templateData struct {
		Quarters        []QuarterData
		Prev            PeriodInfo
		FullName        string
		PractitionerABN string
	}

	td := templateData{
		Quarters:        data.Quarters,
		Prev:            data.Prev,
		FullName:        fullName,
		PractitionerABN: practitionerABN,
	}

	tmpl, err := template.New("activity").Funcs(template.FuncMap{
		"calcRefund": func(a1, b1 float64) string {
			return fmt.Sprintf("%.2f", a1-b1)
		},
	}).Parse(activityTemplate)
	if err != nil {
		return "", err
	}

	var htmlBuf bytes.Buffer

	// Print button that only shows on screen, not on the PDF/Printout
	b := `<div class="no-print" style="width:100%;text-align:right;margin-bottom:15px;">
	<button onclick="window.print()" style="padding:10px 20px;background:#DAEEF3;color:#000;border:1.2pt solid #000;border-radius:4px;cursor:pointer;font-weight:bold;font-family:sans-serif;">Print to PDF</button>
	<style>@media print{.no-print{display:none}}</style></div>`

	if err := tmpl.Execute(&htmlBuf, td); err != nil {
		return "", err
	}

	finalHTML := strings.Replace(htmlBuf.String(), "<body>", "<body>"+b, 1)
	return finalHTML, nil
}

type activityHTMLData struct {
	Quarters []QuarterData
	Prev     PeriodInfo
}

type PeriodInfo struct {
	From  string
	To    string
	Label string
}

func (s *service) GetPeriodDates(ctx context.Context, f *BASReportFilter) (curr, prev PeriodInfo, err error) {
	var start, end time.Time

	// 1. Get Current Range
	if f.QuarterID != nil {
		qID, _ := uuid.Parse(*f.QuarterID)
		fromStr, toStr, err := s.repo.GetQuarterDates(ctx, qID)
		if err != nil {
			return curr, prev, err
		}
		start, _ = time.Parse("2006-01-02", fromStr)
		end, _ = time.Parse("2006-01-02", toStr)
	} else if f.Month != nil {
		start, end, err = util.GetMonthRange(*f.Month)
		if err != nil {
			return curr, prev, err
		}
	}

	// 2. Custom Quarter Mapping for your project
	// Jan-Mar: Q3 | Apr-Jun: Q4 | Jul-Sep: Q1 | Oct-Dec: Q2
	getProjectQuarter := func(t time.Time) string {
		month := t.Month()
		var qNum int
		var qMonths string

		switch {
		case month >= 1 && month <= 3:
			qNum = 3
			qMonths = "January-March"
		case month >= 4 && month <= 6:
			qNum = 4
			qMonths = "April-June"
		case month >= 7 && month <= 9:
			qNum = 1
			qMonths = "July-September"
		case month >= 10 && month <= 12:
			qNum = 2
			qMonths = "October-December"
		}
		return fmt.Sprintf("Q%d (%s) %d", qNum, qMonths, t.Year())
	}

	// 3. Set Current Period
	curr.From = start.Format("02-Jan-06")
	curr.To = end.Format("02-Jan-06")
	curr.Label = getProjectQuarter(start)

	// 4. Set Previous Period (Preceding Quarter = Current Start - 3 Months)
	// Example: If current is April (Q4), prevStart becomes January (Q3)
	prevStart := start.AddDate(0, -3, 0)

	// We calculate the end of that previous quarter
	// (3 months from prevStart, then minus 1 day)
	prevEnd := prevStart.AddDate(0, 3, 0).Add(-time.Hour * 24)

	prev.From = prevStart.Format("02-Jan-06")
	prev.To = prevEnd.Format("02-Jan-06")
	prev.Label = getProjectQuarter(prevStart)

	return curr, prev, nil
}

func (s *service) GetAllQuartersInYear(ctx context.Context, quarterID uuid.UUID) ([]BASQuarterInfo, error) {
	// 1. Call the repository method to fetch all quarters in the same financial year
	quarters, err := s.repo.GetAllQuartersInYear(ctx, quarterID)
	if err != nil {
		// Log the error if you have a logger, then return
		return nil, fmt.Errorf("service: failed to fetch quarters for year: %w", err)
	}

	// 2. Return the list (it will contain Q1, Q2, Q3, Q4)
	return quarters, nil
}

func (s *service) ExportBASPreparation(ctx context.Context, data *RsBASPreparation, actorID uuid.UUID, role string, userID uuid.UUID, filter *BASFilter, exportType string, PracIDs []uuid.UUID, filterPractitionerID string) (interface{}, error) {
	f := excelize.NewFile()
	sheet := "Quarterly BAS REPORT"
	f.NewSheet(sheet)
	f.DeleteSheet("Sheet1")

	parsedActorID := actorID.String()

	var fullName string
	var entityName string
	var practitionerABN string
	var FY *fy.FinancialYear
	var targetPracIDs []uuid.UUID

	err := util.RunInTransaction(ctx, s.repo.(*repository).db, func(ctx context.Context, tx *sqlx.Tx) error {
		var innerErr error

		// --- FETCH METADATA ---
		user, innerErr := s.authRepo.FindByID(ctx, userID) // Fallback to transactional find if supported by authRepo
		if innerErr == nil {
			fullName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
		}

		targetID := filterPractitionerID
		if targetID == "" && role == util.RolePractitioner {
			targetID = actorID.String()
		}

		if targetID != "" {
			pracUUID, innerErr := uuid.Parse(targetID)
			if innerErr == nil {
				prac, innerErr := s.practitionerSvc.GetPractitioner(ctx, pracUUID)
				if innerErr == nil {
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
				prac, innerErr := s.practitionerSvc.GetPractitioner(ctx, uuid.MustParse(targetID))
				entityName = fullName
				if innerErr == nil {
					if prac.ABN != nil {
						practitionerABN = *prac.ABN
					}
				}
			} else {
				acc, innerErr := s.accountantRepo.GetAccountantByUserID(ctx, userID.String())
				if innerErr == nil {
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

		// Get Financial Year
		parsedID, innerErr := uuid.Parse(*filter.FinancialYearID)
		if innerErr != nil {
			return fmt.Errorf("invalid financial year id: %w", innerErr)
		}

		FY, innerErr = s.fyRepo.GetFinancialYearByID(ctx, parsedID)
		if innerErr != nil {
			return innerErr
		}

		if role == util.RoleAccountant && len(PracIDs) > 0 {
			targetPracIDs = PracIDs
			if len(filter.ParsedClinicIDs) > 0 {
				uniqueOwners := make(map[uuid.UUID]bool)
				for _, cID := range filter.ParsedClinicIDs {
					clinic, innerErr := s.clinicRepo.GetClinicByID(ctx, tx, cID)
					if innerErr == nil {
						uniqueOwners[clinic.PractitionerID] = true
					}
				}
				targetPracIDs = make([]uuid.UUID, 0, len(uniqueOwners))
				for id := range uniqueOwners {
					targetPracIDs = append(targetPracIDs, id)
				}
			}
			for _, pID := range targetPracIDs {
				innerErr = s.eventsSvc.Record(ctx, events.SharedEvent{
					ID:             uuid.New(),
					PractitionerID: pID,
					AccountantID:   actorID,
					ActorID:        userID,
					ActorName:      &fullName,
					ActorType:      role,
					EventType:      "bas_report.exported",
					EntityType:     "REPORT",
					EntityID:       actorID,
					Description:    fmt.Sprintf("Accountant %s exported BAS Report", fullName),
					Metadata:       events.JSONBMap{"report_type": "Quarterly BAS Report", "financial_year": filter.FinancialYearID, "export_type": exportType},
					CreatedAt:      time.Now(),
				})
				if innerErr != nil {
					return fmt.Errorf("failed to log shared event: %w", innerErr)
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// --- STYLES ---
	// Top Headers (Q1, Q2, etc.) - Light Blue, Bold, Black Borders
	styleHeaderBlue, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Family: "Calibri", Size: 11},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#DAEEF3"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})

	// Standard Grid Style (Used for all data cells)
	styleDataGrid, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Family: "Calibri", Size: 10},
		CustomNumFmt: func() *string { s := "$#,##0.00;[Red] $#,##0.00;$0.00"; return &s }(),
		Alignment:    &excelize.Alignment{Horizontal: "left"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})

	// Standard Table Grid Style (Used for all table data cells)
	styleTableGrid, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Family: "Calibri", Size: 10},
		CustomNumFmt: func() *string { s := "$#,##0.00;[Red] $#,##0.00;$0.00"; return &s }(),
		Alignment:    &excelize.Alignment{Horizontal: "right"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"#DAEEF3"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})

	// Section Titles (INCOME, EXPENSES, Net GST Payable) - Bold, Underline, Large
	styleSectionTitle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Family: "Calibri", Size: 12},
	})

	// Net GST Payable (Red Text)
	styleGSTPayableCol, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Color: "dc3545"},
		CustomNumFmt: func() *string { s := "$#,##0.00;$#,##0.00"; return &s }(),
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		},
	})

	// --- DATA PREPARATION ---
	allCols := append(data.Columns, data.GrandTotal)

	setRichMeta := func(cell string, label string, value string) {
		f.SetCellRichText(sheet, cell, []excelize.RichTextRun{
			{Text: label, Font: &excelize.Font{Bold: true, Family: "Calibri", Size: 10}},
			{Text: " " + value, Font: &excelize.Font{Bold: false, Family: "Calibri", Size: 10}},
		})
	}

	// --- RENDER HEADERS ---
	lastColIdx := (len(allCols) * 4)
	lastColName, _ := excelize.ColumnNumberToName(lastColIdx)

	// 1. Financial Year Header (Row 2)
	f.MergeCell(sheet, "A2", fmt.Sprintf("%s2", lastColName))
	f.SetCellValue(sheet, "A2", FY.Label)
	f.SetCellStyle(sheet, "A2", fmt.Sprintf("%s2", lastColName), styleHeaderBlue)

	// 2. Metadata Rows (Rows 3-4)
	f.MergeCell(sheet, "A3", fmt.Sprintf("%s3", lastColName))
	setRichMeta("A3", "Exported by:", entityName)

	f.MergeCell(sheet, "A4", fmt.Sprintf("%s4", lastColName))
	if practitionerABN != "" {
		setRichMeta("A4", "ABN:", practitionerABN)
	}

	f.MergeCell(sheet, "A5", fmt.Sprintf("%s5", lastColName))
	currentTimeStr := time.Now().Format("02/01/2006, 3:04:05 pm")
	setRichMeta("A5", "Generated:", currentTimeStr)

	f.MergeCell(sheet, "A6", fmt.Sprintf("%s6", lastColName)) // Spacer Row

	for i := range allCols {
		cIdx := 1 + (i * 4)
		startCol, _ := excelize.ColumnNumberToName(cIdx + 1)
		midCol, _ := excelize.ColumnNumberToName(cIdx + 2)
		endCol, _ := excelize.ColumnNumberToName(cIdx + 3)

		headerValue := allCols[i].Quarter.Name

		if allCols[i].Quarter.StartDate != "" {
			t, err := time.Parse("2006-01-02", allCols[i].Quarter.StartDate)
			yearStr := ""
			if err == nil {
				yearStr = fmt.Sprintf("%d", t.Year())
			}

			headerValue = fmt.Sprintf("%s (%s) %s",
				allCols[i].Quarter.Name,
				allCols[i].Quarter.DisplayRange,
				yearStr,
			)
		}

		f.MergeCell(sheet, fmt.Sprintf("%s7", startCol), fmt.Sprintf("%s7", endCol))
		f.SetCellValue(sheet, fmt.Sprintf("%s7", startCol), headerValue)
		f.SetCellStyle(sheet, fmt.Sprintf("%s7", startCol), fmt.Sprintf("%s7", endCol), styleHeaderBlue)

		f.SetCellValue(sheet, fmt.Sprintf("%s8", startCol), "Gross")
		f.SetCellValue(sheet, fmt.Sprintf("%s8", midCol), "GST")
		f.SetCellValue(sheet, fmt.Sprintf("%s8", endCol), "Net")
		f.SetCellStyle(sheet, fmt.Sprintf("%s8", startCol), fmt.Sprintf("%s8", endCol), styleHeaderBlue)
	}

	type SectionMeta struct {
		StartRow int
		EndRow   int
	}
	var incomeMeta, expenseMeta SectionMeta

	// --- INCOME SECTION ---
	currentRow := 9
	incomeHeaderRow := currentRow
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "INCOME")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleSectionTitle)
	currentRow++
	incomeMeta.StartRow = currentRow

	incomeRows := s.getUniqueNamesFromSection(allCols, "income")
	for _, name := range incomeRows {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), name)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleDataGrid)

		for i := range allCols {
			cIdx := 1 + (i * 4)
			startCol, _ := excelize.ColumnNumberToName(cIdx + 1)
			midCol, _ := excelize.ColumnNumberToName(cIdx + 2)
			endCol, _ := excelize.ColumnNumberToName(cIdx + 3)

			f.SetCellValue(sheet, fmt.Sprintf("%s%d", startCol, currentRow), 0)
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", midCol, currentRow), 0)
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", endCol, currentRow), 0)

			f.SetCellStyle(sheet, fmt.Sprintf("%s%d", startCol, currentRow), fmt.Sprintf("%s%d", endCol, currentRow), styleTableGrid)
			s.writeFormattedAmounts(f, sheet, cIdx, currentRow, allCols[i].Sections.Income.Items, name, styleTableGrid)
		}
		currentRow++
	}
	incomeMeta.EndRow = currentRow - 1

	if len(incomeRows) > 0 {
		tblRange := fmt.Sprintf("A%d:A%d", incomeHeaderRow, incomeMeta.EndRow)
		showH := true

		f.AddTable(sheet, &excelize.Table{
			Range:         tblRange,
			Name:          "IncomeTable",
			StyleName:     "",
			ShowHeaderRow: &showH,
		})
	}

	// --- EXPENSES SECTION ---
	currentRow += 1
	expenseHeaderRow := currentRow
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "EXPENSES")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleSectionTitle)
	currentRow++
	expenseMeta.StartRow = currentRow

	expenseRows := s.getUniqueNamesFromSection(allCols, "expenses")
	for _, name := range expenseRows {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), name)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleDataGrid)

		for i := range allCols {
			cIdx := 1 + (i * 4)
			startCol, _ := excelize.ColumnNumberToName(cIdx + 1)
			midCol, _ := excelize.ColumnNumberToName(cIdx + 2)
			endCol, _ := excelize.ColumnNumberToName(cIdx + 3)

			f.SetCellValue(sheet, fmt.Sprintf("%s%d", startCol, currentRow), 0)
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", midCol, currentRow), 0)
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", endCol, currentRow), 0)

			f.SetCellStyle(sheet, fmt.Sprintf("%s%d", startCol, currentRow), fmt.Sprintf("%s%d", endCol, currentRow), styleTableGrid)
			s.writeFormattedAmounts(f, sheet, cIdx, currentRow, allCols[i].Sections.Expenses.Items, name, styleTableGrid)
		}
		currentRow++
	}
	expenseMeta.EndRow = currentRow - 1

	if len(expenseRows) > 0 {
		tblRange := fmt.Sprintf("A%d:A%d", expenseHeaderRow, expenseMeta.EndRow)
		showH := true

		f.AddTable(sheet, &excelize.Table{
			Range:         tblRange,
			Name:          "ExpenseTable",
			StyleName:     "",
			ShowHeaderRow: &showH,
		})
	}

	// --- SUMMARY SECTION ---
	currentRow += 2
	netGSTRow := currentRow
	f.SetCellValue(sheet, fmt.Sprintf("A%d", currentRow), "Net GST Payable")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", currentRow), fmt.Sprintf("A%d", currentRow), styleSectionTitle)

	for i := range allCols {
		cIdx := 1 + (i * 4)
		grossCol, _ := excelize.ColumnNumberToName(cIdx + 1)
		gstCol, _ := excelize.ColumnNumberToName(cIdx + 2)
		netCol, _ := excelize.ColumnNumberToName(cIdx + 3)

		f.MergeCell(sheet, fmt.Sprintf("%s%d", grossCol, netGSTRow), fmt.Sprintf("%s%d", netCol, netGSTRow))

		incomeGST := fmt.Sprintf("SUBTOTAL(109, %s%d:%s%d)", gstCol, incomeMeta.StartRow, gstCol, incomeMeta.EndRow)
		expenseGST := fmt.Sprintf("SUBTOTAL(109, %s%d:%s%d)", gstCol, expenseMeta.StartRow, gstCol, expenseMeta.EndRow)
		f.SetCellFormula(sheet, fmt.Sprintf("%s%d", netCol, netGSTRow), fmt.Sprintf("%s-%s", incomeGST, expenseGST))
		f.SetCellStyle(sheet, fmt.Sprintf("%s%d", grossCol, netGSTRow), fmt.Sprintf("%s%d", netCol, netGSTRow), styleGSTPayableCol)
	}

	// --- FINAL DIMENSIONS ---
	f.SetColWidth(sheet, "A", "A", 45)
	for col := 2; col <= 1+(len(allCols)*4); col++ {
		name, _ := excelize.ColumnNumberToName(col)
		if (col-1)%4 == 0 {
			f.SetColWidth(sheet, name, name, 3)
		} else {
			f.SetColWidth(sheet, name, name, 15)
		}
	}

	// --- AUDIT LOG (Asynchronous Execution After Successful DB Operations) ---
	meta := auditctx.GetMetadata(ctx)
	var userIDStr string
	userIDStr = userID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: nil,
		UserID:     &userIDStr,
		Action:     auditctx.ActionBASReportExported,
		Module:     auditctx.ModuleReport,
		EntityType: strPtr(auditctx.EntityBASReport),
		EntityID:   &parsedActorID,
		AfterState: map[string]interface{}{
			"report_type":    "Quarterly BAS Report",
			"financial_year": filter.FinancialYearID,
			"export_type":    exportType,
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})

	if exportType == "pdf" {
		htmlContent, err := s.generateHTMLString(f, sheet, data, FY.Label, fullName, practitionerABN)
		if err != nil {
			return nil, err
		}
		return htmlContent, nil
	}

	return f, nil
}

func (s *service) writeFormattedAmounts(f *excelize.File, sheet string, startIdx, row int, items []BASLineItem, name string, styleID int) {
	for _, item := range items {
		if item.Name == name {
			g, _ := excelize.ColumnNumberToName(startIdx + 1)
			t, _ := excelize.ColumnNumberToName(startIdx + 2)
			n, _ := excelize.ColumnNumberToName(startIdx + 3)

			f.SetCellValue(sheet, fmt.Sprintf("%s%d", g, row), item.Amounts.Gross)
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", t, row), item.Amounts.GST)
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", n, row), item.Amounts.Net)

			f.SetCellStyle(sheet, fmt.Sprintf("%s%d", g, row), fmt.Sprintf("%s%d", n, row), styleID)
			return
		}
	}
}

func (s *service) getUniqueNamesFromSection(allCols []BASColumn, section string) []string {
	m := make(map[string]bool)
	var names []string
	for _, col := range allCols {
		var items []BASLineItem
		if section == "income" {
			items = col.Sections.Income.Items
		} else {
			items = col.Sections.Expenses.Items
		}
		for _, itm := range items {
			if itm.Name != "" && !m[itm.Name] {
				m[itm.Name] = true
				names = append(names, itm.Name)
			}
		}
	}
	return names
}

func strPtr(s string) *string {
	return &s
}

// Helper to convert the Excel file to PDF using HTML
func (s *service) generateHTMLString(f *excelize.File, sheetName string, data *RsBASPreparation, FYLabel string, fullName string, practitionerABN string) (string, error) {
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return "", err
	}

	var b bytes.Buffer
	b.WriteString("<html><head><style>")
	b.WriteString(`
		@page { size: A3 landscape; margin: 0.5cm; }
		body { font-family: 'Calibri', sans-serif; margin: 0; padding: 10px; }
		table { border-collapse: collapse; table-layout: fixed; width: 100%; border: 1.2pt solid #000; }
		td { border: 1px solid #000; padding: 4px 2px; font-size: 8.5pt; height: 22px; text-align: center; }
		.header-blue { background-color: #DAEEF3 !important; font-weight: bold; }
		.fy-title { font-size: 16pt; font-weight: bold; background-color: #DAEEF3 !important; padding: 15px; border: 1.2pt solid #000; }
		.section-title { font-weight: bold; font-size: 11pt; border: none; padding-top: 12px; text-align: left; }
		.data-left { text-align: left; border: 1px solid #000; }
		.text-right { text-align: right; }
		.profit-green { background-color: #c4f0ce !important; font-weight: bold; color: #28a745; text-align: right; }
		.gst-red { font-weight: bold; color: #dc3545; text-align: right; }
		.income-blue td {background-color: #DAEEF3 !important; }
		.expense-blue td {background-color: #DAEEF3 !important; }
        .meta-item { font-size: 10pt; margin-bottom: 4px; }
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

	b.WriteString("<table>")

	// 16 columns: 1 Label + (4 Quarters * 3 Cols) + (1 Total * 3 Cols)
	totalCols := 1 + (len(data.Columns)+1)*3 // +1 for Total

	b.WriteString("<colgroup>")
	b.WriteString("<col style='width: 16%;'>")

	colWidth := 84.0 / float64(totalCols-1)

	for i := 0; i < totalCols-1; i++ {
		b.WriteString(fmt.Sprintf("<col style='width: %.2f%%;'>", colWidth))
	}

	b.WriteString("</colgroup>")

	formatCurr := func(v float64) string { return fmt.Sprintf("$%.2f", v) }

	for rIdx, row := range rows {
		rowNum := rIdx + 1

		// --- ROW 1: FINANCIAL YEAR ---
		if rowNum == 1 {
			// Render FY row BEFORE iterating Excel
			b.WriteString("<tr>")
			b.WriteString(fmt.Sprintf("<td colspan='%d' class='fy-title'>%s</td>", totalCols, FYLabel))
			b.WriteString("</tr>")
			continue
		}

		//  skip empty rows
		if len(row) == 0 {
			continue
		}

		// --- ROW 6: QUARTERS ---
		if rowNum == 6 {
			b.WriteString("<tr>")

			// Column A spacer (Particulars column)
			b.WriteString("<td class='header-blue'></td>")

			// Dynamic quarters from API
			for _, col := range data.Columns {
				// Extract the Year from the startDate
				yearDisplay := ""
				if col.Quarter.StartDate != "" {
					// Parse the "2025-07-01" format
					t, err := time.Parse("2006-01-02", col.Quarter.StartDate)
					if err == nil {
						yearDisplay = fmt.Sprintf(" %d", t.Year())
					}
				}

				// Build the label: Quarter name (Display Range) Year
				label := fmt.Sprintf("%s (%s) %s",
					col.Quarter.Name,
					col.Quarter.DisplayRange,
					yearDisplay,
				)
				b.WriteString(fmt.Sprintf(
					"<td class='header-blue' colspan='3' style='font-size:10pt;'>%s</td>",
					label,
				))
			}

			// Grand Total column (always last)
			b.WriteString("<td class='header-blue' colspan='3' style='font-size:10pt;'>Total</td>")

			b.WriteString("</tr>")
			continue
		}

		// --- ROW 7: SUBHEADERS ---
		if rowNum == 7 {
			b.WriteString("<tr>")
			b.WriteString("<td class='header-blue'>Particulars</td>")
			totalBlocks := len(data.Columns) + 1 // +1 for Total
			for i := 0; i < totalBlocks; i++ {
				b.WriteString("<td class='header-blue'>Gross</td><td class='header-blue'>GST</td><td class='header-blue'>Net</td>")
			}
			b.WriteString("</tr>")
			continue
		}

		// skip header rows completely
		if rowNum <= 6 {
			continue
		}

		// --- DATA ROWS ---
		valA := ""
		if len(row) > 0 {
			valA = row[0]
		}

		classA := "data-left"
		if valA == "INCOME" || valA == "EXPENSES" {
			classA = "section-title"
			b.WriteString("<tr>")
			b.WriteString(fmt.Sprintf("<td colspan='%d' class='%s'>%s</td>", totalCols, classA, valA))
			b.WriteString("</tr>")
			continue
		}

		b.WriteString(fmt.Sprintf("<td class='%s'>%s</td>", classA, valA))

		// Combine data columns (4 quarters + 1 grand total)
		allColumns := append(data.Columns, data.GrandTotal)

		for _, col := range allColumns {
			var g, gst, n float64
			found := false

			// Match Account from API Data
			for _, item := range append(col.Sections.Income.Items, col.Sections.Expenses.Items...) {
				if item.Name == valA {
					g, gst, n = item.Amounts.Gross, item.Amounts.GST, item.Amounts.Net
					found = true
					break
				}
			}

			// Handle Special Rows
			if valA == "Net GST Payable" {
				gst = col.NetGSTPayable
				found = true
			}

			cellClass := "text-right"
			if valA == "Net GST Payable" {
				cellClass += " gst-red"
			}

			if found {
				b.WriteString(fmt.Sprintf("<td class='%s'>%s</td><td class='%s'>%s</td><td class='%s'>%s</td>",
					cellClass, formatCurr(g), cellClass, formatCurr(gst), cellClass, formatCurr(n)))
			} else {
				for i := 0; i < 3; i++ {
					b.WriteString("<td class='text-right'>$0.00</td>")
				}
			}
		}
		b.WriteString("</tr>")
	}

	b.WriteString("</table></body></html>")

	return b.String(), err
}

func (s *service) GetBASAnalytics(ctx context.Context, targetPracIDs []uuid.UUID, f *BASAnalyticsFilter) (*RsBASAnalytics, error) {
	fyID, err := uuid.Parse(f.FinancialYearID)
	if err != nil {
		return nil, fmt.Errorf("invalid financial year id: %w", err)
	}

	fy, err := s.fyRepo.GetFinancialYearByID(ctx, fyID)
	if err != nil {
		return nil, fmt.Errorf("financial year not found: %w", err)
	}

	now := time.Now()
	var resolvedFrom, resolvedTo time.Time

	// Default to the financial year boundaries.
	resolvedFrom, resolvedTo = fy.StartDate, fy.EndDate
	period := strings.ToLower(f.Period)

	//When quarter_ids are provided, shrink the boundaries to exactly that quarter's date range.
	quarterSelected := f.QuarterIDs != nil && *f.QuarterIDs != ""
	if quarterSelected {
		idStrings := strings.Split(*f.QuarterIDs, ",")
		var minStart, maxEnd time.Time
		foundValidQuarter := false

		for _, sID := range idStrings {
			uID, parseErr := uuid.Parse(strings.TrimSpace(sID))
			if parseErr != nil {
				continue
			}

			qInfo, qErr := s.repo.GetQuarterInfoByID(ctx, uID)
			if qErr == nil && qInfo != nil {
				sTime, _ := time.Parse("2006-01-02", qInfo.StartDate)
				eTime, _ := time.Parse("2006-01-02", qInfo.EndDate)

				if minStart.IsZero() || sTime.Before(minStart) {
					minStart = sTime
				}
				if maxEnd.IsZero() || eTime.After(maxEnd) {
					maxEnd = eTime
				}
				foundValidQuarter = true
			}
		}

		if foundValidQuarter {
			resolvedFrom = minStart
			resolvedTo = maxEnd
		}
	} else if period != "" {
		// Period is only applied when no quarter is selected.
		switch period {
		case "today":
			resolvedFrom, resolvedTo = now, now
		case "yesterday":
			resolvedFrom = now.AddDate(0, 0, -1)
			resolvedTo = resolvedFrom
		case "this_week":
			resolvedFrom = now.AddDate(0, 0, -int(now.Weekday()))
			resolvedTo = now
		case "last_week":
			resolvedFrom = now.AddDate(0, 0, -int(now.Weekday())-7)
			resolvedTo = resolvedFrom.AddDate(0, 0, 6)
		case "last_28_days":
			resolvedFrom = now.AddDate(0, 0, -28)
			resolvedTo = now
		case "last_30_days":
			resolvedFrom = now.AddDate(0, 0, -30)
			resolvedTo = now
		case "last_month":
			resolvedFrom = time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location())
			resolvedTo = resolvedFrom.AddDate(0, 1, -1)
		case "custom_range":
			if f.FromDate != nil && *f.FromDate != "" {
				resolvedFrom, _ = time.Parse("2006-01-02", *f.FromDate)
			}
			if f.ToDate != nil && *f.ToDate != "" {
				resolvedTo, _ = time.Parse("2006-01-02", *f.ToDate)
			}
		case "custom_month":
			if f.FromDate != nil && *f.FromDate != "" {
				t, _ := time.Parse("2006-01", *f.FromDate)
				resolvedFrom = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
				resolvedTo = resolvedFrom.AddDate(0, 1, -1)
			}
		}
	}

	// Boundary Clamping ensures the dates never escape the selected Financial Year.
	if !resolvedFrom.IsZero() && resolvedFrom.Before(fy.StartDate) {
		resolvedFrom = fy.StartDate
	}
	if !resolvedTo.IsZero() && resolvedTo.After(fy.EndDate) {
		resolvedTo = fy.EndDate
	}

	// Prepare the repository filter.
	fromStr, toStr := resolvedFrom.Format("2006-01-02"), resolvedTo.Format("2006-01-02")
	repoFilter := &BASFilter{
		FinancialYearID: &f.FinancialYearID,
		QuarterIDs:      f.QuarterIDs,
		FromDate:        &fromStr,
		ToDate:          &toStr,
	}
	_ = repoFilter.MapToFilter()

	// When no quarters are selected and a period filter is active, clear ParsedQuarterIDs so the repo uses the resolved date range instead of quarter-based filtering.
	if !quarterSelected && period != "" {
		repoFilter.ParsedQuarterIDs = nil
	}

	nilClinic := uuid.Nil
	rows, err := s.repo.GetBASAnalytics(ctx, targetPracIDs, &nilClinic, repoFilter)
	if err != nil {
		return nil, err
	}

	incomeMap := make(map[string]*BASAccountGroup)
	expenseMap := make(map[string]*BASAccountGroup)
	incomeTotalsByDate := make(map[string]BASValue)
	expenseTotalsByDate := make(map[string]BASValue)
	dateSet := make(map[string]bool)

	selectedCoas := make(map[string]bool)
	if f.SelectedCoaIDs != nil && *f.SelectedCoaIDs != "" {
		for _, id := range strings.Split(*f.SelectedCoaIDs, ",") {
			selectedCoas[strings.TrimSpace(id)] = true
		}
	}

	for _, r := range rows {
		if BASCategory(r.BasCategory) == BASCategoryBASExcluded {
			continue
		}

		if r.PeriodQuarter.Before(resolvedFrom) || r.PeriodQuarter.After(resolvedTo) {
			continue
		}

		if len(selectedCoas) > 0 && !selectedCoas[r.CoaID] {
			continue
		}

		dateKey := r.PeriodQuarter.Format("2006-01-02")
		dateSet[dateKey] = true
		val := BASValue{
			Date: dateKey, Gross: roundToTwo(r.GrossAmount),
			GST: roundToTwo(r.GstAmount), Net: roundToTwo(r.NetAmount),
		}

		if r.SectionType != nil && strings.ToUpper(*r.SectionType) == "COLLECTION" {
			if _, ok := incomeMap[r.CoaID]; !ok {
				incomeMap[r.CoaID] = &BASAccountGroup{ID: r.CoaID, Name: r.AccountName}
			}
			incomeMap[r.CoaID].Values = append(incomeMap[r.CoaID].Values, val)

			t := incomeTotalsByDate[dateKey]
			t.Date = dateKey
			t.Gross += val.Gross
			t.GST += val.GST
			t.Net += val.Net
			incomeTotalsByDate[dateKey] = t
		} else {
			if _, ok := expenseMap[r.CoaID]; !ok {
				expenseMap[r.CoaID] = &BASAccountGroup{ID: r.CoaID, Name: r.AccountName}
			}
			expenseMap[r.CoaID].Values = append(expenseMap[r.CoaID].Values, val)

			t := expenseTotalsByDate[dateKey]
			t.Date = dateKey
			t.Gross += val.Gross
			t.GST += val.GST
			t.Net += val.Net
			expenseTotalsByDate[dateKey] = t
		}
	}

	// Assemble the final response based on the categorized maps.
	resp := &RsBASAnalytics{}
	secStr := ""
	if f.Sections != nil {
		secStr = *f.Sections
	}

	sortedDates := make([]string, 0, len(dateSet))
	for d := range dateSet {
		sortedDates = append(sortedDates, d)
	}
	sort.Strings(sortedDates)

	// Summary calculations for net profit and GST payable.
	showIncome := strings.Contains(secStr, "income") || secStr == ""
	showExpense := strings.Contains(secStr, "expense") || secStr == ""

	if showIncome && len(incomeMap) > 0 {
		for _, acc := range incomeMap {
			resp.Income = append(resp.Income, *acc)
		}
		var tv []BASValue
		for _, d := range sortedDates {
			tv = append(tv, incomeTotalsByDate[d])
		}
		resp.Income = append(resp.Income, BASAccountGroup{Name: "total", Values: tv})
	}

	if showExpense && len(expenseMap) > 0 {
		for _, acc := range expenseMap {
			resp.Expense = append(resp.Expense, *acc)
		}
		var tv []BASValue
		for _, d := range sortedDates {
			tv = append(tv, expenseTotalsByDate[d])
		}
		resp.Expense = append(resp.Expense, BASAccountGroup{Name: "total", Values: tv})
	}

	if (strings.Contains(secStr, "gstPayable") || secStr == "") && len(dateSet) > 0 {
		resp.GSTPayable = &BASAccountGroup{Name: "gstPayable"}
		for _, d := range sortedDates {
			inc, exp := incomeTotalsByDate[d], expenseTotalsByDate[d]
			resp.GSTPayable.Values = append(resp.GSTPayable.Values, BASValue{Date: d, GST: roundToTwo(inc.GST - exp.GST)})
		}
	}

	return resp, nil
}
