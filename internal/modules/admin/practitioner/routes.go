package practitioner

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.GET("", h.ListPractitionersWithSubscriptions)
	rg.GET("/:id", h.GetPractitionerWithSubscription)
}
