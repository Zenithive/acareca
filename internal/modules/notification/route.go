package notification

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
	"github.com/iamarpitzala/acareca/pkg/config"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, hub *sharednotification.Hub, cfg *config.Config) {
	nft := rg.Group("/notification")

	// WebSocket — auth via ?token= query param
	nft.GET("/ws", hub.ServeWS(cfg))

	// REST — require Bearer auth
	nft.Use(middleware.Auth(cfg))
	nft.GET("", h.ListNotifications)
	nft.PATCH("/:id/read", h.MarkRead)
	nft.PATCH("/:id/dismissed", h.MarkDismissed)
}
