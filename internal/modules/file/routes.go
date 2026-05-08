package file

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
)

// RegisterRoutes registers file upload routes
func RegisterRoutes(rg *gin.RouterGroup, h Handler, authMiddleware gin.HandlerFunc) {
	files := rg.Group("/files", authMiddleware, middleware.AuditContext())
	{
		files.POST("/upload", h.Upload)
		files.GET("", h.ListDocument)
		files.GET("/:id", h.GetDocument)
		files.PUT("/:id", h.UpdateDocument)
		files.DELETE("/:id", h.DeleteDocument)
	}
}
