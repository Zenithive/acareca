package subscription

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	rg.Use(MiddlewareTentantID())
	rg.GET("", h.ListByTentantID)
	rg.POST("", h.Create)
	rg.GET("/:sub_id", h.GetByID)
	rg.PATCH("/:sub_id", h.Update)
	rg.DELETE("/:sub_id", h.Delete)
}

func MiddlewareTentantID() gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		if idStr == "" {
			c.Next()
			return
		}
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.Next()
			return
		}
		c.Set(tentantIDKey, id)
		c.Next()
	}
}
