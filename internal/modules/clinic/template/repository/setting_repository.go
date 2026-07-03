package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template/domain"
	"github.com/jmoiron/sqlx"
)

// ISettingRepository handles template settings persistence
type ISettingRepository interface {
	Get(ctx context.Context, invoiceId uuid.UUID) (*domain.Setting, error)
	Create(ctx context.Context, st *domain.Setting) error
	Update(ctx context.Context, st *domain.Setting, invoiceId uuid.UUID) error
}

// SettingRepository implements setting data access
type SettingRepository struct {
	db *sqlx.DB
}

// NewSettingRepository creates a new setting repository
func NewSettingRepository(db *sqlx.DB) ISettingRepository {
	return &SettingRepository{db: db}
}

// dbSetting is the database model for settings
type dbSetting struct {
	ID               uuid.UUID  `db:"id"`
	InvoiceID        *uuid.UUID `db:"invoice_id"`
	PrimaryColor     string     `db:"primary_color"`
	AccentColor      string     `db:"accent_color"`
	BodyFontFamily   string     `db:"body_font_family"`
	HeaderFontFamily string     `db:"header_font_family"`
	IsLogo           bool       `db:"is_logo"`
	LogoID           *uuid.UUID `db:"logo_id"`
	TermText         *string    `db:"terms_text"`
	PaymentTerms     *string    `db:"payment_terms"`
	IsWaterMark      bool       `db:"is_watermark"`
	WaterMarkText    *string    `db:"watermark_text"`
	TableStyle       *string    `db:"table_style"`
	CreatedAt        string     `db:"created_at"`
	UpdatedAt        *string    `db:"updated_at"`
	DeletedAt        *string    `db:"deleted_at"`
}

// Get retrieves settings for an invoice with fallback to global defaults
func (r *SettingRepository) Get(ctx context.Context, invoiceId uuid.UUID) (*domain.Setting, error) {
	var db dbSetting

	// Try to get invoice-specific settings first
	if invoiceId != uuid.Nil {
		const qSpecific = `
			SELECT 
				id, invoice_id, primary_color, accent_color, body_font_family, header_font_family, 
				is_logo, logo_id, terms_text, payment_terms, is_watermark, watermark_text, table_style, 
				created_at, updated_at, deleted_at
			FROM tbl_template_setting 
			WHERE invoice_id = $1 
			  AND deleted_at IS NULL 
			LIMIT 1`

		err := r.db.GetContext(ctx, &db, qSpecific, invoiceId)
		if err == nil {
			return r.toDomain(&db), nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("failed fetching invoice setting: %w", err)
		}
	}

	// Fallback to global defaults
	const qDefault = `
		SELECT 
			id, invoice_id, primary_color, accent_color, body_font_family, header_font_family, 
			is_logo, logo_id, terms_text, payment_terms, is_watermark, watermark_text, table_style, 
			created_at, updated_at, deleted_at
		FROM tbl_template_setting 
		WHERE invoice_id IS NULL 
		  AND deleted_at IS NULL 
		LIMIT 1`

	if err := r.db.GetContext(ctx, &db, qDefault); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No settings found
		}
		return nil, fmt.Errorf("failed fetching global settings: %w", err)
	}

	return r.toDomain(&db), nil
}

// Create inserts a new setting record
func (r *SettingRepository) Create(ctx context.Context, st *domain.Setting) error {
	const q = `
		INSERT INTO tbl_template_setting (
			id, invoice_id, primary_color, accent_color, body_font_family, header_font_family,
			is_logo, logo_id, terms_text, payment_terms, is_watermark, watermark_text, table_style
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		)
		RETURNING created_at`

	err := r.db.QueryRowContext(ctx, q,
		st.ID,
		st.InvoiceID,
		st.PrimaryColor,
		st.AccentColor,
		st.BodyFontFamily,
		st.HeaderFontFamily,
		st.IsLogo,
		st.LogoID,
		st.TermText,
		st.PaymentTerms,
		st.IsWaterMark,
		st.WaterMarkText,
		st.TableStyle,
	).Scan(&st.CreatedAt)

	return err
}

// Update modifies or inserts a setting (UPSERT)
func (r *SettingRepository) Update(ctx context.Context, st *domain.Setting, invoiceId uuid.UUID) error {
	const q = `
		INSERT INTO tbl_template_setting (
			id, invoice_id, primary_color, accent_color, body_font_family, header_font_family,
			is_logo, logo_id, terms_text, payment_terms, is_watermark, watermark_text, table_style
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		)
		ON CONFLICT (id) DO UPDATE SET
			invoice_id         = EXCLUDED.invoice_id,
			primary_color      = EXCLUDED.primary_color,
			accent_color       = EXCLUDED.accent_color,
			body_font_family   = EXCLUDED.body_font_family,
			header_font_family = EXCLUDED.header_font_family,
			is_logo            = EXCLUDED.is_logo,
			logo_id            = EXCLUDED.logo_id,
			terms_text         = EXCLUDED.terms_text,
			payment_terms      = EXCLUDED.payment_terms,
			is_watermark       = EXCLUDED.is_watermark,
			watermark_text     = EXCLUDED.watermark_text,
			table_style        = EXCLUDED.table_style,
			updated_at         = NOW()
		RETURNING id, created_at, updated_at`

	// Set the invoice_id in the setting if provided
	if invoiceId != uuid.Nil {
		st.InvoiceID = &invoiceId
	}

	var createdAt, updatedAt string
	err := r.db.QueryRowContext(ctx, q,
		st.ID,
		st.InvoiceID,
		st.PrimaryColor,
		st.AccentColor,
		st.BodyFontFamily,
		st.HeaderFontFamily,
		st.IsLogo,
		st.LogoID,
		st.TermText,
		st.PaymentTerms,
		st.IsWaterMark,
		st.WaterMarkText,
		st.TableStyle,
	).Scan(&st.ID, &createdAt, &updatedAt)
	
	if err != nil {
		return err
	}

	// Parse timestamps
	if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
		st.CreatedAt = t
	}
	if updatedAt != "" {
		if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			st.UpdatedAt = &t
		}
	}

	return err
}

// toDomain converts database model to domain entity
func (r *SettingRepository) toDomain(db *dbSetting) *domain.Setting {
	setting := &domain.Setting{
		ID:               db.ID,
		InvoiceID:        db.InvoiceID,
		PrimaryColor:     db.PrimaryColor,
		AccentColor:      db.AccentColor,
		BodyFontFamily:   db.BodyFontFamily,
		HeaderFontFamily: db.HeaderFontFamily,
		IsLogo:           db.IsLogo,
		LogoID:           db.LogoID,
		TermText:         db.TermText,
		PaymentTerms:     db.PaymentTerms,
		IsWaterMark:      db.IsWaterMark,
		WaterMarkText:    db.WaterMarkText,
		TableStyle:       db.TableStyle,
	}
	
	// Parse timestamps
	if t, err := time.Parse(time.RFC3339, db.CreatedAt); err == nil {
		setting.CreatedAt = t
	}
	if db.UpdatedAt != nil {
		if t, err := time.Parse(time.RFC3339, *db.UpdatedAt); err == nil {
			setting.UpdatedAt = &t
		}
	}
	if db.DeletedAt != nil {
		if t, err := time.Parse(time.RFC3339, *db.DeletedAt); err == nil {
			setting.DeletedAt = &t
		}
	}
	
	return setting
}
