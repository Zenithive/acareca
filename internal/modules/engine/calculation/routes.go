package calculation

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	rg.POST("/calculate", h.CalculateFromEntries)
	rg.POST("/calculate/live", h.LiveCalculate)
	rg.POST("/calculate/preview", h.FormPreview)
	rg.GET("/calculate/:id", h.Calculation)
	rg.GET("/calculate/formula/:form_id", h.FormulaCalculate)
	rg.GET("/summary/:id", h.GetFormSummary)
}
