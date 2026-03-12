package fy

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler) {
	admin := rg.Group("/admin")
	{
		admin.POST("/create-fy", h.CreateFY)
		admin.PUT("/update-fy/:financial_year_id", h.UpdateFYLabel)
		admin.GET("/get-fys", h.GetFinancialYears)
		admin.GET("/get-quarters/:financial_year_id", h.GetFinancialQuarters)
	}
}
