package template

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, authMiddleware gin.HandlerFunc, roleMiddleware gin.HandlerFunc) {
	rg.POST("/templates/sync-defaults", h.BulkSyncDefaultsHandler)

	template := rg.Group("/templates", authMiddleware, roleMiddleware)

	template.POST("", h.Create)
	template.GET("", h.List)
	template.GET("/:id", h.Get)
	template.PUT("/:id", h.Update)
	template.DELETE("/:id", h.Delete)

	template.PUT("/:id/settings", h.UpdateSetting)

	template.POST("/:id/preview-pdf", h.GeneratePDF)

	template.GET("/invoice-settings", h.GetInvoiceSetting)
	template.GET("/invoices/:invoice_id/download", h.DownloadPDF)

}
