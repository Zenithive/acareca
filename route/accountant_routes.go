package route

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
)

func RegisterAccountantRoutes(v1 *gin.RouterGroup, cfg *config.Config, accountantSvc accountant.IService) {
	accountantHandler := accountant.NewHandler(accountantSvc)
	accountant.RegisterRoutes(v1, accountantHandler, middleware.Auth(cfg))
}
