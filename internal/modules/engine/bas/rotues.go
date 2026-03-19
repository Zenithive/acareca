package bas

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, cfg *config.Config) {
	bas := rg.Group("/bas")
	bas.Use(middleware.Auth(cfg))

	clinic := bas.Group("/clinic/:clinic_id")
	{
		clinic.GET("/summary", h.GetQuarterlySummary)
		clinic.GET("/by-account", h.GetByAccount)
		clinic.GET("/monthly", h.GetMonthly)
	}
}
