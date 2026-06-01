package bs

import (
	"github.com/google/uuid"
)

type BSRow struct {
	PractitionerID      uuid.UUID `db:"practitioner_id"`
	ClinicID            uuid.UUID `db:"clinic_id"`
	AccountType         string    `db:"account_type"`
	AccountCode         int16     `db:"account_code"`
	AccountName         string    `db:"account_name"`
	CoaID               uuid.UUID `db:"coa_id"`
	Balance             float64   `db:"balance"`
	EntryCount          int       `db:"entry_count"`
	LastTransactionDate string    `db:"last_transaction_date"`
}

type BSFilter struct {
	PractitionerID *string `form:"practitioner_id"`
	EndDate        *string `form:"end_date"`
}

type RsBalanceSheet struct {
	EndDate           string      `json:"end_date,omitempty"`
	Assets            []RsAccount `json:"assets"`
	TotalAssets       float64     `json:"total_assets"`
	Liabilities       []RsAccount `json:"liabilities"`
	TotalLiabilities  float64     `json:"total_liabilities"`
	NetAssets         float64     `json:"net_assets"`
	Equity            []RsAccount `json:"equity"`
	CurrentYearProfit float64     `json:"current_year_profit"`
	TotalEquity       float64     `json:"total_equity"`
}

type RsAccount struct {
	CoaId   uuid.UUID `json:"coa_id"`
	Code    int16     `json:"code"`
	Name    string    `json:"name"`
	Balance float64   `json:"balance"`
}

func (r *BSRow) ToRs() RsAccount {
	return RsAccount{
		CoaId:   r.CoaID,
		Code:    r.AccountCode,
		Name:    r.AccountName,
		Balance: r.Balance,
	}
}

type ExportBalanceSheetResponse struct {
	Result interface{}
}
