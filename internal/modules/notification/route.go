package notification

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(nft *gin.RouterGroup, h IHandler) {
	nft.GET("", h.ListNotifications)
	nft.PATCH("/:id/read", h.MarkRead)
	nft.PATCH("/read-all", h.MarkAllRead)
	nft.PATCH("/:id/dismissed", h.MarkDismissed)
	nft.GET("/preferences", h.GetPreferences)
	nft.PUT("/preferences", h.UpdatePreference)
}
