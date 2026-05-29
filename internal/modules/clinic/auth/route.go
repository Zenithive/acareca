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
		invoiceAuth.GET("/verify-email", h.VerifyEmail)
		invoiceAuth.POST("/forgot-password", h.ForgotPassword)
		invoiceAuth.POST("/reset-password", h.ResetPassword)

		// Protected Routes
		protected := invoiceAuth.Group("", authMiddleware, roleMiddleware, middleware.AuditContext())
		{
			protected.GET("/profile", h.GetProfile)
			protected.PUT("/profile", h.UpdateProfile)
			protected.PUT("/change-password", h.ChangePassword)
			protected.POST("/logout", h.Logout)
			protected.DELETE("", h.DeleteClinic)
		}
	}
}
