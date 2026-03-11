package coa

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	rg.GET("/account-types", h.ListAccountTypes)
	rg.GET("/account-types/:id", h.GetAccountTypeByID)
	rg.GET("/account-taxes", h.ListAccountTaxes)
	rg.GET("/account-taxes/:id", h.GetAccountTaxByID)

	// Chart of Accounts CRUD (global, no clinic)
	accounts := rg.Group("/accounts")
	accounts.GET("", h.ListCharts)
	accounts.GET("/:id", h.GetChartByID)
	accounts.POST("", h.CreateChart)
	accounts.PUT("/:id", h.UpdateChart)
	accounts.DELETE("/:id", h.DeleteChart)
}
