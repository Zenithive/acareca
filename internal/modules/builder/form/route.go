package form

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, permAdapter *middleware.PermissionAdapter) {
	// Forms require sales_purchases permission
	// Read operations
	formRead := rg.Group("")
	formRead.Use(middleware.RequireFeaturePermission(permAdapter, middleware.FeatureSalesPurchases, middleware.ActionRead))
	{
		formRead.GET("/:id", h.GetFormWithFields)
		formRead.GET("", h.List)
	}

	// Write operations
	formWrite := rg.Group("")
	formWrite.Use(middleware.RequireFeaturePermission(permAdapter, middleware.FeatureSalesPurchases, middleware.ActionWrite))
	{
		formWrite.POST("", h.CreateFormWithFields)
		formWrite.PATCH("/:id", h.UpdateFormWithFields)
		formWrite.DELETE("/:id", h.Delete)
		formWrite.PATCH("/:id/status", h.UpdateFormStatus)
	}
}
