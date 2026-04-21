package bas

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

var ErrClinicNotFound = errors.New("clinic not found")

type BASCategory string

const (
	BASCategoryTaxable     BASCategory = "TAXABLE"
	BASCategoryGSTFree     BASCategory = "GST_FREE"
	BASCategoryBASExcluded BASCategory = "BAS_EXCLUDED"
)

// BASSummaryRow maps one row of vw_bas_summary (quarterly).
type BASSummaryRow struct {
	ClinicID       uuid.UUID `db:"clinic_id"`
	PractitionerID uuid.UUID `db:"practitioner_id"`
	PeriodQuarter  time.Time `db:"period_quarter"`
	PeriodYear     time.Time `db:"period_year"`

	// Sales
	G1TotalSalesGross float64 `db:"g1_total_sales_gross"`
	G3GSTFreeSales    float64 `db:"g3_gst_free_sales"`
	G8TaxableSales    float64 `db:"g8_taxable_sales"`
	Label1AGSTOnSales float64 `db:"label_1a_gst_on_sales"`

	// Purchases
	G11TotalPurchasesGross float64 `db:"g11_total_purchases_gross"`
	G14GSTFreePurchases    float64 `db:"g14_gst_free_purchases"`
	G15TaxablePurchases    float64 `db:"g15_taxable_purchases"`
	Label1BGSTOnPurchases  float64 `db:"label_1b_gst_on_purchases"`

	// Net
	NetGSTPayable     float64 `db:"net_gst_payable"`
	TotalSalesNet     float64 `db:"total_sales_net"`
	TotalPurchasesNet float64 `db:"total_purchases_net"`
}

// BASByAccountRow maps one row of vw_bas_by_account.
type BASByAccountRow struct {
	ClinicID       uuid.UUID `db:"clinic_id"`
	PractitionerID uuid.UUID `db:"practitioner_id"`
	PeriodQuarter  time.Time `db:"period_quarter"`
	PeriodYear     time.Time `db:"period_year"`
	SectionType    string    `db:"section_type"`
	BASCategory    string    `db:"bas_category"`
	AccountCode    int16     `db:"account_code"`
	AccountName    string    `db:"account_name"`
	TaxName        string    `db:"tax_name"`
	TaxRate        float64   `db:"tax_rate"`
	EntryCount     int64     `db:"entry_count"`
	TotalNet       float64   `db:"total_net"`
	TotalGST       float64   `db:"total_gst"`
	TotalGross     float64   `db:"total_gross"`
}

// BASMonthlyRow maps one row of vw_bas_monthly.
type BASMonthlyRow struct {
	ClinicID       uuid.UUID `db:"clinic_id"`
	PractitionerID uuid.UUID `db:"practitioner_id"`
	PeriodMonth    time.Time `db:"period_month"`

	G1TotalSalesGross      float64 `db:"g1_total_sales_gross"`
	G3GSTFreeSales         float64 `db:"g3_gst_free_sales"`
	Label1AGSTOnSales      float64 `db:"label_1a_gst_on_sales"`
	G11TotalPurchasesGross float64 `db:"g11_total_purchases_gross"`
	G14GSTFreePurchases    float64 `db:"g14_gst_free_purchases"`
	Label1BGSTOnPurchases  float64 `db:"label_1b_gst_on_purchases"`
	NetGSTPayable          float64 `db:"net_gst_payable"`
	TotalSalesNet          float64 `db:"total_sales_net"`
	TotalPurchasesNet      float64 `db:"total_purchases_net"`
}

type BASFilter struct {
	ClinicIds       *string `form:"clinic_ids"`
	FromDate        *string `form:"from_date"`         // YYYY-MM-DD
	ToDate          *string `form:"to_date"`           // YYYY-MM-DD
	FinancialYearID *string `form:"financial_year_id"` // UUID — maps quarter to FY
	QuarterIDs      *string `form:"quarter_ids"`       // UUIDs of tbl_financial_quarter

	ParsedQuarterIDs []uuid.UUID
	ParsedClinicIDs  []uuid.UUID
}

