package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, authMiddleware gin.HandlerFunc) {
	auth := rg.Group("/auth")

	auth.POST("/register", h.Register)

	auth.POST("/login", h.Login)
	// auth.POST("/logout", h.Logout)

	auth.GET("/google", h.GoogleAuthURL)
	auth.GET("/google/callback", h.GoogleCallback)

	auth.GET("/verify", h.VerifyEmail)

	auth.POST("/forgot-password", h.ForgotPassword)
	auth.POST("/reset-password", h.ResetPassword)

	// Protected Routes
	protected := auth.Group("/user", authMiddleware, middleware.AuditContext())
	{
		protected.GET("/profile", h.GetProfile)
		protected.PUT("/profile", h.UpdateProfile)
		protected.PUT("/change-password", h.ChangePassword)
		protected.POST("/logout", h.Logout)
		protected.DELETE("", h.DeleteUser)
	}

}
