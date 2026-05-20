package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, authMiddleware gin.HandlerFunc) {
	auth := rg.Group("/auth")
	{
		// Public Authentication Routes
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)

		// Google Authentication Routes
		auth.GET("/google", h.GoogleAuthURL)
		auth.GET("/google/callback", h.GoogleCallback)

		// Verification & Recovery Flow Routes
		auth.GET("/verify", h.VerifyEmail)
		auth.POST("/forgot-password", h.ForgotPassword)
		auth.POST("/reset-password", h.ResetPassword)
	}

	// Protected Routes
	protected := auth.Group("/user", authMiddleware, middleware.AuditContext())
	{
		// User Identity & Profile Management Routes
		protected.GET("/profile", h.GetProfile)
		protected.PUT("/profile", h.UpdateProfile)
		protected.PUT("/change-password", h.ChangePassword)
		protected.POST("/logout", h.Logout)
		protected.DELETE("", h.DeleteUser)
	}

}
