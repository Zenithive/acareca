package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
)

func RegisterRoutes(r *gin.RouterGroup, h IHandler, authMiddleware gin.HandlerFunc, roleMiddleware gin.HandlerFunc) {
	invoiceAuth := r.Group("/clinic")
	{
		// Public Endpoints
		invoiceAuth.POST("/register", h.Register)
		invoiceAuth.POST("/login", h.Login)
		invoiceAuth.GET("/verify", h.VerifyEmail)

		// Protected Routes
		protected := invoiceAuth.Group("", authMiddleware, roleMiddleware, middleware.AuditContext())
		{
			protected.GET("/profile", h.GetProfile)
			protected.POST("/logout", h.Logout)
		}
	}
}
