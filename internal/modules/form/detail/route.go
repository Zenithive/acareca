package detail

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	rg.GET("", h.ListForm)
	rg.POST("", h.CreateForm)
	rg.GET("/:id", h.GetForm)
	rg.PATCH("/:id", h.UpdateForm)
	rg.DELETE("/:id", h.DeleteForm)
}
