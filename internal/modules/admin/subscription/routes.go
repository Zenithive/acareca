package subscription

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	rg.Use(middleware.RequireRole("ADMIN"))
	rg.POST("", h.CreateSubscription)
	rg.GET("", h.ListSubscriptions)
	rg.GET("/:id", h.GetSubscription)
	rg.PATCH("/:id", h.UpdateSubscription)
	rg.DELETE("/:id", h.DeleteSubscription)

	// Permission management
	rg.GET("/:id/permissions", h.ListPermissions)
	rg.PUT("/:id/permissions/:key", h.UpdatePermission)
}
