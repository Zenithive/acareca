package admin

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, cfg *config.Config) {
	admin := rg.Group("/admin")

	admin.POST("", h.Create)
	protected := admin.Group("/")
	protected.Use(middleware.Auth(cfg))
	{
		protected.GET("/:id", h.GetById)
	}
}
