package clinic

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, cfg *config.Config, permAdapter *middleware.PermissionAdapter, db *sqlx.DB) {
	clinic := rg.Group("/clinic")
	clinic.Use(middleware.Auth(cfg), middleware.RequireActiveSubscription(db), middleware.AuditContext(), middleware.SetPractitionerIDFromAuth())

	// All clinic operations - no permission checks
	clinic.GET("", h.List)
	clinic.GET("/:id", h.GetByID)
	clinic.POST("", h.Create)
	clinic.PUT("/:id", h.Update)
	clinic.PUT("/bulk-update", h.BulkUpdate)
	clinic.DELETE("/:id", h.Delete)
	clinic.DELETE("/bulk-delete", h.BulkDelete)
}
