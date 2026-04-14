package form

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, permChecker middleware.PermissionChecker) {
	// Apply permission middleware for all form routes
	rg.Use(middleware.MethodBasedPermission(permChecker))

	rg.GET("/:id", h.GetById)
	rg.GET("", h.List)
	rg.POST("", h.CreateFormWithFields)
	rg.PATCH("/:id", h.UpdateFormWithFields)
	rg.DELETE("/:id", h.Delete)
	rg.PATCH("/:id/status", h.UpdateFormStatus)
}
