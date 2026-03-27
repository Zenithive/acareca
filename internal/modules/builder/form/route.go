package form

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	rg.GET("/form/:id", h.GetById)
	rg.GET("", h.List)
	rg.GET("/:id", h.GetFormWithFields)
	// rg.POST("/sync", h.Sync)
	rg.POST("", h.CreateFormWithFields)
	rg.PATCH("/:id", h.UpdateFormWithFields)
	rg.DELETE("/:id", h.Delete)
	rg.PATCH("/:id/status", h.UpdateFormStatus)
}
