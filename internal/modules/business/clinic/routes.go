package clinic

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, cfg *config.Config, permAdapter *middleware.PermissionAdapter) {
	clinic := rg.Group("/clinic")
	clinic.Use(middleware.Auth(cfg), middleware.AuditContext(), middleware.SetPractitionerIDFromAuth())

	// Clinic management requires sales_purchases permission
	// Read operations
	clinicRead := clinic.Group("")
	clinicRead.Use(middleware.RequireFeaturePermission(permAdapter, middleware.FeatureSalesPurchases, middleware.ActionRead))
	{
		clinicRead.GET("", h.List)
		clinicRead.GET("/:id", h.GetByID)
	}

	// Write operations
	clinicWrite := clinic.Group("")
	clinicWrite.Use(middleware.RequireFeaturePermission(permAdapter, middleware.FeatureSalesPurchases, middleware.ActionWrite))
	{
		clinicWrite.POST("", h.Create)
		clinicWrite.PUT("/:id", h.Update)
		clinicWrite.PUT("/bulk-update", h.BulkUpdate)
		clinicWrite.DELETE("/:id", h.Delete)
		clinicWrite.DELETE("/bulk-delete", h.BulkDelete)
	}
}
