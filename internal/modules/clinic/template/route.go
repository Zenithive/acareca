package template

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, authMiddleware gin.HandlerFunc, roleMiddleware gin.HandlerFunc) {
	template := rg.Group("/template", authMiddleware, roleMiddleware)

	template.POST("", h.Create)
	template.GET("", h.List)
	template.GET("/:id", h.Get)
	template.PUT("/:id", h.Update)
	template.DELETE("/:id", h.Delete)

	template.PUT("/:id/setting", h.UpdateSetting)
	template.GET("/:id/setting", h.GetSetting)
}
