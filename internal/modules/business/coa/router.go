package coa

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	rg.GET("/account-types", h.ListAccountTypes)
	rg.GET("/account-types/:id", h.GetAccountTypeByID)
	rg.GET("/account-taxes", h.ListAccountTaxes)
	rg.GET("/account-taxes/:id", h.GetAccountTaxByID)

	// Chart of Accounts CRUD (4 routes: Create, Read, Update, Delete)
	accounts := rg.Group("/clinic/:clinicId/accounts")
	accounts.GET("", h.ListChartByClinic)     // Read (list)
	accounts.GET("/:id", h.GetChartByID)      // Read (one)
	accounts.POST("", h.CreateChart)           // Create
	accounts.PUT("/:id", h.UpdateChart)        // Update
	accounts.DELETE("/:id", h.DeleteChart)    // Delete
}
