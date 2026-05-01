package form

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, permAdapter *middleware.PermissionAdapter) {
	// Expense routes - must come before generic /:id routes to avoid conflicts
	rg.POST("/expenses", h.CreateExpense)
	rg.GET("/expenses/:id", h.GetExpense)
	rg.PATCH("/expenses/:id", h.UpdateExpense)
	
	// All form operations - no permission checks
	rg.GET("/:id", h.GetFormWithFields)
	rg.GET("", h.List)
	rg.POST("", h.CreateFormWithFields)
	rg.PATCH("/:id", h.UpdateFormWithFields)
	rg.DELETE("/:id", h.Delete)
	rg.PATCH("/:id/status", h.UpdateFormStatus)
}
