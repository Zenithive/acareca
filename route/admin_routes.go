package route

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/modules/admin/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/admin/analytics"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	adminPractitioner "github.com/iamarpitzala/acareca/internal/modules/admin/practitioner"
	"github.com/iamarpitzala/acareca/internal/modules/admin/subscription"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	sharedstripe "github.com/iamarpitzala/acareca/internal/shared/stripe"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
)

func RegisterAdminRoutes(v1 *gin.RouterGroup, cfg *config.Config, dbConn *sqlx.DB, auditSvc audit.Service, stripeClient sharedstripe.StripeClient) {
	adminGroup := v1.Group("/admin")
	adminGroup.Use(middleware.Auth(cfg), middleware.RequireRole("ADMIN"))

	// Subscription routes - initialize subscription-specific services
	subscriptionGroup := adminGroup.Group("/subscription")
	subscriptionRepo := subscription.NewRepository(dbConn)
	subscriptionHandler := subscription.NewHandler(subscription.NewService(dbConn, subscriptionRepo, auditSvc, stripeClient))
	subscription.RegisterRoutes(subscriptionGroup, subscriptionHandler)

	// Audit routes
	auditGroup := adminGroup.Group("/audit")
	auditHandler := audit.NewHandler(auditSvc)
	audit.RegisterRoutes(auditGroup, auditHandler)

	// Practitioner routes - initialize admin practitioner-specific services
	practitionerGroup := adminGroup.Group("/practitioner")
	adminPractitionerRepo := adminPractitioner.NewRepository(dbConn)
	adminPractitionerSvc := adminPractitioner.NewService(adminPractitionerRepo)
	adminPractitionerHandler := adminPractitioner.NewHandler(adminPractitionerSvc)
	adminPractitioner.RegisterRoutes(practitionerGroup, adminPractitionerHandler)

	// Accountant routes
	accountantGroup := adminGroup.Group("/accountant")
	accountant.RegisterRoutes(accountantGroup, dbConn)

	// Analytics routes - initialize analytics-specific services
	analyticsGroup := adminGroup.Group("/analytics")
	analyticsRepo := analytics.NewRepository(dbConn)
	analyticsSvc := analytics.NewService(analyticsRepo)
	analyticsHandler := analytics.NewHandler(analyticsSvc)
	analytics.RegisterRoutes(analyticsGroup, analyticsHandler)
}
