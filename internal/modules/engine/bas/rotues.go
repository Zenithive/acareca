package bas

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, cfg *config.Config) {
	bas := rg.Group("/bas")
	bas.Use(middleware.Auth(cfg))

	bas.GET("/report", h.GetReport)
	bas.GET("/bas-preparation", h.GetBASPreparation)
	bas.GET("/activity-statement/report/export", h.ExportBASReport)
	bas.GET("/bas-preparation/export", h.ExportBASPreparation)
}
