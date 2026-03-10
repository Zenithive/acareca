package detail

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	rg.Use(MiddlewareClinicID())
	rg.GET("", h.ListForm)
	rg.POST("", h.CreateForm)
	rg.GET("/:id", h.GetForm)
	rg.PATCH("/:id", h.UpdateForm)
	rg.DELETE("/:id", h.DeleteForm)
}

func MiddlewareClinicID() gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("clinic_id")
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
