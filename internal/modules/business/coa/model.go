package coa

import "time"

type AccountType struct {
	ID          int16     `db:"id"`
	Name        string    `db:"name"`
	Description *string   `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type AccountTax struct {
	ID          int16     `db:"id"`
	Name        string    `db:"name"`
	Rate        float64   `db:"rate"`
	BASField    *string   `db:"bas_field"`
	IsTaxable   bool      `db:"is_taxable"`
	Description *string   `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

func (a *AccountTax) ToRs() AccountTax {
	return AccountTax{
		ID:          a.ID,
		Name:        a.Name,
		Rate:        a.Rate,
		BASField:    a.BASField,
		IsTaxable:   a.IsTaxable,
		Description: a.Description,
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
	}
}
func (a *AccountType) ToRs() AccountType {
	return AccountType{
		ID:          a.ID,
		Name:        a.Name,
		Description: a.Description,
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
	}
}
