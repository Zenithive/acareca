package coa

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	group := rg.Group("/chart-of-account-templates")

	group.POST("", h.Create)
	group.PUT("", h.Update)
	group.GET("", h.List)
	group.GET("/:id", h.GetById)
	group.DELETE("/:id", h.Delete)
}
