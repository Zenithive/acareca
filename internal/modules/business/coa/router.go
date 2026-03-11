package coa

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	rg.GET("/account-types", h.ListAccountTypes)
	rg.GET("/account-types/:id", h.GetAccountTypeByID)
	rg.GET("/account-taxes", h.ListAccountTaxes)
	rg.GET("/account-taxes/:id", h.GetAccountTaxByID)

	// Chart of Accounts CRUD — scoped by practice_id (practitioner id)
	accounts := rg.Group("/created-by/:practice_idId/accounts")
	accounts.GET("", h.ListChartsBypractice_id)
	accounts.GET("/:id", h.GetChartByIDAndpractice_id)
	accounts.POST("", h.CreateChart)
	accounts.PUT("/:id", h.UpdateChart)
	accounts.DELETE("/:id", h.DeleteChart)
}
