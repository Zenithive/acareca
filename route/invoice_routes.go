package route

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	invoice "github.com/iamarpitzala/acareca/internal/modules/clinic/auth"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
)

func RegisterInvoiceRoutes(v1 *gin.RouterGroup, cfg *config.Config, dbConn *sqlx.DB, auditSvc audit.Service, tmp template.IService) {
	repo := invoice.NewRepository(dbConn)
	svc := invoice.NewService(repo, cfg, dbConn, auditSvc, tmp)
	h := invoice.NewHandler(svc)

	tmpRepo := template.NewRepository(dbConn)
	tmpSvc := template.NewService(tmpRepo, cfg)
	tmpHld := template.NewHandler(tmpSvc)

	invoice.RegisterRoutes(v1, h, middleware.Auth(cfg), middleware.RequireRole(util.RoleClinic))
	template.RegisterRoutes(v1, tmpHld, middleware.Auth(cfg), middleware.RequireRole(util.RoleClinic))
}
