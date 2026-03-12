package form

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	rg.GET("", h.List)
	rg.GET("/:id", h.GetFormWithFields)
	rg.POST("/sync", h.Sync)
	rg.POST("/create", h.CreateFormWithFields)
	rg.PATCH("/:id/update", h.UpdateFormWithFields)
	rg.DELETE("/:id", h.Delete)
}
