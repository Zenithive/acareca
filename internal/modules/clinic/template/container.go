package template

import (
	"fmt"
	"sync"

	"github.com/iamarpitzala/acareca/internal/modules/clinic/template/repository"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
)

type Container struct {
	cfg *config.Config
	db  *sqlx.DB

	templateRepo repository.ITemplateRepository
	settingRepo  repository.ISettingRepository
	reposOnce    sync.Once

	handler IHandler
	hOnce   sync.Once

	serviceFactory ServiceFactory
}

type ServiceFactory func(cfg *config.Config, templateRepo repository.ITemplateRepository, settingRepo repository.ISettingRepository) IService

func NewContainer(cfg *config.Config, db *sqlx.DB) (*Container, error) {
	if len(cfg.TemplateEncryptionKey) != 32 {
		return nil, fmt.Errorf("template encryption key must be exactly 32 characters, got %d", len(cfg.TemplateEncryptionKey))
	}

	return &Container{
		cfg: cfg,
		db:  db,
	}, nil
}

func (c *Container) SetServiceFactory(factory ServiceFactory) {
	c.serviceFactory = factory
}

func (c *Container) initRepositories() {
	c.reposOnce.Do(func() {
		c.templateRepo = repository.NewTemplateRepository(c.db)
		c.settingRepo = repository.NewSettingRepository(c.db)
	})
}

func (c *Container) Service() IService {
	c.initRepositories()

	if c.serviceFactory == nil {
		panic("service factory not set - call SetServiceFactory first")
	}

	return c.serviceFactory(c.cfg, c.templateRepo, c.settingRepo)
}

func (c *Container) SettingRepo() repository.ISettingRepository {
	c.initRepositories()
	return c.settingRepo
}

func (c *Container) Handler() IHandler {
	c.hOnce.Do(func() {
		c.handler = NewHandler(c.Service())
	})
	return c.handler
}

func (c *Container) Config() *config.Config {
	return c.cfg
}
