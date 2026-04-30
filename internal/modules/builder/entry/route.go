package entry

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, permAdapter *middleware.PermissionAdapter) {
	// All entry operations - no permission checks
	
	// Version-based routes
	versionGroup := rg.Group("/version/:version_id")
	{
		versionGroup.GET("", h.List)
		versionGroup.POST("", h.Create)
	}

	// Transaction routes
	rg.GET("/transactions", h.ListTransactions)

	// COA-grouped routes
	coaGroup := rg.Group("/coa-entries")
	{
		coaGroup.GET("", h.ListCoaEntries)
		coaGroup.GET("/:coa_id/entries", h.ListCoaEntryDetails)
		coaGroup.GET("/export", h.HandleExport)
	}

	// ID-based routes
	idGroup := rg.Group("/:id")
	{
		idGroup.GET("", h.Get)
		idGroup.PATCH("", h.Update)
		idGroup.DELETE("", h.Delete)
	}
}
