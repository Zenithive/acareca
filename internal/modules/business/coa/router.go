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

	// Chart of Accounts group - requires authentication only
	accounts := rg.Group("/chart-of-account")
	accounts.Use(middleware.Auth(cfg), middleware.AuditContext(), middleware.SetPractitionerIDFromAuth())
	
	// All operations - authentication required, no granular permissions
	{
		accounts.GET("", h.ListChartOfAccount)
		accounts.GET("/by-key/:key", h.GetChartOfAccountByKey)
		accounts.GET("/:id", h.GetChartOfAccount)
		accounts.POST("/check-code", h.CheckCodeUnique)
		accounts.POST("", h.CreateChartOfAccount)
		accounts.PUT("/:id", h.UpdateCharOfAccount)
		accounts.DELETE("/:id", h.DeleteChartOfAccount)
	}
}
