package fy

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	admin := rg.Group("/admin")
	{
		// --- User Access ---
		admin.GET("/get-fys", h.GetFinancialYears)
		admin.GET("/get-quarters/:financial_year_id", h.GetFinancialQuarters)
		admin.GET("/get-fy/:financial_year_id", h.GetFinancialYearByID)

		// --- Admin-Only Access ---
		restricted := admin.Group("/")
		restricted.Use(middleware.RequireRole("ADMIN"))
		{
			restricted.POST("/create-fy", h.CreateFY)
			restricted.PUT("/update-fy/:financial_year_id", h.UpdateFY)
			restricted.PATCH("/activate-fy/:financial_year_id", h.ActivateFY)
		}
	}
}
