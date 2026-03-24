package setting

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, cfg *config.Config) {
	rg.Use(middleware.Auth(cfg), middleware.AuditContext())
	rg.POST("", h.CreatePractitioner)
	rg.GET("list", h.ListPractitioners)
	rg.GET("/by-user/:user_id", h.GetPractitionerByUserID)
	rg.GET("setting", h.GetSetting)
	rg.PUT("/setting", h.UpsertSetting)
	rg.GET("", h.GetPractitioner)
	rg.PATCH("", h.UpdatePractitioner)
	rg.DELETE("", h.DeletePractitioner)
}