// MapToFilter returns the BASFilter as a common.Filter, omitting nil values.
func (f *BASFilter) MapToFilter() common.Filter {
	filters := map[string]interface{}{}
	operators := map[string]common.Operator{}

	if f.QuarterIDs != nil && *f.QuarterIDs != "" {
		f.ParsedQuarterIDs = nil
		idStrings := strings.Split(*f.QuarterIDs, ",")
		for _, s := range idStrings {
			parsedID, err := uuid.Parse(strings.TrimSpace(s))
			if err == nil {
				f.ParsedQuarterIDs = append(f.ParsedQuarterIDs, parsedID)
			}
		}

		if len(f.ParsedQuarterIDs) > 0 {
			filters["quarter_ids"] = f.ParsedQuarterIDs
		}
	}

	if f.ClinicIds != nil && *f.ClinicIds != "" {
		f.ParsedClinicIDs = nil
		cStrings := strings.Split(*f.ClinicIds, ",")
		for _, s := range cStrings {
			if parsedID, err := uuid.Parse(strings.TrimSpace(s)); err == nil {
				f.ParsedClinicIDs = append(f.ParsedClinicIDs, parsedID)
			}
		}
		if len(f.ParsedClinicIDs) > 0 {
			filters["clinic_ids"] = f.ParsedClinicIDs
		}
	}

	if f.FromDate != nil {
		filters["from_date"] = *f.FromDate
	}
	if f.ToDate != nil {
		filters["to_date"] = *f.ToDate
	}
	if f.FinancialYearID != nil {
		filters["financial_year_id"] = *f.FinancialYearID
	}

	return common.NewFilter(
		nil,
		filters,
		operators,
		nil,
		nil,
		nil,
		nil,
	)
}

// BASReportFilter is used by the /bas/report endpoint.
type BASReportFilter struct {
	PractitionerID string  `form:"-"`          // set from JWT
	QuarterID      *string `form:"quarter_id"` // UUID of tbl_financial_quarter
	Month          *string `form:"month"`      // e.g. "January"
}

type BASExportFilter struct {
	
	PractitionerID  string   `form:"-"` // set from JWT
	FinancialYearID *string  `form:"financial_year_id" binding:"required"`
	QuarterIDs      []string `form:"quarter_id" binding:"required"`
	Month           *string  `form:"month"`
}

// RsBASReport is the flat totals response for /bas/report.
type RsBASReport struct {
	G1  float64 `json:"G1"`
	G11 float64 `json:"G11"`
	A1  float64 `json:"1A"`
	B1  float64 `json:"1B"`
}

// BASReportRow is the DB scan target for the report query.
// G1/G11 are net (ex-GST) amounts; 1A/1B are the GST collected/paid.
type BASReportRow struct {
	G1TotalSalesGross      float64 `db:"g1_total_sales_gross"`
	Label1AGSTOnSales      float64 `db:"label_1a_gst_on_sales"`
	G11TotalPurchasesGross float64 `db:"g11_total_purchases_gross"`
	Label1BGSTOnPurchases  float64 `db:"label_1b_gst_on_purchases"`
}

type RsBASSummary struct {
	// Period
	PeriodQuarter string `json:"period_quarter"` // e.g. "2026-01-01"
	PeriodYear    string `json:"period_year"`    // e.g. "2026-01-01"

	// Sales (ATO labels)
	G1TotalSalesGross float64 `json:"g1_total_sales_gross"`
	G3GSTFreeSales    float64 `json:"g3_gst_free_sales"`
	G8TaxableSales    float64 `json:"g8_taxable_sales"`
	Label1AGSTOnSales float64 `json:"label_1a_gst_on_sales"`

	// Purchases (ATO labels)
	G11TotalPurchasesGross float64 `json:"g11_total_purchases_gross"`
	G14GSTFreePurchases    float64 `json:"g14_gst_free_purchases"`
	G15TaxablePurchases    float64 `json:"g15_taxable_purchases"`
	Label1BGSTOnPurchases  float64 `json:"label_1b_gst_on_purchases"`

	// Net GST
	NetGSTPayable     float64 `json:"net_gst_payable"`
	TotalSalesNet     float64 `json:"total_sales_net"`
	TotalPurchasesNet float64 `json:"total_purchases_net"`
}

func (r *BASSummaryRow) ToRs() RsBASSummary {
	return RsBASSummary{
		PeriodQuarter:          r.PeriodQuarter.Format("2006-01-02"),
		PeriodYear:             r.PeriodYear.Format("2006-01-02"),
		G1TotalSalesGross:      r.G1TotalSalesGross,
		G3GSTFreeSales:         r.G3GSTFreeSales,
		G8TaxableSales:         r.G8TaxableSales,
		Label1AGSTOnSales:      r.Label1AGSTOnSales,
		G11TotalPurchasesGross: r.G11TotalPurchasesGross,
		G14GSTFreePurchases:    r.G14GSTFreePurchases,
		G15TaxablePurchases:    r.G15TaxablePurchases,
		Label1BGSTOnPurchases:  r.Label1BGSTOnPurchases,
		NetGSTPayable:          r.NetGSTPayable,
		TotalSalesNet:          r.TotalSalesNet,
		TotalPurchasesNet:      r.TotalPurchasesNet,
	}
}

type RsBASByAccount struct {
	PeriodQuarter string  `json:"period_quarter"`
	PeriodYear    string  `json:"period_year"`
	SectionType   string  `json:"section_type"`
	BASCategory   string  `json:"bas_category"`
	AccountCode   int16   `json:"account_code"`
	AccountName   string  `json:"account_name"`
	TaxName       string  `json:"tax_name"`
	TaxRate       float64 `json:"tax_rate"`
	EntryCount    int64   `json:"entry_count"`
	TotalNet      float64 `json:"total_net"`
	TotalGST      float64 `json:"total_gst"`
	TotalGross    float64 `json:"total_gross"`
}

