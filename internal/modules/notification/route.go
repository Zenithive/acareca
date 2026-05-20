package notification

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(nft *gin.RouterGroup, h IHandler) {
	nft.GET("", h.ListNotifications)
	nft.PATCH("/:id/read", h.MarkRead)
	nft.PATCH("/read-all", h.MarkAllRead)
	nft.PATCH("/:id/dismiss", h.MarkDismissed)
	nft.PATCH("/dismiss", h.MarkAllDismissed)

	nft.GET("/preferences", h.GetPreferences)
	nft.PUT("/preference", h.UpdatePreference)
}
