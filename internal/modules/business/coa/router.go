package coa

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, cfg *config.Config, permAdapter *middleware.PermissionAdapter) {
	// Public reference data - no auth required
	rg.GET("/account-types", h.ListAccountTypes)
	rg.GET("/account-types/:id", h.GetAccountType)
	rg.GET("/account-taxes", h.ListAccountTaxes)
	rg.GET("/account-taxes/:id", h.GetAccountTax)

	// Chart of Accounts group - requires sales_purchases permission
	accounts := rg.Group("/chart-of-account")
	accounts.Use(middleware.Auth(cfg), middleware.AuditContext(), middleware.SetPractitionerIDFromAuth())
	
	// Read operations
	accountsRead := accounts.Group("")
	accountsRead.Use(middleware.RequireFeaturePermission(permAdapter, middleware.FeatureSalesPurchases, middleware.ActionRead))
	{
		accountsRead.GET("", h.ListChartOfAccount)
		accountsRead.GET("/by-key/:key", h.GetChartOfAccountByKey)
		accountsRead.GET("/:id", h.GetChartOfAccount)
		accountsRead.POST("/check-code", h.CheckCodeUnique)
	}
	
	// Write operations
	accountsWrite := accounts.Group("")
	accountsWrite.Use(middleware.RequireFeaturePermission(permAdapter, middleware.FeatureSalesPurchases, middleware.ActionWrite))
	{
		accountsWrite.POST("", h.CreateChartOfAccount)
		accountsWrite.PUT("/:id", h.UpdateCharOfAccount)
		accountsWrite.DELETE("/:id", h.DeleteChartOfAccount)
	}
}
