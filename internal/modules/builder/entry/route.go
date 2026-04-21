package entry

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, permAdapter *middleware.PermissionAdapter) {
	// Entries require sales_purchases permission

	// Version-based routes - Read operations
	versionRead := rg.Group("/version/:version_id")
	versionRead.Use(middleware.RequireFeaturePermission(permAdapter, middleware.FeatureSalesPurchases, middleware.ActionRead))
	{
		versionRead.GET("", h.List)
	}

	// Version-based routes - Write operations
	versionWrite := rg.Group("/version/:version_id")
	versionWrite.Use(middleware.RequireFeaturePermission(permAdapter, middleware.FeatureSalesPurchases, middleware.ActionWrite))
	{
		versionWrite.POST("", h.Create)
	}

	// Transaction routes - Read operations
	transactionRead := rg.Group("")
	transactionRead.Use(middleware.RequireFeaturePermission(permAdapter, middleware.FeatureSalesPurchases, middleware.ActionRead))
	{
		transactionRead.GET("/transactions", h.ListTransactions)
	}

	// COA-grouped routes - Read operations
	coaRead := rg.Group("/coa-entries")
	coaRead.Use(middleware.RequireFeaturePermission(permAdapter, middleware.FeatureSalesPurchases, middleware.ActionRead))
	{
		coaRead.GET("", h.ListCoaEntries)
		coaRead.GET("/:coa_id/entries", h.ListCoaEntryDetails)
		coaRead.GET("/export", h.HandleExport)

	}

	// ID-based routes - Read operations
	idRead := rg.Group("/:id")
	idRead.Use(middleware.RequireFeaturePermission(permAdapter, middleware.FeatureSalesPurchases, middleware.ActionRead))
	{
		idRead.GET("", h.Get)
	}

	// ID-based routes - Write operations
	idWrite := rg.Group("/:id")
	idWrite.Use(middleware.RequireFeaturePermission(permAdapter, middleware.FeatureSalesPurchases, middleware.ActionWrite))
	{
		idWrite.PATCH("", h.Update)
		idWrite.DELETE("", h.Delete)
	}
}
