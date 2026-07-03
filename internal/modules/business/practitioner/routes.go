package practitioner

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, cfg *config.Config, db *sqlx.DB) {
	g := rg.Group("/practitioner")
	g.Use(middleware.Auth(cfg), middleware.RequireActiveSubscription(db), middleware.AuditContext())
	g.GET("/lock-date", h.GetLockDate)
	g.PATCH("/lock-date", h.UpdateLockDate)
	g.GET("", h.ListPractitioners)
	g.GET("/:id", h.GetPractitioner)
}
