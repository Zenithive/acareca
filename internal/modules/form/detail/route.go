package detail

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	rg.GET("", h.ListForm)
	rg.POST("", h.CreateForm)
	rg.GET("/:form_id", h.GetForm)
	rg.PATCH("/:form_id", h.UpdateForm)
	rg.DELETE("/:form_id", h.DeleteForm)
}

func MiddlewareClinicID() gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		if idStr == "" {
			c.Next()
			return
		}
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.Next()
			return
		}
		c.Set(util.ClinicIDKey, id)
		c.Next()
	}
}
