package practitioner

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, cfg *config.Config) {
	g := rg.Group("/practitioner")
	g.Use(middleware.Auth(cfg))
	g.GET("", h.ListPractitioners)
	g.GET("/:id", h.GetPractitioner)
}
