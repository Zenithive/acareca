package coa

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, cfg *config.Config, permChecker middleware.PermissionChecker) {
	rg.GET("/account-types", h.ListAccountTypes)
	rg.GET("/account-types/:id", h.GetAccountType)
	rg.GET("/account-taxes", h.ListAccountTaxes)
	rg.GET("/account-taxes/:id", h.GetAccountTax)

	listGroup := rg.Group("")
	listGroup.Use(middleware.Auth(cfg), middleware.AuditContext())
	listGroup.Use(middleware.MethodBasedPermission(permChecker))
	{
		listGroup.GET("/chart-of-account", h.ListChartOfAccount)
		// Alternative route for GetChartOfAccountByKey without practitioner_id in path
		listGroup.GET("/chart-of-account/by-key/:key", h.GetChartOfAccountByKey)
	}

	// Chart of Accounts CRUD — scoped by practitioner_id
	// Format: /coa/:practitioner_id/chart-of-account
	accounts := rg.Group("/:practitioner_id/chart-of-account")
	accounts.Use(middleware.Auth(cfg), middleware.AuditContext())
	accounts.Use(middleware.MethodBasedPermission(permChecker))
	// accounts.GET("", h.ListChartOfAccount)
	accounts.POST("/check-code", h.CheckCodeUnique)
	accounts.GET("/by-key/:key", h.GetChartOfAccountByKey)
	accounts.GET("/:id", h.GetChartOfAccount)
	accounts.POST("", h.CreateChartOfAccount)
	accounts.PUT("/:id", h.UpdateCharOfAccount)
	accounts.DELETE("/:id", h.DeleteChartOfAccount)
}
