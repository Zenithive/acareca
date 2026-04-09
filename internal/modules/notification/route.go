package notification

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(nft *gin.RouterGroup, h IHandler) {
	nft.GET("", h.ListNotifications)
	nft.PATCH("/:id/read", h.MarkRead)
	nft.PATCH("/:id/dismissed", h.MarkDismissed)
}
