package template

import (
	"fmt"
	"sync"

	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
)

// Container manages all dependencies for the template module
type Container struct {
	cfg *config.Config
	db  *sqlx.DB

	// Lazy-loaded handler
	handler IHandler
	hOnce   sync.Once

	// Lazy-loaded legacy service (for backward compatibility)
	legacySvc  IService
	legacyOnce sync.Once
}

// NewContainer creates a new dependency injection container
func NewContainer(cfg *config.Config, db *sqlx.DB) (*Container, error) {
	// Validate encryption key immediately
	if len(cfg.TemplateEncryptionKey) != 32 {
		return nil, fmt.Errorf("template encryption key must be exactly 32 characters, got %d", len(cfg.TemplateEncryptionKey))
	}

	return &Container{
		cfg: cfg,
		db:  db,
	}, nil
}

// Handler - lazy initialization
func (c *Container) Handler() IHandler {
	c.hOnce.Do(func() {
		// Use legacy service for now to maintain backward compatibility
		c.handler = NewHandler(c.LegacyService())
	})
	return c.handler
}

// LegacyService provides the old monolithic service interface for backward compatibility
// This allows existing code to continue working while we migrate to the new structure
func (c *Container) LegacyService() IService {
	c.legacyOnce.Do(func() {
		// Create the old repository interface using the existing repository
		legacyRepo := NewRepository(c.db)
		
		// Create the legacy service
		c.legacySvc = NewService(legacyRepo, c.cfg)
	})
	return c.legacySvc
}

// Configuration provides access to the config
func (c *Container) Config() *config.Config {
	return c.cfg
}

// DB provides access to the database connection
func (c *Container) DB() *sqlx.DB {
	return c.db
}
