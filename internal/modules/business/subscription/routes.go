package subscription

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	rg.Use(MiddlewarePractitionerID())

	rg.GET("/active", h.GetActiveSubscription)
	rg.GET("/history", h.GetSubscriptionHistory)

	rg.GET("", h.ListByPractitionerID)
	rg.POST("", h.Create)
	rg.GET("/:sub_id", h.GetByID)
	rg.PATCH("/:sub_id", h.Update)
	rg.DELETE("/:sub_id", h.Delete)

}

func MiddlewarePractitionerID() gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("prac_id")
		if idStr == "" {
			c.Next()
			return
		}
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.Next()
			return
		}
		c.Set(util.PractitionerIDKey, id)
		c.Next()
	}
}
