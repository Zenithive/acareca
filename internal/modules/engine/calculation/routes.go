package calculation

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	rg.POST("/net-amount", h.NetAmount)
	rg.POST("/net-result", h.NetResult)
	rg.POST("/gross-result", h.GrossResult)
	rg.POST("/out-work-result", h.OutWorkResult)
}
