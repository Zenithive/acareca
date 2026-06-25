package route

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/business/admin"
	"github.com/iamarpitzala/acareca/internal/modules/business/billing"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	userSubscription "github.com/iamarpitzala/acareca/internal/modules/business/subscription"
	"github.com/iamarpitzala/acareca/internal/modules/notification"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
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
	notificationSvc notification.Service,
	adminRepo admin.Repository,
) {

	billingRepo := billing.NewRepository(dbConn)

	var pub *sharednotification.Publisher
	if notificationSvc != nil {
		pub = sharednotification.NewPublisher(notification.NewServiceAdapter(notificationSvc), nil)
	}

	billingSvc := billing.NewService(billingRepo, practitionerRepo, uSubRepo, stripeClient, auditSvc, pub, adminRepo)

	billingHandler := billing.NewHandler(billingSvc)

	billing.RegisterWebhookRoute(r.Group("/api/v1/webhooks"), billingHandler)

	billing.RegisterRoutes(v1, billingHandler, cfg)
}
