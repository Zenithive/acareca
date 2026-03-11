package clinic

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	rg.PUT("/create", h.CreateClinic)
	rg.GET("/all", h.GetClinics)
	rg.GET("/:id", h.GetClinicByID)
	rg.DELETE("/:id", h.DeleteClinic)
}
