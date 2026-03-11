package entry

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	rg.GET("", h.List)
	rg.POST("", h.Create)
	rg.GET("/:id", h.Get)
	rg.PATCH("/:id", h.Update)
	rg.DELETE("/:id", h.Delete)
}

// MiddlewareClinicID sets clinic_id from path param for entry routes (clinic_id may be in parent path).
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