func (r *BASByAccountRow) ToRs() RsBASByAccount {
	return RsBASByAccount{
		PeriodQuarter: r.PeriodQuarter.Format("2006-01-02"),
		PeriodYear:    r.PeriodYear.Format("2006-01-02"),
		SectionType:   r.SectionType,
		BASCategory:   r.BASCategory,
		AccountCode:   r.AccountCode,
		AccountName:   r.AccountName,
		TaxName:       r.TaxName,
		TaxRate:       r.TaxRate,
		EntryCount:    r.EntryCount,
		TotalNet:      r.TotalNet,
		TotalGST:      r.TotalGST,
		TotalGross:    r.TotalGross,
	}
}

type RsBASMonthly struct {
	PeriodMonth            string  `json:"period_month"`
	G1TotalSalesGross      float64 `json:"g1_total_sales_gross"`
	G3GSTFreeSales         float64 `json:"g3_gst_free_sales"`
	Label1AGSTOnSales      float64 `json:"label_1a_gst_on_sales"`
	G11TotalPurchasesGross float64 `json:"g11_total_purchases_gross"`
	G14GSTFreePurchases    float64 `json:"g14_gst_free_purchases"`
	Label1BGSTOnPurchases  float64 `json:"label_1b_gst_on_purchases"`
	NetGSTPayable          float64 `json:"net_gst_payable"`
	TotalSalesNet          float64 `json:"total_sales_net"`
	TotalPurchasesNet      float64 `json:"total_purchases_net"`
}

func (r *BASMonthlyRow) ToRs() RsBASMonthly {
	return RsBASMonthly{
		PeriodMonth:            r.PeriodMonth.Format("2006-01"),
		G1TotalSalesGross:      r.G1TotalSalesGross,
		G3GSTFreeSales:         r.G3GSTFreeSales,
		Label1AGSTOnSales:      r.Label1AGSTOnSales,
		G11TotalPurchasesGross: r.G11TotalPurchasesGross,
		G14GSTFreePurchases:    r.G14GSTFreePurchases,
		Label1BGSTOnPurchases:  r.Label1BGSTOnPurchases,
		NetGSTPayable:          r.NetGSTPayable,
		TotalSalesNet:          r.TotalSalesNet,
		TotalPurchasesNet:      r.TotalPurchasesNet,
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

const dateLayout = "2006-01-02"

func validateDateFilter(f *BASFilter) error {
	var from, to time.Time
	var err error
	if f.FromDate != nil {
		if from, err = time.Parse(dateLayout, *f.FromDate); err != nil {
			return fmt.Errorf("invalid from_date: use YYYY-MM-DD format")
		}
	}
	if f.ToDate != nil {
		if to, err = time.Parse(dateLayout, *f.ToDate); err != nil {
			return fmt.Errorf("invalid to_date: use YYYY-MM-DD format")
		}
	}
	if f.FromDate != nil && f.ToDate != nil && from.After(to) {
		return fmt.Errorf("from_date must not be after to_date")
	}
	return nil
}

// BAS Preparation
type BASAmount struct {
	Gross float64 `json:"gross"`
	GST   float64 `json:"gst"`
	Net   float64 `json:"net"`
}

type BASLineItem struct {
	Name    string    `json:"name"`
	Amounts BASAmount `json:"amounts"`
}

type BASGSTItem struct {
	Name        string  `json:"name"`
	GrossAmount float64 `json:"gross_amount"`
}

type BASSection struct {
	Items []BASLineItem `json:"items"`
}

type BASQuarterInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	StartDate    string `json:"startDate"`
	EndDate      string `json:"endDate"`
	DisplayRange string `json:"displayRange"`
}

type BASLineItemRow struct {
	PeriodQuarter time.Time `db:"period_quarter"`
	SectionType   *string   `db:"section_type"`
	BasCategory   string    `db:"bas_category"`
	AccountName   string    `db:"account_name"`
	NetAmount     float64   `db:"net_amount"`
	GstAmount     float64   `db:"gst_amount"`
	GrossAmount   float64   `db:"gross_amount"`
	CoaID         string    `db:"coa_id"`
}

type BASColumn struct {
	Quarter  BASQuarterInfo `json:"quarter"`
	Sections struct {
		Income        BASSection `json:"income"`
		Expenses      BASSection `json:"expenses"`
		NetProfitLoss BASSection `json:"net_profit_loss"`
	} `json:"sections"`
	NetGSTPayable float64 `json:"net_gst_payable"`
}

type RsBASPreparation struct {
	Columns    []BASColumn `json:"columns"`
	GrandTotal BASColumn   `json:"grand_total"`
}
