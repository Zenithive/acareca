package bas

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
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
	"github.com/iamarpitzala/acareca/internal/shared/export"
	basexport "github.com/iamarpitzala/acareca/internal/shared/export/bas"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
)

type Service interface {
	GetQuarterlySummary(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]RsBASSummary, error)
	GetByAccount(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]RsBASByAccount, error)
	GetMonthly(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]RsBASMonthly, error)
	GetReport(ctx context.Context, f *BASReportFilter, PracIDs []uuid.UUID, userID uuid.UUID, actorID uuid.UUID, role string) (*RsBASReport, error)
	GetBASPreparation(ctx context.Context, actorID uuid.UUID, role string, f *BASFilter, userID uuid.UUID) (*RsBASPreparation, error)
	ExportActivityStatement(ctx context.Context, quarters []QuarterData, prevDates PeriodInfo, exportType string, actorID uuid.UUID, role string, userID uuid.UUID, practitionerIDs []uuid.UUID, filterPractitionerID string) (interface{}, string, error)
	GetPeriodDates(ctx context.Context, f *BASReportFilter) (curr PeriodInfo, prev PeriodInfo, err error)
	GetAllQuartersInYear(ctx context.Context, quarterID uuid.UUID) ([]BASQuarterInfo, error)
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
		EntityType: lo.ToPtr(auditctx.EntityActivityStatement),
		EntityID:   &parsedActorID,
		AfterState: map[string]interface{}{
			"report_type": "Activity Statement",
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})

	// --- Shared Events ---
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

	practitionerMap := make(map[uuid.UUID]bool)

	var targetPracIDs []uuid.UUID

	if isAccountant {
		commonFilter := f.MapToFilter()
		clinics, err := s.clinicRepo.ListClinicByAccountant(ctx, actorID, commonFilter)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch accessible practitioners: %w", err)
		}

		for _, clinic := range clinics {
			practitionerMap[clinic.PractitionerID] = true
		}

		for pID := range practitionerMap {
			targetPracIDs = append(targetPracIDs, pID)
		}

		if len(targetPracIDs) == 0 {
			return &RsBASPreparation{Columns: []BASColumn{}}, nil
		}
	} else {
		targetPracIDs = []uuid.UUID{actorID}
		practitionerMap[actorID] = true
	}

	var rawRows []*BASLineItemRow
	nilClinic := uuid.Nil
	rows, err := s.repo.GetBASLineItems(ctx, targetPracIDs, &nilClinic, f)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch BAS items: %w", err)
	}
	rawRows = append(rawRows, rows...)

	rowKey := func(r *BASLineItemRow) string {
		sec := ""
		if r.AccountType != nil {
			sec = *r.AccountType
		}
		return fmt.Sprintf("%s-%s", r.CoaID, sec)
	}

	masterAccounts := make(map[string][]*BASLineItemRow)
	for _, r := range rawRows {
		key := rowKey(r)
		masterAccounts[key] = append(masterAccounts[key], r)
	}

	quarterGroups := make(map[string][]*BASLineItemRow)
	for _, r := range rawRows {
		k := r.PeriodQuarter.Format("2006-01-02")
		quarterGroups[k] = append(quarterGroups[k], r)
	}

	resp := &RsBASPreparation{Columns: []BASColumn{}}
	var finalizedRowsForTotal []*BASLineItemRow

	if len(f.ParsedQuarterIDs) > 0 {
		for _, qID := range f.ParsedQuarterIDs {
			qInfo, err := s.repo.GetQuarterInfoByID(ctx, qID)
			if err != nil {
				continue
			}

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

	// This ensures Q1 comes before Q2, even if Q3 is missing.
	sort.Slice(resp.Columns, func(i, j int) bool {
		return resp.Columns[i].Quarter.StartDate < resp.Columns[j].Quarter.StartDate
	})

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
		EntityType: lo.ToPtr(auditctx.EntityBASReport),
		EntityID:   &parsedActorID,
		AfterState: map[string]interface{}{
			"report_type":    "Quarterly BAS Report",
			"financial_year": f.FinancialYearID,
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})

	if isAccountant {
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
	var mgtFee, labWork BASAmount

	for _, r := range rows {
		if BASCategory(r.BasCategory) == BASCategoryBASExcluded {
			continue
		}

		accountType := ""
		if r.AccountType != nil {
			accountType = strings.ToUpper(*r.AccountType)
		}

		gstToAdd := r.GstAmount
		if BASCategory(r.BasCategory) == BASCategoryGSTFree {
			gstToAdd = 0
		}

		if accountType == "REVENUE" {
			if _, seen := incomeAccounts[r.CoaID]; !seen {
				incomeOrder = append(incomeOrder, r.CoaID)
				incomeAccounts[r.CoaID] = &accGroup{Name: r.AccountName}
			}
			incomeAccounts[r.CoaID].Amounts.Gross += r.GrossAmount
			incomeAccounts[r.CoaID].Amounts.GST += gstToAdd
			incomeAccounts[r.CoaID].Amounts.Net += r.NetAmount
			continue
		}

		// Expense
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

		col.Sections.Income.Items = append(col.Sections.Income.Items, BASLineItem{Name: acc.Name, Amounts: fAmts})
		totalIncome.Gross += fAmts.Gross
		totalIncome.GST += fAmts.GST
		totalIncome.Net += fAmts.Net
	}
	totalIncome = finalize(totalIncome)

	// --- Expenses ---
	mgtFee = finalize(mgtFee)
	labWork = finalize(labWork)

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

	for _, key := range expenseOrder {
		acc := expenseAccounts[key]
		fAmts := finalize(acc.Amounts)

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

	col.NetGSTPayable = roundToTwo(totalIncome.GST - subtotalExpenses.GST)

	return col
}

// Helper to round values after calculation
func roundToTwo(val float64) float64 {
	return math.Round(val*100) / 100
}

type QuarterData struct {
	Period PeriodInfo
	Report *RsBASReport
}

func (s *service) ExportActivityStatement(ctx context.Context, quarters []QuarterData, prevDates PeriodInfo, exportType string, actorID uuid.UUID, role string, userID uuid.UUID, practitionerIDs []uuid.UUID, filterPractitionerID string) (interface{}, string, error) {
	parsedActorID := actorID.String()

	var fullName string
	user, err := s.authRepo.FindByID(ctx, userID)
	if err == nil {
		fullName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
	}

	var entityName string
	var practitionerABN string
	targetID := ""
	if filterPractitionerID != "" {
		targetID = filterPractitionerID
	} else if role == util.RolePractitioner {
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

	if strings.ToLower(exportType) == "excel" {
		var exportQuarters []basexport.QuarterData
		exportQuarters = make([]basexport.QuarterData, len(quarters))
		for i, q := range quarters {
			var report *basexport.RsBASReport
			if q.Report != nil {
				report = &basexport.RsBASReport{G1: q.Report.G1, A1: q.Report.A1, G11: q.Report.G11, B1: q.Report.B1}
			}
			exportQuarters[i] = basexport.QuarterData{
				Period: basexport.PeriodInfo{From: q.Period.From, To: q.Period.To, Label: q.Period.Label},
				Report: report,
			}
		}

		config := export.ExportConfig{
			EntityName:     entityName,
			EntityABN:      practitionerABN,
			ExportType:     exportType,
			ExportedByName: fullName,
			GeneratedTime:  time.Now().Format("02/01/2006, 3:04:05 pm"),
		}

		result, err = basexport.GenerateActivityStatementExcelReport(exportQuarters, basexport.PeriodInfo{From: prevDates.From, To: prevDates.To, Label: prevDates.Label}, config)
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
		EntityType: lo.ToPtr(auditctx.EntityActivityStatement),
		EntityID:   &parsedActorID,
		AfterState: map[string]interface{}{
			"report_type": "Activity Statement",
			"export_type": exportType,
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})

	// Record the Shared Event
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

type PeriodInfo struct {
	From  string
	To    string
	Label string
}

func (s *service) GetPeriodDates(ctx context.Context, f *BASReportFilter) (curr, prev PeriodInfo, err error) {
	var start, end time.Time

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

	curr.From = start.Format("02-Jan-06")
	curr.To = end.Format("02-Jan-06")
	curr.Label = getProjectQuarter(start)

	prevStart := start.AddDate(0, -3, 0)

	prevEnd := prevStart.AddDate(0, 3, 0).Add(-time.Hour * 24)

	prev.From = prevStart.Format("02-Jan-06")
	prev.To = prevEnd.Format("02-Jan-06")
	prev.Label = getProjectQuarter(prevStart)

	return curr, prev, nil
}

func (s *service) GetAllQuartersInYear(ctx context.Context, quarterID uuid.UUID) ([]BASQuarterInfo, error) {
	quarters, err := s.repo.GetAllQuartersInYear(ctx, quarterID)
	if err != nil {
		return nil, fmt.Errorf("service: failed to fetch quarters for year: %w", err)
	}

	return quarters, nil
}

func (s *service) ExportBASPreparation(ctx context.Context, data *RsBASPreparation, actorID uuid.UUID, role string, userID uuid.UUID, filter *BASFilter, exportType string, PracIDs []uuid.UUID, filterPractitionerID string) (interface{}, error) {
	parsedActorID := actorID.String()

	var fullName string
	var entityName string
	var practitionerABN string
	var FY *fy.FinancialYear
	var targetPracIDs []uuid.UUID

	err := util.RunInTransaction(ctx, s.repo.(*repository).db, func(ctx context.Context, tx *sqlx.Tx) error {
		var innerErr error

		user, innerErr := s.authRepo.FindByID(ctx, userID)
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

	exportData := &basexport.RsBASPreparation{
		Columns:    make([]basexport.BASColumn, len(data.Columns)),
		GrandTotal: convertBASColumn(data.GrandTotal),
	}
	for i, col := range data.Columns {
		exportData.Columns[i] = convertBASColumn(col)
	}

	config := export.ExportConfig{
		EntityName:     entityName,
		EntityABN:      practitionerABN,
		ExportType:     exportType,
		ExportedByName: fullName,
		GeneratedTime:  time.Now().Format("02/01/2006, 3:04:05 pm"),
	}

	f, err := basexport.GenerateBASPreparationExcelReport(exportData, config, FY.Label)
	if err != nil {
		return nil, fmt.Errorf("failed to generate BAS preparation excel: %w", err)
	}

	meta := auditctx.GetMetadata(ctx)
	var userIDStr string
	userIDStr = userID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: nil,
		UserID:     &userIDStr,
		Action:     auditctx.ActionBASReportExported,
		Module:     auditctx.ModuleReport,
		EntityType: lo.ToPtr(auditctx.EntityBASReport),
		EntityID:   &parsedActorID,
		AfterState: map[string]interface{}{
			"report_type":    "Quarterly BAS Report",
			"financial_year": filter.FinancialYearID,
			"export_type":    exportType,
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})

	return f, nil
}

func convertBASSection(section BASSection) basexport.BASSection {
	items := make([]basexport.BASLineItem, len(section.Items))
	for i, item := range section.Items {
		items[i] = basexport.BASLineItem{
			Name: item.Name,
			Amounts: basexport.BASAmount{
				Gross: item.Amounts.Gross,
				GST:   item.Amounts.GST,
				Net:   item.Amounts.Net,
			},
		}
	}
	return basexport.BASSection{Items: items}
}

func convertBASColumn(col BASColumn) basexport.BASColumn {
	var sections basexport.BASColumn
	sections.Sections.Income = convertBASSection(col.Sections.Income)
	sections.Sections.Expenses = convertBASSection(col.Sections.Expenses)

	return basexport.BASColumn{
		Quarter: basexport.BASQuarterInfo{
			ID:           col.Quarter.ID,
			Name:         col.Quarter.Name,
			StartDate:    col.Quarter.StartDate,
			EndDate:      col.Quarter.EndDate,
			DisplayRange: col.Quarter.DisplayRange,
		},
		Sections:      sections.Sections,
		NetGSTPayable: col.NetGSTPayable,
	}
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

		if r.AccountType != nil && strings.ToUpper(*r.AccountType) == "REVENUE" {
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
