package calculation

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, permAdapter *middleware.PermissionAdapter) {
	// All calculation operations - no permission checks
	
	rg.GET("/calculate/:id", h.Calculation)
	rg.GET("/calculate/formula/:form_id", h.FormulaCalculate)
	rg.GET("/summary/:id", h.GetFormSummary)
	rg.POST("/calculate", h.CalculateFromEntries)
	rg.POST("/calculate/live", h.LiveCalculate)
	rg.POST("/calculate/preview", h.FormPreview)
}
