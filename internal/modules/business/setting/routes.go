package setting

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	rg.POST("", h.CreatePractitioner)
	rg.GET("", h.ListPractitioners)
	rg.GET("/by-user/:user_id", h.GetPractitionerByUserID)
	rg.GET("/:id/setting", h.GetSetting)
	rg.PUT("/:id/setting", h.UpsertSetting)
	rg.GET("/:id", h.GetPractitioner)
	rg.PATCH("/:id", h.UpdatePractitioner)
	rg.DELETE("/:id", h.DeletePractitioner)
}
