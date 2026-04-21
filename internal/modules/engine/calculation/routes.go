package calculation

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, permAdapter *middleware.PermissionAdapter) {
	// Calculations require sales_purchases permission (read-only for viewing, write for creating)
	
	// Read operations
	calcRead := rg.Group("")
	calcRead.Use(middleware.RequireFeaturePermission(permAdapter, middleware.FeatureSalesPurchases, middleware.ActionRead))
	{
		calcRead.GET("/calculate/:id", h.Calculation)
		calcRead.GET("/calculate/formula/:form_id", h.FormulaCalculate)
		calcRead.GET("/summary/:id", h.GetFormSummary)
	}
	
	// Write operations (creating calculations)
	calcWrite := rg.Group("")
	calcWrite.Use(middleware.RequireFeaturePermission(permAdapter, middleware.FeatureSalesPurchases, middleware.ActionWrite))
	{
		calcWrite.POST("/calculate", h.CalculateFromEntries)
		calcWrite.POST("/calculate/live", h.LiveCalculate)
		calcWrite.POST("/calculate/preview", h.FormPreview)
	}
}
