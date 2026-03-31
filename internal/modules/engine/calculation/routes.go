package calculation

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	rg.POST("/calculate", h.CalculateFromEntries)
	rg.POST("/calculate/live", h.LiveCalculate)
	rg.GET("/calculate/:id", h.Calculation)
	rg.GET("/calculate/formula/:form_id", h.FormulaCalculate)
}
