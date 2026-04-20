package fy

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	admin := rg.Group("/admin")
	//admin.Use(middleware.RequireSuperadmin())
	{
		// --- User Access ---
		admin.GET("/get-fys", h.GetFinancialYears)
		admin.GET("/get-quarters/:financial_year_id", h.GetFinancialQuarters)

		// --- Admin-Only Access ---
		restricted := admin.Group("/")
		//restricted.Use(middleware.RequireRole("ADMIN"))
		{
			restricted.POST("/create-fy", h.CreateFY)
			restricted.PUT("/update-fy/:financial_year_id", h.UpdateFYLabel)
			restricted.PATCH("/activate-fy/:financial_year_id", h.ActivateFY)
		}
	}
}
