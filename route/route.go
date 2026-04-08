package route

import (
	"context"
	"log"

	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	adminAccountant "github.com/iamarpitzala/acareca/internal/modules/admin/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	adminPractitioner "github.com/iamarpitzala/acareca/internal/modules/admin/practitioner"
	"github.com/iamarpitzala/acareca/internal/modules/admin/subscription"
	"github.com/iamarpitzala/acareca/internal/modules/auth"
	"github.com/iamarpitzala/acareca/internal/modules/builder/detail"
	"github.com/iamarpitzala/acareca/internal/modules/builder/entry"
	"github.com/iamarpitzala/acareca/internal/modules/builder/field"
	"github.com/iamarpitzala/acareca/internal/modules/builder/form"
	"github.com/iamarpitzala/acareca/internal/modules/builder/version"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/business/admin"
	"github.com/iamarpitzala/acareca/internal/modules/business/billing"
	"github.com/iamarpitzala/acareca/internal/modules/business/clinic"
	"github.com/iamarpitzala/acareca/internal/modules/business/coa"
	"github.com/iamarpitzala/acareca/internal/modules/business/fy"
	"github.com/iamarpitzala/acareca/internal/modules/business/invitation"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	"github.com/iamarpitzala/acareca/internal/modules/business/setting"
	"github.com/iamarpitzala/acareca/internal/modules/business/shared/events"
	userSubscription "github.com/iamarpitzala/acareca/internal/modules/business/subscription"
	"github.com/iamarpitzala/acareca/internal/modules/engine/bas"
	"github.com/iamarpitzala/acareca/internal/modules/engine/calculation"
	"github.com/iamarpitzala/acareca/internal/modules/engine/formula"
	"github.com/iamarpitzala/acareca/internal/modules/engine/method"
	"github.com/iamarpitzala/acareca/internal/modules/engine/pl"
	"github.com/iamarpitzala/acareca/internal/modules/notification"
	"github.com/iamarpitzala/acareca/internal/modules/seed"
	"github.com/iamarpitzala/acareca/internal/shared/db"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
	sharedstripe "github.com/iamarpitzala/acareca/internal/shared/stripe"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/stripe/stripe-go/v82"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// permissionAdapter adapts invitation.Service to middleware.PermissionChecker
type permissionAdapter struct {
	invSvc invitation.Service
}

func (a *permissionAdapter) ListAccountantPermission(ctx context.Context, accId uuid.UUID) ([]middleware.PermissionItem, error) {
	perms, _, err := a.invSvc.ListAccountantPermission(ctx, accId)
	if err != nil {
		return nil, err
	}
	
	// Convert []invitation.Permissions to []middleware.PermissionItem
	result := make([]middleware.PermissionItem, 0, len(*perms))
	for i := range *perms {
		result = append(result, &(*perms)[i])
	}
	return result, nil
}

func RegisterRoutes(r *gin.Engine, cfg *config.Config) (audit.Service, *sharednotification.Hub, notification.Repository) {

	// Initialize Stripe SDK
	if cfg.StripeSecretKey == "" {
		log.Fatal("STRIPE_SECRET_KEY is required but not set")
	}
	stripe.Key = cfg.StripeSecretKey

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Register webhook route BEFORE v1 group and any JSON middleware
	// (Stripe signature verification requires the raw request body)
	stripeClient := sharedstripe.NewStripeClient()

	v1 := r.Group("/api/v1")

	dbConn, err := db.DBConn(cfg)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	authRepo := auth.NewRepository(dbConn)
	subscriptionRepo := subscription.NewRepository(dbConn)
	practitionerRepo := practitioner.NewRepository(dbConn)
	userSubscriptionRepo := userSubscription.NewRepository(dbConn)
	coaRepo := coa.NewRepository(dbConn)

	// Initialize audit service
	auditRepo := audit.NewRepository(dbConn)
	auditSvc := audit.NewService(auditRepo)

	subscriptionSvc := subscription.NewService(dbConn, subscriptionRepo, auditSvc, stripeClient)
	userSubscriptionSvc := userSubscription.NewService(userSubscriptionRepo)
	practitionerSvc := practitioner.NewService(practitionerRepo, subscriptionSvc, userSubscriptionSvc, coaRepo)
	accountantRepo := accountant.NewRepository(dbConn)
	accountantSvc := accountant.NewService(accountantRepo)

	// notification (in-app list)
	notificationRepo := notification.NewRepository(dbConn)

	notifier := sharednotification.NewNotifier(dbConn)

	notificationSvc := notification.NewService(notificationRepo, notifier)

	// invitation
	invitationRepo := invitation.NewRepository(dbConn)
	invitationSvc := invitation.NewService(invitationRepo, cfg, notificationSvc, auditSvc)
	invitationHandler := invitation.NewHandler(invitationSvc, accountantRepo)

	//invite api
	invite := v1.Group("/invite")
	// Public Route (These endpoints are for the person receiving the invite)
	invite.POST("/process", invitationHandler.ProcessInvitation)
	invite.GET("/:id", invitationHandler.GetInvitation)
	// Protected Route
	invite.Use(middleware.Auth(cfg))
	invitation.RegisterRoutes(invite, invitationHandler)

	//admin auth
	adminRepo := admin.NewRepository(dbConn)
	adminSvc := admin.NewService(adminRepo, dbConn)
	adminHandler := admin.NewHandler(adminSvc)
	admin.RegisterRoutes(v1, adminHandler, cfg)

	authSvc := auth.NewService(authRepo, cfg, dbConn, practitionerSvc, auditSvc, invitationSvc, practitionerRepo, accountantSvc, adminSvc, invitationRepo)
	authHandler := auth.NewHandler(authSvc)
	auth.RegisterRoutes(v1, authHandler, middleware.Auth(cfg))


	adminGroup := v1.Group("/admin")
	subscriptionGroup := adminGroup.Group("/subscription")
	
	subscriptionHandler := subscription.NewHandler(subscriptionSvc)
	subscription.RegisterRoutes(subscriptionGroup, subscriptionHandler)

	// Audit routes
	auditGroup := adminGroup.Group("/audit")
	// auditGroup.Use(middleware.Auth(cfg), middleware.RequireSuperadmin(func(ctx context.Context, userID string) (bool, error) {
	// 	id, err := util.ParseUUID(userID)
	// 	if err != nil {
	// 		return false, err
	// 	}
	// 	return superadminCheck(ctx, id)
	// }))
	auditHandler := audit.NewHandler(auditSvc)
	audit.RegisterRoutes(auditGroup, auditHandler)

	// Admin Practitioner routes
	practitionerGroup := adminGroup.Group("/practitioner")
	adminPractitionerRepo := adminPractitioner.NewRepository(dbConn)
	adminPractitionerSvc := adminPractitioner.NewService(adminPractitionerRepo)
	adminPractitionerHandler := adminPractitioner.NewHandler(adminPractitionerSvc)
	adminPractitioner.RegisterRoutes(practitionerGroup, adminPractitionerHandler)

	// Admin Accountant routes
	accountantGroup := adminGroup.Group("/accountant")
	adminAccountant.RegisterRoutes(accountantGroup, dbConn)

	// Initialize events service first
	eventsRepo := events.NewRepository(dbConn)
	eventsSvc := events.NewService(eventsRepo, notificationSvc, auditSvc)

	// clinic
	clinicRepo := clinic.NewRepository(dbConn)
	clinicSvc := clinic.NewService(dbConn, clinicRepo, accountantRepo, authRepo, auditSvc, eventsSvc)
	clinicHandler := clinic.NewHandler(clinicSvc)
	clinic.RegisterRoutes(v1, clinicHandler, cfg)

	coaSvc := coa.NewService(coaRepo, dbConn, auditSvc)
	coaHandler := coa.NewHandler(coaSvc)
	coa.RegisterRoutes(v1.Group("/coa"), coaHandler, cfg)

	fyRepo := fy.NewRepository(dbConn)
	fySvc := fy.NewService(fyRepo, dbConn, auditSvc)
	fyHandler := fy.NewHandler(fySvc)
	fyGroup := v1.Group("/")
	fy.RegisterRoutes(fyGroup, fyHandler)

	formGroup := v1.Group("/form", middleware.Auth(cfg), middleware.AuditContext())

	detailRepo := detail.NewRepository(dbConn)
	versionRepo := version.NewRepository(dbConn)
	fieldRepo := field.NewRepository(dbConn)
	entryRepo := entry.NewRepository(dbConn)
	detailSvc := detail.NewService(dbConn, detailRepo, version.NewService(dbConn, versionRepo, clinicSvc), clinicRepo, invitationSvc)
	fieldSvc := field.NewService(fieldRepo, coaSvc, clinicSvc, practitionerSvc, version.NewService(dbConn, versionRepo, clinicSvc))

	versionSvc := version.NewService(dbConn, versionRepo, clinicSvc)
	formulaRepo := formula.NewRepository(dbConn)
	formulaSvc := formula.NewService(formulaRepo)
	formSvc := form.NewService(dbConn, detailSvc, versionSvc, fieldSvc, formulaSvc, entryRepo, coaSvc, auditSvc, eventsSvc, accountantRepo, authRepo, clinicSvc, invitationSvc)
	formHandler := form.NewHandler(formSvc)
	
	// Apply method-based permission middleware (GET=read, POST=create, PUT/PATCH=update, DELETE=delete)
	permChecker := &permissionAdapter{invSvc: invitationSvc}
	formGroup.Use(middleware.MethodBasedPermission(permChecker))
	form.RegisterRoutes(formGroup, formHandler)

	entryGroup := v1.Group("/entry",middleware.Auth(cfg), middleware.AuditContext())
	entriesRepo := entry.NewRepository(dbConn)
	entriesSvc := entry.NewService(dbConn, entriesRepo, fieldRepo, method.NewService(), detailSvc, versionSvc, auditSvc, eventsSvc, accountantRepo, authRepo, clinicRepo, clinicSvc, formulaSvc, fieldSvc, invitationSvc)
	entriesHandler := entry.NewHandler(entriesSvc)

	// Apply method-based permission middleware (GET=read, POST=create, PUT/PATCH=update, DELETE=delete)
	entryGroup.Use(middleware.MethodBasedPermission(permChecker))
	entry.RegisterRoutes(entryGroup, entriesHandler)

	calculationGroup := v1.Group("")
	calculationGroup.Use(middleware.Auth(cfg))
	calculationSvc := calculation.NewServiceWithFormula(formSvc, versionSvc, fieldSvc, entriesSvc, formulaSvc)
	calculationHandler := calculation.NewHandler(calculationSvc)
	calculation.RegisterRoutes(calculationGroup, calculationHandler)

	// P&L reporting — engine/pl module
	plRepo := pl.NewRepository(dbConn)
	plSvc := pl.NewService(plRepo, clinicRepo, accountantRepo, practitionerSvc)
	plHandler := pl.NewHandler(plSvc)
	pl.RegisterRoutes(v1, plHandler, cfg)

	// BAS reporting — engine/bas module
	basRepo := bas.NewRepository(dbConn)
	basSvc := bas.NewService(basRepo, accountantRepo, auditSvc, clinicRepo)
	basHandler := bas.NewHandler(basSvc, invitationSvc)
	bas.RegisterRoutes(v1, basHandler, cfg)

	settingGroup := v1.Group("/setting")
	settingRepo := setting.NewRepository(dbConn)
	settingSvc := setting.NewService(dbConn, settingRepo, auditSvc)
	settingHandler := setting.NewHandler(settingSvc)

	setting.RegisterRoutes(settingGroup, settingHandler, cfg)

	practitionerHandler := practitioner.NewHandler(practitionerSvc)
	practitioner.RegisterRoutes(v1, practitionerHandler, cfg)

	userSubscriptionHandler := userSubscription.NewHandler(userSubscriptionSvc, dbConn)

	userSubscriptionGroup := v1.Group("/practitioner/subscription",middleware.Auth(cfg))

	userSubscription.RegisterRoutes(userSubscriptionGroup, userSubscriptionHandler)

	// notification (in-app list + WebSocket)
	notificationHandler := notification.NewHandler(notificationSvc)
	nft := v1.Group("/notification")

	// WebSocket — auth via ?token= query param
	nft.GET("/ws", notifier.ServeWS(cfg))
	nft.Use(middleware.Auth(cfg))

	notification.RegisterRoutes(nft, notificationHandler)

	accountantHandler := accountant.NewHandler(accountantSvc)

	accountant.RegisterRoutes(v1, accountantHandler, middleware.Auth(cfg))

	// Billing module
	billingRepo := billing.NewRepository(dbConn)
	billingSvc := billing.NewService(billingRepo, practitionerRepo, userSubscriptionRepo, stripeClient)
	billingHandler := billing.NewHandler(billingSvc)

	// Webhook route MUST be registered before any JSON body-parsing middleware
	billing.RegisterWebhookRoute(r.Group("/api/v1/webhooks"), billingHandler)
	billing.RegisterRoutes(v1, billingHandler, cfg)

	// Seed module (for development/testing)
	seedSvc := seed.NewService(dbConn)
	seedHandler := seed.NewHandler(seedSvc)
	seed.RegisterRoutes(v1, seedHandler)

	return auditSvc, notifier, notificationRepo

}
