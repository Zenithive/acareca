package notification

import (
	"github.com/gin-gonic/gin"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, hub *sharednotification.Hub, cfg *config.Config) {
	nft := rg.Group("/notification")

	// WebSocket — auth via ?token= query param (no Bearer header possible in WS)
	nft.GET("/ws", hub.ServeWS(cfg))

	// REST endpoints — require Bearer auth
	nft.Use(middleware.Auth(cfg))
	nft.GET("", h.ListNotifications)
	nft.PATCH("/:id/read", h.MarkRead)
	nft.PATCH("/:id/dismissed", h.MarkDismissed)
}
