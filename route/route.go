package route

import (
	"context"
	"log"

	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	adminSubscription "github.com/iamarpitzala/acareca/internal/modules/admin/subscription"
	"github.com/iamarpitzala/acareca/internal/modules/auth"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/business/admin"
	"github.com/iamarpitzala/acareca/internal/modules/business/clinic"
	"github.com/iamarpitzala/acareca/internal/modules/business/coa"
	"github.com/iamarpitzala/acareca/internal/modules/business/equity"
	"github.com/iamarpitzala/acareca/internal/modules/business/fy"
	"github.com/iamarpitzala/acareca/internal/modules/business/invitation"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	"github.com/iamarpitzala/acareca/internal/modules/business/setting"
	"github.com/iamarpitzala/acareca/internal/modules/business/shared/events"
	userSubscription "github.com/iamarpitzala/acareca/internal/modules/business/subscription"
	"github.com/iamarpitzala/acareca/internal/modules/engine/bas"
	"github.com/iamarpitzala/acareca/internal/modules/engine/bs"
	"github.com/iamarpitzala/acareca/internal/modules/engine/pl"
	"github.com/iamarpitzala/acareca/internal/modules/notification"
	"github.com/iamarpitzala/acareca/internal/shared/db"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
	sharedstripe "github.com/iamarpitzala/acareca/internal/shared/stripe"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/stripe/stripe-go/v82"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

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

	stripeClient := sharedstripe.NewStripeClient()

	v1 := r.Group("/api/v1")

	dbConn, err := db.DBConn(cfg)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	// ============ SHARED/CROSS-MODULE SERVICES ============
	authRepo := auth.NewRepository(dbConn)

	// notification (in-app list)
	notificationRepo := notification.NewRepository(dbConn)
	notifier := sharednotification.NewNotifier(dbConn)
	notificationSvc := notification.NewService(notificationRepo, notifier)

	// Initialize audit service (used across modules)
	auditRepo := audit.NewRepository(dbConn)
	auditSvc := audit.NewService(auditRepo, notificationSvc)

	// invitation (cross-module dependency)
	invitationRepo := invitation.NewRepository(dbConn)
	invitationSvc := invitation.NewService(invitationRepo, cfg, notificationSvc, auditSvc, dbConn)
	invitationHandler := invitation.NewHandler(invitationSvc, accountant.NewRepository(dbConn))

	// Create permission adapter for feature-based permissions
	// Wrap the service methods to convert *Permissions to FeaturePermissions interface
	permAdapter := middleware.NewPermissionAdapterFromFuncs(
		func(ctx context.Context, accountantID uuid.UUID, practitionerID uuid.UUID) (middleware.FeaturePermissions, error) {
			perms, err := invitationSvc.GetPermissionsForAccountant(ctx, accountantID, practitionerID)
			if err != nil || perms == nil {
				return nil, err
			}
			return nil, nil // *Permissions implements FeaturePermissions
		},
		invitationSvc.IsAccountantLinkedToPractitioner,
	)

	// invite api
	invite := v1.Group("/invite")
	invite.POST("/process", invitationHandler.ProcessInvitation)
	invite.GET("/:id", invitationHandler.GetInvitation)
	invite.Use(middleware.Auth(cfg))
	invitation.RegisterRoutes(invite, invitationHandler)

	// ============ ADMIN AUTH ============
	adminRepo := admin.NewRepository(dbConn)
	adminSvc := admin.NewService(adminRepo, dbConn)
	adminHandler := admin.NewHandler(adminSvc)
	admin.RegisterRoutes(v1, adminHandler, cfg)

	// Initialize events service first
	eventsRepo := events.NewRepository(dbConn)
	eventsSvc := events.NewService(eventsRepo, notificationSvc, auditSvc)

	// ============ CLINIC SERVICE (cross-module dependency) ============
	clinicRepo := clinic.NewRepository(dbConn)
	clinicSvc := clinic.NewService(dbConn, clinicRepo, accountant.NewRepository(dbConn), authRepo, auditSvc, eventsSvc)
	clinicHandler := clinic.NewHandler(clinicSvc)
	clinic.RegisterRoutes(v1, clinicHandler, cfg, permAdapter)

	// ============ COA SERVICE (cross-module dependency) ============
	coaRepo := coa.NewRepository(dbConn)
	coaSvc := coa.NewService(coaRepo, dbConn, auditSvc)
	coaHandler := coa.NewHandler(coaSvc)
	coa.RegisterRoutes(v1.Group("/coa"), coaHandler, cfg, permAdapter)

	// ============ PRACTITIONER SERVICE (cross-module dependency) ============
	practitionerRepo := practitioner.NewRepository(dbConn)

	userSubscriptionRepo := userSubscription.NewRepository(dbConn)
	userSubscriptionSvc := userSubscription.NewService(userSubscriptionRepo)

	// ============ AUTH SERVICE (depends on practitioner, accountant, admin) ============
	// Initialize practitioner and accountant services for auth
	accountantRepo := accountant.NewRepository(dbConn)
	accountantSvc := accountant.NewService(accountantRepo)

	// Temporarily create practitioner service for auth (will be recreated in RegisterPractitionerRoutes)
	adminSubscriptionRepo := adminSubscription.NewRepository(dbConn)
	adminSubscriptionSvc := adminSubscription.NewService(dbConn, adminSubscriptionRepo, auditSvc, stripeClient)
	practitionerSvc := practitioner.NewService(practitionerRepo, adminSubscriptionSvc, userSubscriptionSvc, coaRepo, invitationRepo)

	authSvc := auth.NewService(authRepo, cfg, dbConn, practitionerSvc, auditSvc, invitationSvc, practitionerRepo, accountantSvc, adminSvc, invitationRepo)
	authHandler := auth.NewHandler(authSvc)
	auth.RegisterRoutes(v1, authHandler, middleware.Auth(cfg))

	// ============ FY MODULE ============
	fyRepo := fy.NewRepository(dbConn)
	fySvc := fy.NewService(fyRepo, dbConn, auditSvc)
	fyHandler := fy.NewHandler(fySvc)
	fyGroup := v1.Group("/", middleware.Auth(cfg))
	fy.RegisterRoutes(fyGroup, fyHandler)

	// ============ ENGINE MODULES (P&L, BAS, Balance Sheet) ============
	plRepo := pl.NewRepository(dbConn)
	plSvc := pl.NewService(plRepo, clinicRepo, accountantRepo, practitionerSvc)
	plHandler := pl.NewHandler(plSvc)
	pl.RegisterRoutes(v1, plHandler, cfg, permAdapter)

	basRepo := bas.NewRepository(dbConn)
	basSvc := bas.NewService(basRepo, accountantRepo, auditSvc, clinicRepo, fyRepo, eventsSvc, authRepo)
	basHandler := bas.NewHandler(basSvc, invitationSvc)
	bas.RegisterRoutes(v1, basHandler, cfg)

	// Equity service for automatic owner fund calculations
	equitySvc := equity.NewService(dbConn, fyRepo)
	equityHandler := equity.NewHandler(equitySvc)
	equity.RegisterRoutes(v1, equityHandler, cfg)

	bsRepo := bs.NewRepository(dbConn)
	bsSvc := bs.NewService(bsRepo, equitySvc)
	bsHandler := bs.NewHandler(bsSvc)
	bs.RegisterRoutes(v1, bsHandler, cfg)

	// ============ SETTING MODULE ============
	settingGroup := v1.Group("/setting")
	settingRepo := setting.NewRepository(dbConn)
	settingSvc := setting.NewService(dbConn, settingRepo, auditSvc)
	settingHandler := setting.NewHandler(settingSvc)
	setting.RegisterRoutes(settingGroup, settingHandler, cfg)

	// ============ MODULE-SPECIFIC ROUTES ============
	// Register admin routes
	RegisterAdminRoutes(v1, cfg, dbConn, auditSvc, stripeClient)

	// Register practitioner routes
	RegisterPractitionerRoutes(v1, cfg, practitionerSvc)

	// Register accountant routes
	RegisterAccountantRoutes(v1, cfg, accountantSvc)

	RegisterBuilderRoutes(v1, cfg, dbConn, clinicSvc, coaSvc, practitionerSvc, accountantRepo, authRepo, auditSvc, eventsSvc, invitationSvc)
	// ============ USER SUBSCRIPTION ============
	userSubscriptionHandler := userSubscription.NewHandler(userSubscriptionSvc, dbConn)
	userSubscriptionGroup := v1.Group("/practitioner/subscription", middleware.Auth(cfg))
	userSubscription.RegisterRoutes(userSubscriptionGroup, userSubscriptionHandler)

	// ============ NOTIFICATION ============
	notificationHandler := notification.NewHandler(notificationSvc)
	nft := v1.Group("/notification")
	nft.GET("/ws", notifier.ServeWS(cfg))
	nft.Use(middleware.Auth(cfg))
	notification.RegisterRoutes(nft, notificationHandler)

	// ============ BILLING MODULE ============
	RegisterBillingRoutes(r, v1, cfg, dbConn, practitionerRepo, userSubscriptionRepo, stripeClient, auditSvc)

	return auditSvc, notifier, notificationRepo

}
