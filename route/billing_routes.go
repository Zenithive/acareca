package route

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/business/billing"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	userSubscription "github.com/iamarpitzala/acareca/internal/modules/business/subscription"
	sharedstripe "github.com/iamarpitzala/acareca/internal/shared/stripe"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
)

func RegisterBillingRoutes(
	r *gin.Engine,
	v1 *gin.RouterGroup,
	cfg *config.Config,
	dbConn *sqlx.DB,
	practitionerRepo practitioner.Repository,
	uSubRepo userSubscription.Repository,
	stripeClient sharedstripe.StripeClient,
	auditSvc audit.Service,
) {

	billingRepo := billing.NewRepository(dbConn)

	billingSvc := billing.NewService(billingRepo, practitionerRepo, uSubRepo, stripeClient, auditSvc)

	billingHandler := billing.NewHandler(billingSvc)

	billing.RegisterWebhookRoute(r.Group("/api/v1/webhooks"), billingHandler)

	billing.RegisterRoutes(v1, billingHandler, cfg)
}
