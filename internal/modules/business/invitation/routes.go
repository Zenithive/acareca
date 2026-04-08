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
	invite.Use(middleware.Auth(cfg))

	invite.POST("", h.SendInvitation)
	invite.POST("/:id/resend", h.ResendInvitation)
	invite.DELETE("/:id/revoke", h.RevokeInvitation)
	invite.GET("", h.ListInvitations)
	invite.POST("/permissions", h.HandlePermissions)
	invite.GET("/list-permissions", h.ListAccountantPermissions)
}
