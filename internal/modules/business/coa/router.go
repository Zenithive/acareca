package coa

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	rg.GET("/account-types", h.ListAccountTypes)
	rg.GET("/account-types/:id", h.GetAccountType)
	rg.GET("/account-taxes", h.ListAccountTaxes)
	rg.GET("/account-taxes/:id", h.GetAccountTax)

	// Chart of Accounts CRUD — scoped by practitioner_id
	accounts := rg.Group("/chat-of-account")
	accounts.GET("", h.ListChartOfAccount)
	accounts.GET("/:id", h.GetChartOfAccount)
	accounts.POST("", h.CreateChartOfAccount)
	accounts.PUT("/:id", h.UpdateCharOfAccount)
	accounts.DELETE("/:id", h.DeleteChartOfAccount)
}
