package bs

import (
	"github.com/google/uuid"
)

// BSRow represents a single account balance from the database
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

// BSFilter defines query parameters for balance sheet
type BSFilter struct {
	PractitionerID *string `form:"practitioner_id"`
	ClinicID       *string `form:"clinic_id"`
	AsOfDate       *string `form:"as_of_date"` // Show balances as of this date (YYYY-MM-DD)
}

// RsBalanceSheet is the complete balance sheet response
type RsBalanceSheet struct {
	AsOfDate                  string      `json:"as_of_date"`
	Assets                    []RsAccount `json:"assets"`
	TotalAssets               float64     `json:"total_assets"`
	Liabilities               []RsAccount `json:"liabilities"`
	TotalLiabilities          float64     `json:"total_liabilities"`
	Equity                    []RsAccount `json:"equity"`
	CurrentYearProfit         float64     `json:"current_year_profit"`
	TotalEquity               float64     `json:"total_equity"`
	TotalLiabilitiesAndEquity float64     `json:"total_liabilities_and_equity"`
}

// RsAccount represents a single account line in the balance sheet
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
