package audit

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h Handler) {
	rg.GET("/", h.ListAuditLogs)
	rg.GET("/:id", h.GetAuditLog)
}
