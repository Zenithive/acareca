package preference

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(nft *gin.RouterGroup, h IHandler) {
	nft.GET("/preferences", h.Get)
	nft.PUT("/preference", h.Update)
}
