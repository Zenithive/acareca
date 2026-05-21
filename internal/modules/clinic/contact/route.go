package contact

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, authMiddleware gin.HandlerFunc, roleMiddleware gin.HandlerFunc) {
	contact := rg.Group("/contact", authMiddleware, roleMiddleware)

	contact.POST("/", h.Create)
	contact.GET("/", h.List)
	contact.GET("/:id", h.Get)
	contact.PUT("/:id", h.Update)
	contact.DELETE("/:id", h.Delete)

	contact.DELETE("/address/:id", h.DeleteAddressByID)
}
