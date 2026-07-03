package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Setting represents template settings domain entity
type Setting struct {
	ID               uuid.UUID
	InvoiceID        *uuid.UUID
	PrimaryColor     string
	AccentColor      string
	BodyFontFamily   string
	HeaderFontFamily string
	IsLogo           bool
	LogoID           *uuid.UUID
	TermText         *string
	PaymentTerms     *string
	IsWaterMark      bool
	WaterMarkText    *string
	TableStyle       *string
	CreatedAt        time.Time
	UpdatedAt        *time.Time
	DeletedAt        *time.Time
}

// Validate checks if setting meets business rules
func (s *Setting) Validate() error {
	if s.PrimaryColor == "" {
		return errors.New("primary color is required")
	}
	if s.AccentColor == "" {
		return errors.New("accent color is required")
	}
	if s.BodyFontFamily == "" {
		return errors.New("body font family is required")
	}
	if s.HeaderFontFamily == "" {
		return errors.New("header font family is required")
	}
	return nil
}

// IsGlobal checks if this is a global setting (not invoice-specific)
func (s *Setting) IsGlobal() bool {
	return s.InvoiceID == nil
}

// IsDeleted checks if setting is soft-deleted
func (s *Setting) IsDeleted() bool {
	return s.DeletedAt != nil
}
