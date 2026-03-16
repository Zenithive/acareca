package coa

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, cfg *config.Config) {
	rg.GET("/account-types", h.ListAccountTypes)
	rg.GET("/account-types/:id", h.GetAccountType)
	rg.GET("/account-taxes", h.ListAccountTaxes)
	rg.GET("/account-taxes/:id", h.GetAccountTax)

	// Chart of Accounts CRUD — scoped by practitioner_id
	accounts := rg.Group("/chart-of-account")
	accounts.Use(middleware.Auth(cfg))
	accounts.GET("", h.ListChartOfAccount)
	accounts.GET("/:id", h.GetChartOfAccount)
	accounts.POST("", h.CreateChartOfAccount)
	accounts.PUT("/:id", h.UpdateCharOfAccount)
	accounts.DELETE("/:id", h.DeleteChartOfAccount)
}
