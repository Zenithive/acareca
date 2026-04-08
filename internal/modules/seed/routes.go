package seed

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	seedRoutes := rg.Group("/seed")
	{
		seedRoutes.POST("", h.SeedData)
		seedRoutes.POST("/cleanup", h.CleanupData)
	}
}
