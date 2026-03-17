package clinic

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, cfg *config.Config) {
	clinic := rg.Group("/clinic")
	clinic.Use(middleware.Auth(cfg), middleware.AuditContext())

	clinic.POST("/create", h.CreateClinic)
	clinic.GET("/all", h.GetClinics)
	clinic.GET("/:id", h.GetClinicByID)
	clinic.PUT("/:id", h.UpdateClinic)
	clinic.PUT("/bulk-update", h.BulkUpdateClinics)
	clinic.DELETE("/:id", h.DeleteClinic)
	clinic.DELETE("/bulk-delete", h.BulkDeleteClinics)
}
