package subscription

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	rg.POST("/", h.CreateSubscription)
	rg.GET("/", h.ListSubscriptions)
	rg.GET("/:id", h.GetSubscription)
	rg.PATCH("/:id", h.UpdateSubscription)
	rg.DELETE("/:id", h.DeleteSubscription)
}
