package invitation

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(invite *gin.RouterGroup, h IHandler) {
	invite.POST("", h.SendInvitation)
	invite.POST("/:id/resend", h.ResendInvitation)
	invite.DELETE("/:id/revoke", h.RevokeInvitation)
	invite.GET("", h.ListInvitations)
	invite.PUT("/permission", h.UpdatePermission)
	invite.GET("/list-permission", h.ListPermissions)
}
