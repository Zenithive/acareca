package repository

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// ITemplateRepository defines template data access interface
type ITemplateRepository interface {
	Create(ctx context.Context, t *common.Template) error
	Update(ctx context.Context, t *common.Template) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*common.Template, error)
	List(ctx context.Context, method string) (*util.RsList, error)
	BulkCreate(ctx context.Context, templates []common.Template) error
	ValidateAccess(ctx context.Context, templateIds []uuid.UUID) error
	GetClinicMailTemplate(ctx context.Context, clinicID uuid.UUID) (string, string, error)
	SaveClinicMailTemplate(ctx context.Context, clinicID uuid.UUID, subject, body string) error
}

type TemplateRepository struct {
	db *sqlx.DB
}

func NewTemplateRepository(db *sqlx.DB) ITemplateRepository {
	return &TemplateRepository{db: db}
}

func (r *TemplateRepository) Create(ctx context.Context, t *common.Template) error {
	const q = `
		INSERT INTO tbl_template (name, description, html, css, is_default, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, q, t.Name, t.Description, t.Html, t.Css, t.IsDefault, t.IsActive).
		Scan(&t.Id, &t.CreatedAt)

	return err
}

func (r *TemplateRepository) BulkCreate(ctx context.Context, templates []common.Template) error {
	const q = `
		INSERT INTO tbl_template (name, description, html, css, is_default, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`

	for _, t := range templates {
		err := r.db.QueryRowContext(ctx, q,
			t.Name,
			t.Description,
			t.Html,
			t.Css,
			t.IsDefault,
			t.IsActive,
		).Scan(&t.Id, &t.CreatedAt)

		if err != nil {
			return fmt.Errorf("failed to create template %s: %w", t.Name, err)
		}
	}

	return nil
}

// Update modifies an existing template
func (r *TemplateRepository) Update(ctx context.Context, t *common.Template) error {
	const q = `
		UPDATE tbl_template
		SET name = $1, html = $2, css = $3, is_default = $4, is_active = $5, updated_at = NOW()
		WHERE id = $6 AND deleted_at IS NULL`

	_, err := r.db.ExecContext(ctx, q, t.Name, t.Html, t.Css, t.IsDefault, t.IsActive, t.Id)
	return err
}

func (r *TemplateRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE tbl_template SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.db.ExecContext(ctx, q, id)
	return err
}

func (r *TemplateRepository) Get(ctx context.Context, id uuid.UUID) (*common.Template, error) {
	const q = `
		SELECT id, name, description, html, css, is_default, is_active, created_at, updated_at, deleted_at 
		FROM tbl_template 
		WHERE id = $1 AND deleted_at IS NULL`

	var template common.Template
	if err := r.db.GetContext(ctx, &template, q, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("template not found")
		}
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	return &template, nil
}

func (r *TemplateRepository) List(ctx context.Context, method string) (*util.RsList, error) {
	var query string
	var args []interface{}
	var err error

	templateNames := common.GetTemplateNames(method)

	if len(templateNames) > 0 {
		query, args, err = sqlx.In(`
			SELECT id, name, description, html, css, is_default, is_active, created_at, updated_at, deleted_at 
			FROM tbl_template 
			WHERE deleted_at IS NULL 
			  AND name IN (?)`, templateNames)
		if err != nil {
			return nil, fmt.Errorf("failed to build query: %w", err)
		}
		query = r.db.Rebind(query)
	} else {
		query = `
			SELECT id, name, description, html, css, is_default, is_active, created_at, updated_at, deleted_at 
			FROM tbl_template 
			WHERE deleted_at IS NULL 
			ORDER BY name ASC, created_at DESC`
	}

	var dbItems []common.Template
	if err := r.db.SelectContext(ctx, &dbItems, query, args...); err != nil {
		return nil, fmt.Errorf("failed to scan templates: %w", err)
	}

	if len(templateNames) > 0 {
		pageOrder := common.GetPageOrder(method)
		sort.Slice(dbItems, func(i, j int) bool {
			return pageOrder[dbItems[i].Name] < pageOrder[dbItems[j].Name]
		})
	}

	rs := make([]map[string]interface{}, len(dbItems))
	for i, db := range dbItems {
		rs[i] = map[string]interface{}{
			"id":          db.Id,
			"name":        db.Name,
			"description": db.Description,
			"html":        base64.StdEncoding.EncodeToString(db.Html),
			"css":         base64.StdEncoding.EncodeToString(db.Css),
			"is_default":  db.IsDefault,
			"is_active":   db.IsActive,
			"created_at":  db.CreatedAt,
			"updated_at":  db.UpdatedAt,
			"deleted_at":  db.DeletedAt,
		}
	}

	return &util.RsList{Items: rs, Total: len(rs)}, nil
}

func (r *TemplateRepository) ValidateAccess(ctx context.Context, templateIds []uuid.UUID) error {
	if len(templateIds) == 0 {
		return nil
	}

	const maxTemplateIds = 10
	if len(templateIds) > maxTemplateIds {
		return fmt.Errorf("too many template IDs provided, maximum is %d", maxTemplateIds)
	}

	const q = `
		SELECT COUNT(*) 
		FROM tbl_template 
		WHERE id = ANY($1) 
		  AND deleted_at IS NULL 
		  AND is_active = TRUE`

	var count int
	if err := r.db.GetContext(ctx, &count, q, pq.Array(templateIds)); err != nil {
		return fmt.Errorf("failed to validate template access: %w", err)
	}

	if count != len(templateIds) {
		return fmt.Errorf("unauthorized access or inactive templates")
	}

	return nil
}

// GetClinicMailTemplate implements [ITemplateRepository].
func (r *TemplateRepository) GetClinicMailTemplate(ctx context.Context, clinicID uuid.UUID) (string, string, error) {
	var subject, body string

	err := r.db.QueryRowContext(ctx, `
		SELECT mail_subject, mail_body 
		FROM tbl_clinic_invoice_mail_templates 
		WHERE clinic_id = $1
	`, clinicID).Scan(&subject, &body)

	if err != nil {
		return "", "", err
	}

	return subject, body, nil
}

// SaveClinicMailTemplate saves or updates clinic mail template
func (r *TemplateRepository) SaveClinicMailTemplate(ctx context.Context, clinicID uuid.UUID, subject, body string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO tbl_clinic_invoice_mail_templates (clinic_id, mail_subject, mail_body)
		VALUES ($1, $2, $3)
		ON CONFLICT (clinic_id) 
		DO UPDATE SET 
			mail_subject = EXCLUDED.mail_subject,
			mail_body = EXCLUDED.mail_body,
			updated_at = NOW()
	`, clinicID, subject, strings.TrimSpace(body))

	return err
}
