package pl

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, cfg *config.Config) {
	pl := rg.Group("/pl")
	pl.Use(middleware.Auth(cfg))

	pl.GET("/summary", h.GetMonthlySummary)
	pl.GET("/by-account", h.GetByAccount)
	pl.GET("/by-responsibility", h.GetByResponsibility)
	pl.GET("/fy-summary", h.GetFYSummary)
}
