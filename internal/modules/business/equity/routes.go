package equity

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
)

func RegisterRoutes(rg *gin.RouterGroup, h Handler, cfg *config.Config, db *sqlx.DB) {
	routes := rg.Group("/equity")
	routes.Use(middleware.Auth(cfg), middleware.RequireActiveSubscription(db))
	routes.Use(middleware.SetPractitionerIDFromAuth())
	{
		routes.GET("/calculation", h.GetOwnerEquityCalculation)
		routes.GET("/retained-earnings", h.GetRetainedEarnings)
		routes.GET("/movements", h.GetEquityMovements)
	}
}
