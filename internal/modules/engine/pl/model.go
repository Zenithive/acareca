package pl

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrClinicNotFound = errors.New("clinic not found")

type PLSummaryRow struct {
	PractitionerID uuid.UUID `db:"practitioner_id"`
	PeriodMonth    time.Time `db:"period_month"`
	IncomeNet      float64   `db:"income_net"`
	IncomeGST      float64   `db:"income_gst"`
	IncomeGross    float64   `db:"income_gross"`
	CogsNet        float64   `db:"cogs_net"`
	CogsGST        float64   `db:"cogs_gst"`
	CogsGross      float64   `db:"cogs_gross"`
	GrossProfitNet float64   `db:"gross_profit_net"`
	OtherExpNet    float64   `db:"other_expenses_net"`
	OtherExpGST    float64   `db:"other_expenses_gst"`
	OtherExpGross  float64   `db:"other_expenses_gross"`
	NetProfitNet   float64   `db:"net_profit_net"`
	NetProfitGross float64   `db:"net_profit_gross"`
}

type PLAccountRow struct {
	PractitionerID uuid.UUID `db:"practitioner_id"`
	PeriodMonth    time.Time `db:"period_month"`
	PLSection      string    `db:"pl_section"`
	SectionType    string    `db:"section_type"`
	AccountCode    int16     `db:"account_code"`
	AccountName    string    `db:"account_name"`
	AccountType    string    `db:"account_type"`
	TaxName        string    `db:"tax_name"`
	TaxRate        float64   `db:"tax_rate"`
	TotalNet       float64   `db:"total_net"`
	TotalGST       float64   `db:"total_gst"`
	TotalGross     float64   `db:"total_gross"`
	SignedNet      float64   `db:"signed_net"`
	SignedGross    float64   `db:"signed_gross"`
	EntryCount     int64     `db:"entry_count"`
}

type PLResponsibilityRow struct {
	PractitionerID        uuid.UUID `db:"practitioner_id"`
	PeriodMonth           time.Time `db:"period_month"`
	PaymentResponsibility string    `db:"payment_responsibility"`
	SectionType           string    `db:"section_type"`
	PLSection             string    `db:"pl_section"`
	AccountCode           int16     `db:"account_code"`
	AccountName           string    `db:"account_name"`
	TotalNet              float64   `db:"total_net"`
	TotalGST              float64   `db:"total_gst"`
	TotalGross            float64   `db:"total_gross"`
	EntryCount            int64     `db:"entry_count"`
}

type PLFYSummaryRow struct {
	PractitionerID     uuid.UUID `db:"practitioner_id"`
	FinancialYearID    uuid.UUID `db:"financial_year_id"`
	FinancialYear      string    `db:"financial_year"`
	FinancialQuarterID uuid.UUID `db:"financial_quarter_id"`
	Quarter            string    `db:"quarter"`
	IncomeNet          float64   `db:"income_net"`
	IncomeGST          float64   `db:"income_gst"`
	IncomeGross        float64   `db:"income_gross"`
	CogsNet            float64   `db:"cogs_net"`
	CogsGST            float64   `db:"cogs_gst"`
	CogsGross          float64   `db:"cogs_gross"`
	GrossProfitNet     float64   `db:"gross_profit_net"`
	OtherExpensesNet   float64   `db:"other_expenses_net"`
	NetProfitNet       float64   `db:"net_profit_net"`
	NetProfitGross     float64   `db:"net_profit_gross"`
}

type PLFilter struct {
	ClinicID        string  `form:"clinic_id" validate:"uuid"`
	FromDate        *string `form:"from_date"`
	ToDate          *string `form:"to_date"`
	FinancialYearID *string `form:"financial_year_id"`
}

type RsPLSummary struct {
	PeriodMonth    string  `json:"period_month"`
	IncomeNet      float64 `json:"income_net"`
	IncomeGST      float64 `json:"income_gst"`
	IncomeGross    float64 `json:"income_gross"`
	CogsNet        float64 `json:"cogs_net"`
	CogsGST        float64 `json:"cogs_gst"`
	CogsGross      float64 `json:"cogs_gross"`
	GrossProfitNet float64 `json:"gross_profit_net"`
	OtherExpNet    float64 `json:"other_expenses_net"`
	OtherExpGST    float64 `json:"other_expenses_gst"`
	OtherExpGross  float64 `json:"other_expenses_gross"`
	NetProfitNet   float64 `json:"net_profit_net"`
	NetProfitGross float64 `json:"net_profit_gross"`
}

