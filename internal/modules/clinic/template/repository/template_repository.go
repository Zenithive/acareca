package repository

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template/domain"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// ITemplateRepository handles template data persistence
type ITemplateRepository interface {
	Create(ctx context.Context, t *domain.Template) error
	BulkCreate(ctx context.Context, templates []domain.Template) error
	Update(ctx context.Context, t *domain.Template) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*domain.Template, error)
	List(ctx context.Context, method string) (*util.RsList, error)
	ValidateAccess(ctx context.Context, templateIds []uuid.UUID) error
}

// TemplateRepository implements template data access
type TemplateRepository struct {
	db *sqlx.DB
}

// NewTemplateRepository creates a new template repository
func NewTemplateRepository(db *sqlx.DB) ITemplateRepository {
	return &TemplateRepository{db: db}
}

// dbTemplate is the database model for templates
type dbTemplate struct {
	ID          uuid.UUID  `db:"id"`
	Name        string     `db:"name"`
	Description *string    `db:"description"`
	Html        []byte     `db:"html"`
	Css         []byte     `db:"css"`
	IsDefault   bool       `db:"is_default"`
	IsActive    bool       `db:"is_active"`
	CreatedAt   string     `db:"created_at"`
	UpdatedAt   *string    `db:"updated_at"`
	DeletedAt   *string    `db:"deleted_at"`
}

// Create inserts a new template into the database
func (r *TemplateRepository) Create(ctx context.Context, t *domain.Template) error {
	const q = `
		INSERT INTO tbl_template (name, description, html, css, is_default, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`
	
	err := r.db.QueryRowContext(ctx, q, t.Name, t.Description, t.Html, t.Css, t.IsDefault, t.IsActive).
		Scan(&t.ID, &t.CreatedAt)
	
	return err
}

// Update modifies an existing template
func (r *TemplateRepository) Update(ctx context.Context, t *domain.Template) error {
	const q = `
		UPDATE tbl_template
		SET name = $1, html = $2, css = $3, is_default = $4, is_active = $5, updated_at = NOW()
		WHERE id = $6 AND deleted_at IS NULL`
	
	_, err := r.db.ExecContext(ctx, q, t.Name, t.Html, t.Css, t.IsDefault, t.IsActive, t.ID)
	return err
}

// Delete soft deletes a template
func (r *TemplateRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE tbl_template SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.db.ExecContext(ctx, q, id)
	return err
}

// Get retrieves a single template by ID
func (r *TemplateRepository) Get(ctx context.Context, id uuid.UUID) (*domain.Template, error) {
	const q = `
		SELECT id, name, description, html, css, is_default, is_active, created_at, updated_at, deleted_at 
		FROM tbl_template 
		WHERE id = $1 AND deleted_at IS NULL`
	
	var db dbTemplate
	if err := r.db.GetContext(ctx, &db, q, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("template not found")
		}
		return nil, fmt.Errorf("failed to get template: %w", err)
	}
	
	return r.toDomain(&db), nil
}

// List returns templates filtered by invoice method
func (r *TemplateRepository) List(ctx context.Context, method string) (*util.RsList, error) {
	var query string
	var args []interface{}
	var err error

	// Get template names for the specified method
	templateNames := domain.GetTemplateNames(method)

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
		// No method filter: return all templates
		query = `
			SELECT id, name, description, html, css, is_default, is_active, created_at, updated_at, deleted_at 
			FROM tbl_template 
			WHERE deleted_at IS NULL 
			ORDER BY name ASC, created_at DESC`
	}

	var dbItems []dbTemplate
	if err := r.db.SelectContext(ctx, &dbItems, query, args...); err != nil {
		return nil, fmt.Errorf("failed to scan templates: %w", err)
	}

	// Sort templates based on page order if method is specified
	if len(templateNames) > 0 {
		pageOrder := domain.GetPageOrder(method)
		sort.Slice(dbItems, func(i, j int) bool {
			return pageOrder[dbItems[i].Name] < pageOrder[dbItems[j].Name]
		})
	}

	// Convert to response format
	rs := make([]map[string]interface{}, len(dbItems))
	for i, db := range dbItems {
		rs[i] = map[string]interface{}{
			"id":          db.ID,
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

// BulkCreate inserts multiple templates
func (r *TemplateRepository) BulkCreate(ctx context.Context, templates []domain.Template) error {
	const q = `
		INSERT INTO tbl_template (name, description, html, css, is_default, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`

	for i := range templates {
		err := r.db.QueryRowContext(ctx, q, 
			templates[i].Name, 
			templates[i].Description, 
			templates[i].Html, 
			templates[i].Css, 
			templates[i].IsDefault, 
			templates[i].IsActive,
		).Scan(&templates[i].ID, &templates[i].CreatedAt)
		
		if err != nil {
			return fmt.Errorf("failed to create template %s: %w", templates[i].Name, err)
		}
	}
	
	return nil
}

// ValidateAccess ensures all template IDs exist and are active
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

// toDomain converts database model to domain entity
func (r *TemplateRepository) toDomain(db *dbTemplate) *domain.Template {
	return &domain.Template{
		ID:          db.ID,
		Name:        db.Name,
		Description: db.Description,
		Html:        db.Html,
		Css:         db.Css,
		IsDefault:   db.IsDefault,
		IsActive:    db.IsActive,
	}
}
