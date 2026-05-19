package contact

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	contact := rg.Group("/contact")

	contact.POST("/", h.Create)
	contact.GET("/", h.List)
	contact.GET("/:id", h.Get)
	contact.PUT("/:id", h.Update)
	contact.DELETE("/:id", h.Delete)

}
