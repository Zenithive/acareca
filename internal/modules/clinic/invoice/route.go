package invoice

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	invoice := rg.Group("/invoice")

	invoice.POST("/", h.Create)
	invoice.GET("/", h.List)
	invoice.GET("/:id", h.Get)
	invoice.PUT("/:id", h.Update)
	invoice.DELETE("/:id", h.Delete)
}
