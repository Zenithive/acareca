package invitation

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, cfg *config.Config) {
	invite := rg.Group("/invite")

	// Public Route (These endpoints are for the person receiving the invite)
	invite.GET("/:id", h.ProcessInvitation)

	// Protected Route (Only practitioners can send invitations)
	protected := invite.Group("/")
	protected.Use(middleware.Auth(cfg))
	{
		protected.POST("/", h.SendInvitation)
	}
}
