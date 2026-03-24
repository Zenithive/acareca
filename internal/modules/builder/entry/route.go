package entry

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	rg.GET("/transactions", h.ListTransactions)
	rg.GET("/version/:version_id", h.List)
	rg.POST("/version/:version_id", h.Create)
	rg.GET("/:id", h.Get)
	rg.PATCH("/:id", h.Update)
	rg.DELETE("/:id", h.Delete)
	// rg.GET("/summary/:field_id", h.GetFieldSummary)
}
