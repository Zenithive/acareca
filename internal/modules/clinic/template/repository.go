package template

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/file"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("template not found")

type IRepository interface {
	Create(ctx context.Context, t *Template) error
	BulkCreate(ctx context.Context, t []Template) error
	Update(ctx context.Context, t *Template) error
	Delete(ctx context.Context, clinicId uuid.UUID, id uuid.UUID) error
	Get(ctx context.Context, clinicId uuid.UUID, id uuid.UUID) (*Template, error)
	List(ctx context.Context) (*util.RsList, error)
	GetSetting(ctx context.Context, templateId uuid.UUID) (*Setting, error)
	UpdateSetting(ctx context.Context, st *Setting) error
	CreateSetting(ctx context.Context, st *Setting) error
	GetDocumentByID(ctx context.Context, id uuid.UUID) (*file.Document, error)
}

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) IRepository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, t *Template) error {
	const q = `
		INSERT INTO tbl_template (clinic_id, name, description, html, css, is_default, is_active)
		VALUES (:clinic_id, :name, :description, :html, :css, :is_default, :is_active)
		RETURNING id, created_at`
	rows, err := r.db.NamedQueryContext(ctx, q, t)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return rows.StructScan(t)
	}
	return nil
}

func (r *Repository) Update(ctx context.Context, t *Template) error {
	const q = `
		UPDATE tbl_template
		SET name = :name, description = :description, html = :html, css = :css, updated_at = NOW()
		WHERE id = :id AND clinic_id = :clinic_id AND deleted_at IS NULL`
	_, err := r.db.NamedExecContext(ctx, q, t)
	return err
}

func (r *Repository) Delete(ctx context.Context, clinicId uuid.UUID, id uuid.UUID) error {
	const q = `UPDATE tbl_template SET deleted_at = NOW() WHERE id = $1 AND clinic_id = $2 AND deleted_at IS NULL`
	_, err := r.db.ExecContext(ctx, q, id, clinicId)
	return err
}

func (r *Repository) Get(ctx context.Context, clinicId uuid.UUID, id uuid.UUID) (*Template, error) {
	const q = `SELECT * FROM tbl_template WHERE id = $1 AND clinic_id = $2 AND deleted_at IS NULL`
	var t Template
	if err := r.db.GetContext(ctx, &t, q, id, clinicId); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *Repository) List(ctx context.Context) (*util.RsList, error) {
	const q = `SELECT * FROM tbl_template WHERE deleted_at IS NULL ORDER BY created_at DESC`
	var items []Template
	if err := r.db.SelectContext(ctx, &items, q); err != nil {
		return nil, err
	}

	rs := make([]RsTemplate, len(items))
	for i, t := range items {
		rsView := t.ToRs()
		// Encode raw internal binary arrays to Base64 text streams
		rsView.Html = base64.StdEncoding.EncodeToString(t.Html)
		rsView.Css = base64.StdEncoding.EncodeToString(t.Css)
		rs[i] = rsView
	}
	return &util.RsList{Items: rs, Total: len(rs)}, nil
}

func (r *Repository) GetSetting(ctx context.Context, templateId uuid.UUID) (*Setting, error) {
	const q = `SELECT * FROM tbl_template_setting WHERE template_id = $1 AND deleted_at IS NULL`
	var st Setting
	if err := r.db.GetContext(ctx, &st, q, templateId); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &st, nil
}

func (r *Repository) UpdateSetting(ctx context.Context, st *Setting) error {
	const q = `
		INSERT INTO tbl_template_setting (
			template_id, primary_color, accent_color, body_font_family, header_font_family,
			is_logo, logo_id, letterhead_id, footer_id,
			terms_text, is_watermark, watermark_text, is_tax, table_style
		) VALUES (
			:template_id, :primary_color, :accent_color, :body_font_family, :header_font_family,
			:is_logo, :logo_id, :letterhead_id, :footer_id,
			:terms_text, :is_watermark, :watermark_text, :is_tax, :table_style
		)
		ON CONFLICT (template_id) DO UPDATE SET
			primary_color     = EXCLUDED.primary_color,
			accent_color      = EXCLUDED.accent_color,
			body_font_family  = EXCLUDED.body_font_family,
			header_font_family = EXCLUDED.header_font_family,
			is_logo           = EXCLUDED.is_logo,
			logo_id           = EXCLUDED.logo_id,
			letterhead_id     = EXCLUDED.letterhead_id,
			footer_id         = EXCLUDED.footer_id,
			terms_text        = EXCLUDED.terms_text,
			is_watermark      = EXCLUDED.is_watermark,
			watermark_text    = EXCLUDED.watermark_text,
			is_tax = EXCLUDED.is_tax,
			table_style = EXCLUDED.table_style,
			updated_at        = NOW()
		RETURNING id, created_at, updated_at`

	rows, err := r.db.NamedQueryContext(ctx, q, st)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return rows.StructScan(st)
	}
	return nil
}

func (r *Repository) BulkCreate(ctx context.Context, templates []Template) error {
	const q = `
		INSERT INTO tbl_template (clinic_id, name, description, html, css, is_default, is_active)
		VALUES (:clinic_id, :name, :description, :html, :css, :is_default, :is_active)
		RETURNING id, created_at`

	rows, err := r.db.NamedQueryContext(ctx, q, templates)
	if err != nil {
		return err
	}
	defer rows.Close()

	i := 0
	for rows.Next() {
		if err := rows.StructScan(&templates[i]); err != nil {
			return err
		}
		i++
	}
	return rows.Err()
}

func (r *Repository) CreateSetting(ctx context.Context, st *Setting) error {
	const q = `
		INSERT INTO tbl_template_setting (
			template_id, primary_color, accent_color, body_font_family, header_font_family,
			is_logo, logo_id, letterhead_id, footer_id, terms_text, is_watermark, watermark_text
		) VALUES (
			:template_id, :primary_color, :accent_color, :body_font_family, :header_font_family,
			:is_logo, :logo_id, :letterhead_id, :footer_id, :terms_text, :is_watermark, :watermark_text
		)
		RETURNING id, created_at`

	rows, err := r.db.NamedQueryContext(ctx, q, st)
	if err != nil {
		return err
	}
	defer rows.Close()

	if rows.Next() {
		return rows.StructScan(st)
	}
	return rows.Err()
}

func (r *Repository) GetDocumentByID(ctx context.Context, id uuid.UUID) (*file.Document, error) {
	const q = `SELECT * FROM tbl_document WHERE id = $1 AND deleted_at IS NULL`
	var doc file.Document
	if err := r.db.GetContext(ctx, &doc, q, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}