func (r *PLSummaryRow) ToRs() RsPLSummary {
	return RsPLSummary{
		PeriodMonth:    r.PeriodMonth.Format("2006-01"),
		IncomeNet:      r.IncomeNet,
		IncomeGST:      r.IncomeGST,
		IncomeGross:    r.IncomeGross,
		CogsNet:        r.CogsNet,
		CogsGST:        r.CogsGST,
		CogsGross:      r.CogsGross,
		GrossProfitNet: r.GrossProfitNet,
		OtherExpNet:    r.OtherExpNet,
		OtherExpGST:    r.OtherExpGST,
		OtherExpGross:  r.OtherExpGross,
		NetProfitNet:   r.NetProfitNet,
		NetProfitGross: r.NetProfitGross,
	}
}

type RsPLAccount struct {
	PeriodMonth string  `json:"period_month"`
	PLSection   string  `json:"pl_section"`
	SectionType string  `json:"section_type"`
	AccountCode int16   `json:"account_code"`
	AccountName string  `json:"account_name"`
	AccountType string  `json:"account_type"`
	TaxName     string  `json:"tax_name"`
	TaxRate     float64 `json:"tax_rate"`
	TotalNet    float64 `json:"total_net"`
	TotalGST    float64 `json:"total_gst"`
	TotalGross  float64 `json:"total_gross"`
	SignedNet   float64 `json:"signed_net"`
	SignedGross float64 `json:"signed_gross"`
	EntryCount  int64   `json:"entry_count"`
}

func (r *PLAccountRow) ToRs() RsPLAccount {
	return RsPLAccount{
		PeriodMonth: r.PeriodMonth.Format("2006-01"),
		PLSection:   r.PLSection,
		SectionType: r.SectionType,
		AccountCode: r.AccountCode,
		AccountName: r.AccountName,
		AccountType: r.AccountType,
		TaxName:     r.TaxName,
		TaxRate:     r.TaxRate,
		TotalNet:    r.TotalNet,
		TotalGST:    r.TotalGST,
		TotalGross:  r.TotalGross,
		SignedNet:   r.SignedNet,
		SignedGross: r.SignedGross,
		EntryCount:  r.EntryCount,
	}
}

type RsPLResponsibility struct {
	PeriodMonth           string  `json:"period_month"`
	PaymentResponsibility string  `json:"payment_responsibility"`
	SectionType           string  `json:"section_type"`
	PLSection             string  `json:"pl_section"`
	AccountCode           int16   `json:"account_code"`
	AccountName           string  `json:"account_name"`
	TotalNet              float64 `json:"total_net"`
	TotalGST              float64 `json:"total_gst"`
	TotalGross            float64 `json:"total_gross"`
	EntryCount            int64   `json:"entry_count"`
}

func (r *PLResponsibilityRow) ToRs() RsPLResponsibility {
	return RsPLResponsibility{
		PeriodMonth:           r.PeriodMonth.Format("2006-01"),
		PaymentResponsibility: r.PaymentResponsibility,
		SectionType:           r.SectionType,
		PLSection:             r.PLSection,
		AccountCode:           r.AccountCode,
		AccountName:           r.AccountName,
		TotalNet:              r.TotalNet,
		TotalGST:              r.TotalGST,
		TotalGross:            r.TotalGross,
		EntryCount:            r.EntryCount,
	}
}

type RsPLFYSummary struct {
	FinancialYearID    uuid.UUID `json:"financial_year_id"`
	FinancialYear      string    `json:"financial_year"`
	FinancialQuarterID uuid.UUID `json:"financial_quarter_id"`
	Quarter            string    `json:"quarter"`
	IncomeNet          float64   `json:"income_net"`
	IncomeGST          float64   `json:"income_gst"`
	IncomeGross        float64   `json:"income_gross"`
	CogsNet            float64   `json:"cogs_net"`
	CogsGST            float64   `json:"cogs_gst"`
	CogsGross          float64   `json:"cogs_gross"`
	GrossProfitNet     float64   `json:"gross_profit_net"`
	OtherExpensesNet   float64   `json:"other_expenses_net"`
	NetProfitNet       float64   `json:"net_profit_net"`
	NetProfitGross     float64   `json:"net_profit_gross"`
}

func (r *PLFYSummaryRow) ToRs() RsPLFYSummary {
	return RsPLFYSummary{
		FinancialYearID:    r.FinancialYearID,
		FinancialYear:      r.FinancialYear,
		FinancialQuarterID: r.FinancialQuarterID,
		Quarter:            r.Quarter,
		IncomeNet:          r.IncomeNet,
		IncomeGST:          r.IncomeGST,
		IncomeGross:        r.IncomeGross,
		CogsNet:            r.CogsNet,
		CogsGST:            r.CogsGST,
		CogsGross:          r.CogsGross,
		GrossProfitNet:     r.GrossProfitNet,
		OtherExpensesNet:   r.OtherExpensesNet,
		NetProfitNet:       r.NetProfitNet,
		NetProfitGross:     r.NetProfitGross,
	}
}
