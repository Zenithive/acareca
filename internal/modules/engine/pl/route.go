package pl

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, cfg *config.Config, permAdapter *middleware.PermissionAdapter, db *sqlx.DB) {
	pl := rg.Group("/pl")
	pl.Use(middleware.Auth(cfg), middleware.RequireActiveSubscription(db))
	pl.GET("/summary", h.GetMonthlySummary)
	pl.GET("/by-account", h.GetByAccount)
	pl.GET("/by-responsibility", h.GetByResponsibility)
	pl.GET("/fy-summary", h.GetFYSummary)
	pl.GET("/report", h.GetReport)
	pl.GET("/export", h.ExportReport)
}
