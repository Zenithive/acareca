package form

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, permAdapter *middleware.PermissionAdapter) {
	// All form operations - no permission checks
	rg.GET("/:id", h.GetFormWithFields)
	rg.GET("", h.List)
	rg.POST("", h.CreateFormWithFields)
	rg.PATCH("/:id", h.UpdateFormWithFields)
	rg.DELETE("/:id", h.Delete)
	rg.PATCH("/:id/status", h.UpdateFormStatus)
	rg.POST("/expenses", h.CreateExpense)
	rg.GET("/expenses/:id", h.GetExpense)
}
