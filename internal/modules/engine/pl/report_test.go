package pl

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// PROFIT & LOSS (P&L) REPORT
// ============================================================================

func TestParseAndValidatePLErrors(t *testing.T) {
	invalid := "not-a-uuid"
	_, err := parseAndValidate(&PLFilter{ClinicID: invalid})
	require.Error(t, err)
}

func TestBuildPLReportBasic(t *testing.T) {
	rows := []*PLReportRow{
		{PLSection: "1. Income", CoaID: "c1", AccountName: "Sales", NetAmount: 100.0},
		{PLSection: "2. Cost of Sales", CoaID: "c2", AccountName: "COGS", NetAmount: -40.0},
	}
	summary := &PLSummaryRow{GrossProfitNet: 60.0, NetProfitNet: 60.0}
	f := &PLReportFilter{}

	rpt := buildReport(f, rows, summary)
	require.NotNil(t, rpt)
	require.Equal(t, 60.0, rpt.NetProfit)
}

// ============================================================================
// BAS PREPARATION REPORT
// ============================================================================

type BASFilter struct {
	ClinicID string
	FromDate time.Time
	ToDate   time.Time
}

type BASReport struct {
	G1_TotalSales   float64
	GstCollected_1A float64
	GstPaid_1B      float64
	NetGstPayable   float64
}

func buildBASReport(f *BASFilter, sales, gstCollected, gstPaid float64) *BASReport {
	return &BASReport{
		G1_TotalSales:   sales,
		GstCollected_1A: gstCollected,
		GstPaid_1B:      gstPaid,
		NetGstPayable:   gstCollected - gstPaid,
	}
}

func TestBuildBASReportBasic(t *testing.T) {
	filter := &BASFilter{
		ClinicID: uuid.NewString(),
		FromDate: time.Now().AddDate(0, -3, 0),
		ToDate:   time.Now(),
	}

	rpt := buildBASReport(filter, 11000.0, 1000.0, 450.0)

	require.NotNil(t, rpt)
	require.Equal(t, 11000.0, rpt.G1_TotalSales)
	require.Equal(t, 550.0, rpt.NetGstPayable)
}

// ============================================================================
// TRANSACTION  REPORT
// ============================================================================

type TransactionRow struct {
	TransactionID uuid.UUID
	Date          time.Time
	AccountName   string
	Debit         float64
	Credit        float64
}

type TransactionReport struct {
	TotalDebit  float64
	TotalCredit float64
	Rows        []*TransactionRow
}

func buildTransactionReport(rows []*TransactionRow) *TransactionReport {
	var totalDebit, totalCredit float64
	for _, row := range rows {
		totalDebit += row.Debit
		totalCredit += row.Credit
	}
	return &TransactionReport{
		TotalDebit:  totalDebit,
		TotalCredit: totalCredit,
		Rows:        rows,
	}
}

func TestBuildTransactionReportDoubleEntry(t *testing.T) {
	txID := uuid.New()
	rows := []*TransactionRow{
		{TransactionID: txID, Date: time.Now(), AccountName: "Bank Account", Debit: 150.0, Credit: 0.0},
		{TransactionID: txID, Date: time.Now(), AccountName: "Revenue Accounts", Debit: 0.0, Credit: 150.0},
	}

	rpt := buildTransactionReport(rows)

	require.NotNil(t, rpt)
	require.Len(t, rpt.Rows, 2)
	require.Equal(t, rpt.TotalDebit, rpt.TotalCredit, "Assets must balance completely with equity/liabilities allocations")
	require.Equal(t, 150.0, rpt.TotalDebit)
}

// ============================================================================
// ACTIVITY STATEMENT REPORT
// ============================================================================

type ActivityStatementReport struct {
	W1_TotalWithholding  float64
	W2_WithholdingAmount float64
	TotalPayable         float64
}

func buildActivityStatementReport(w1, w2 float64) *ActivityStatementReport {
	return &ActivityStatementReport{
		W1_TotalWithholding:  w1,
		W2_WithholdingAmount: w2,
		TotalPayable:         w2,
	}
}

func TestBuildActivityStatementPAYGCalculation(t *testing.T) {
	rpt := buildActivityStatementReport(50000.0, 7500.0)

	require.NotNil(t, rpt)
	require.Equal(t, 50000.0, rpt.W1_TotalWithholding)
	require.Equal(t, 7500.0, rpt.TotalPayable)
}

// ============================================================================
// BALANCE SHEET REPORT
// ============================================================================

type BalanceSheetRow struct {
	Category    string // "Asset", "Liability", "Equity"
	AccountName string
	Value       float64
}

type BalanceSheetReport struct {
	TotalAssets      float64
	TotalLiabilities float64
	TotalEquity      float64
}

func buildBalanceSheetReport(rows []*BalanceSheetRow) *BalanceSheetReport {
	rpt := &BalanceSheetReport{}
	for _, row := range rows {
		switch row.Category {
		case "Asset":
			rpt.TotalAssets += row.Value
		case "Liability":
			rpt.TotalLiabilities += row.Value
		case "Equity":
			rpt.TotalEquity += row.Value
		}
	}
	return rpt
}

func TestBuildBalanceSheetAccountingEquation(t *testing.T) {
	rows := []*BalanceSheetRow{
		{Category: "Asset", AccountName: "Operating Cash", Value: 12000.0},
		{Category: "Asset", AccountName: "Equipment Store", Value: 8000.0},
		{Category: "Liability", AccountName: "Accounts Payable", Value: 5000.0},
		{Category: "Equity", AccountName: "Retained Earnings", Value: 15000.0},
	}

	rpt := buildBalanceSheetReport(rows)

	require.NotNil(t, rpt)
	require.Equal(t, 20000.0, rpt.TotalAssets)

	// Assets = Liabilities + Equity
	calculatedOpposingSide := rpt.TotalLiabilities + rpt.TotalEquity
	require.Equal(t, rpt.TotalAssets, calculatedOpposingSide, "Balance sheet mismatch: Assets must equal Liabilities + Equity")
}
