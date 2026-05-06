package file

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
)

// RegisterRoutes registers file upload routes
func RegisterRoutes(rg *gin.RouterGroup, h Handler, authMiddleware gin.HandlerFunc) {
	files := rg.Group("/files", authMiddleware, middleware.AuditContext())
	{
		// Direct upload endpoint (for small files)
		files.POST("/upload", h.UploadFile)

		// Presigned URL endpoints (for large files or direct client-to-storage upload)
		files.POST("/presigned-upload", h.GeneratePresignedUploadURL)
		files.POST("/:id/confirm", h.ConfirmUpload)

		// Document management
		files.GET("", h.ListDocuments)
		files.GET("/:id", h.GetDocument)
		files.GET("/:id/download", h.DownloadFile)
		files.PUT("/:id", h.UpdateDocument)
		files.DELETE("/:id", h.DeleteDocument)

		// Entity-specific listing
		files.GET("/entity/:entity_type/:entity_id", h.ListDocumentsByEntity)
	}
}
