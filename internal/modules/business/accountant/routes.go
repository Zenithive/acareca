package accountant

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	accountant := r.Group("/accountant")
	{
		accountant.GET("/", h.ListUsers)
	}
}
