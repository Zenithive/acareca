package setting

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	rg.POST("", h.CreateTentant)
	rg.GET("", h.ListTentants)
	rg.GET("/by-user/:user_id", h.GetTentantByUserID)
	rg.GET("/:id/setting", h.GetSetting)
	rg.PUT("/:id/setting", h.UpsertSetting)
	rg.GET("/:id", h.GetTentant)
	rg.PATCH("/:id", h.UpdateTentant)
	rg.DELETE("/:id", h.DeleteTentant)
}
