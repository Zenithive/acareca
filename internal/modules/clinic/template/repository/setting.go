package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/jmoiron/sqlx"
)

type ISettingRepository interface {
	Get(ctx context.Context, invoiceId uuid.UUID) (*common.Setting, error)
	Create(ctx context.Context, st *common.Setting) error
	Update(ctx context.Context, st *common.Setting, invoiceId uuid.UUID) error
}

type SettingRepository struct {
	db *sqlx.DB
}

func NewSettingRepository(db *sqlx.DB) ISettingRepository {
	return &SettingRepository{db: db}
}

// Get retrieves settings for an invoice (or global if not found)
func (r *SettingRepository) Get(ctx context.Context, invoiceId uuid.UUID) (*common.Setting, error) {
	var setting common.Setting

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

		err := r.db.GetContext(ctx, &setting, qSpecific, invoiceId)
		if err == nil {
			return &setting, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("failed fetching invoice setting: %w", err)
		}
	}

	const qDefault = `
		SELECT 
			id, invoice_id, primary_color, accent_color, body_font_family, header_font_family, 
			is_logo, logo_id, terms_text, payment_terms, is_watermark, watermark_text, table_style, 
			created_at, updated_at, deleted_at
		FROM tbl_template_setting 
		WHERE invoice_id IS NULL 
		  AND deleted_at IS NULL 
		LIMIT 1`

	if err := r.db.GetContext(ctx, &setting, qDefault); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed fetching global settings: %w", err)
	}

	return &setting, nil
}

// Create inserts a new setting
func (r *SettingRepository) Create(ctx context.Context, st *common.Setting) error {
	const q = `
		INSERT INTO tbl_template_setting (
			id, invoice_id, primary_color, accent_color, body_font_family, header_font_family,
			is_logo, logo_id, terms_text, payment_terms, is_watermark, watermark_text, table_style
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		)
		RETURNING created_at`

	err := r.db.QueryRowContext(ctx, q,
		st.Id,
		st.InvoiceId,
		st.PrimaryColor,
		st.AccentColor,
		st.BodyFontFamily,
		st.HeaderFontFamily,
		st.IsLogo,
		st.LogoId,
		st.TermText,
		st.PaymentTerms,
		st.IsWaterMark,
		st.WaterMarkText,
		st.TableStyle,
	).Scan(&st.CreatedAt)

	return err
}

// Update modifies an existing setting or inserts if not exists
func (r *SettingRepository) Update(ctx context.Context, st *common.Setting, invoiceId uuid.UUID) error {
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

	if invoiceId != uuid.Nil {
		st.InvoiceId = &invoiceId
	}

	var createdAt, updatedAt string
	err := r.db.QueryRowContext(ctx, q,
		st.Id,
		st.InvoiceId,
		st.PrimaryColor,
		st.AccentColor,
		st.BodyFontFamily,
		st.HeaderFontFamily,
		st.IsLogo,
		st.LogoId,
		st.TermText,
		st.PaymentTerms,
		st.IsWaterMark,
		st.WaterMarkText,
		st.TableStyle,
	).Scan(&st.Id, &createdAt, &updatedAt)

	if err != nil {
		return err
	}

	if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
		st.CreatedAt = t
	}
	if updatedAt != "" {
		if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			st.UpdatedAt = &t
		}
	}

	return nil
}
