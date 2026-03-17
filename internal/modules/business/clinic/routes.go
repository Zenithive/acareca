package clinic

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, cfg *config.Config) {
	clinic := rg.Group("/clinic")
	clinic.Use(middleware.Auth(cfg), middleware.AuditContext())

	clinic.POST("", h.Create)
	clinic.GET("", h.List)
	clinic.GET("/:id", h.GetByID)
	clinic.PUT("/:id", h.Update)
	clinic.PUT("/bulk-update", h.BulkUpdate)
	clinic.DELETE("/:id", h.Delete)
	clinic.DELETE("/bulk-delete", h.BulkDelete)
}
