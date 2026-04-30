package accountant

import (
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func RegisterRoutes(rg *gin.RouterGroup, db *sqlx.DB) {
	repo := NewRepository(db)
	svc := NewService(repo)
	handler := NewHandler(svc)

	rg.GET("", handler.ListAccountantsWithPractitioners)
}
