package accountant

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.RouterGroup, h *Handler, authMiddleware gin.HandlerFunc) {
	accountant := r.Group("/accountant", authMiddleware)
	{
		accountant.Use(authMiddleware)

		accountant.GET("/", h.ListUsers)
		accountant.GET("/clinics", h.ListClinics)
		accountant.GET("/forms", h.ListForms)
	}
}
