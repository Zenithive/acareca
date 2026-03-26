package invitation

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, cfg *config.Config) {
	invite := rg.Group("/invite")

	// Public Route (These endpoints are for the person receiving the invite)
	invite.POST("/process", h.ProcessInvitation)
	invite.GET("/:id", h.GetInvitation)

	// Protected Route
	protected := invite.Group("/")
	protected.Use(middleware.Auth(cfg))

	protected.POST("", h.SendInvitation)
	protected.GET("", h.ListInvitations)
}
