package events

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, handler *Handler) {
	events := r.Group("/api/v1/shared/events")
	{
		events.POST("/record", handler.RecordEvent)
	}
}
