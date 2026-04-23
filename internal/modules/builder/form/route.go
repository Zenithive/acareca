package form

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	// All form operations - no permission checks
	rg.GET("/:id", h.GetFormWithFields)
	rg.GET("", h.List)
	rg.POST("", h.CreateFormWithFields)
	rg.PATCH("/:id", h.UpdateFormWithFields)
	rg.DELETE("/:id", h.Delete)
	rg.PATCH("/:id/status", h.UpdateFormStatus)
}
