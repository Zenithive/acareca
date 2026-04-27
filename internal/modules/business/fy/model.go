package fy

import (
	"time"

	"github.com/google/uuid"
)

// Database models
type FinancialYear struct {
	ID          uuid.UUID `db:"id"`
	Label       string    `db:"label"`
	IsActive    bool      `db:"is_active"`
	StartDate   time.Time `db:"start_date"`
	EndDate     time.Time `db:"end_date"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
	ExpenseDate time.Time `db:"expense_date"`
}

type FinancialQuarter struct {
	ID              uuid.UUID `db:"id"`
	FinancialYearID uuid.UUID `db:"financial_year_id"`
	Label           string    `db:"label"`
	StartDate       time.Time `db:"start_date"`
	EndDate         time.Time `db:"end_date"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}

// Request models
type RqCreateFY struct {
	Label    string `json:"label" validate:"required"`
	FYYear   string `json:"fy_year" validate:"required"`
	IsActive bool   `json:"is_active"`
}

type RqUpdateFYLabel struct {
	Label    *string `json:"label"`
	IsActive *bool   `json:"is_active"`
}

// Response models
type RsFinancialYear struct {
	ID        uuid.UUID `json:"id"`
	Label     string    `json:"label"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
}

type RsFinancialQuarter struct {
	ID        uuid.UUID `json:"id"`
	Label     string    `json:"label"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
}
