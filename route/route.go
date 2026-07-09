package route

import (
	"context"
	"log"
	"strings"

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
	userSubscription "github.com/iamarpitzala/acareca/internal/modules/business/subscription"
	clinicauth "github.com/iamarpitzala/acareca/internal/modules/clinic/auth"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/contact"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/invoice"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template"
	templateRepository "github.com/iamarpitzala/acareca/internal/modules/clinic/template/repository"
	templateService "github.com/iamarpitzala/acareca/internal/modules/clinic/template/service"
	"github.com/iamarpitzala/acareca/internal/modules/engine/bas"
	"github.com/iamarpitzala/acareca/internal/modules/engine/bs"
	"github.com/iamarpitzala/acareca/internal/modules/engine/pl"
	"github.com/iamarpitzala/acareca/internal/modules/file"
	"github.com/iamarpitzala/acareca/internal/modules/notification"
	"github.com/iamarpitzala/acareca/internal/modules/notification/preference"
	"github.com/iamarpitzala/acareca/internal/modules/worker"
	"github.com/iamarpitzala/acareca/internal/shared/db"
	sharedEvents "github.com/iamarpitzala/acareca/internal/shared/events"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
	sharedstripe "github.com/iamarpitzala/acareca/internal/shared/stripe"
	"github.com/iamarpitzala/acareca/internal/shared/upload"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/stripe/stripe-go/v82"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func RegisterRoutes(r *gin.Engine, cfg *config.Config, events sharedEvents.IEvent) (audit.Service, *sharednotification.Hub, notification.Repository, notification.Service, *worker.Consumer) {

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
	notificationPrefRepo := preference.NewRepository(dbConn)
	notifier := sharednotification.NewNotifier(dbConn)
	notificationSvc := notification.NewService(notificationRepo, events, dbConn, notificationPrefRepo)

	preferenceRepo := preference.NewRepository(dbConn)
	preferenceSvc := preference.NewService(preferenceRepo, dbConn)

	// Initialize audit service (used across modules)
	auditRepo := audit.NewRepository(dbConn)
	auditSvc := audit.NewService(auditRepo, notificationSvc, admin.NewRepository(dbConn))
	// ============ FILE UPLOAD MODULE ============
	fileRepo := file.NewRepository(dbConn)

	// Initialize storage provider (R2)
	storage, err := upload.NewStorageProvider(cfg)
	if err != nil {
		log.Fatalf("failed to initialize storage provider: %v", err)
	}

	// Parse allowed MIME types from config
	allowedTypes := strings.Split(cfg.FileUploadAllowedTypes, ",")
	for i, t := range allowedTypes {
		allowedTypes[i] = strings.TrimSpace(t)
	}

	// Initialize file validator
	fileValidator := upload.NewFileValidator(cfg.FileUploadMaxSize, allowedTypes)

	// ============ ADMIN AUTH ============
	adminRepo := admin.NewRepository(dbConn)

	// invitation (cross-module dependency)
	invitationRepo := invitation.NewRepository(dbConn)
	invitationSvc := invitation.NewService(invitationRepo, cfg, notificationSvc, auditSvc, dbConn, adminRepo)
	invitationHandler := invitation.NewHandler(invitationSvc)

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

	adminSvc := admin.NewService(adminRepo, dbConn)
	adminHandler := admin.NewHandler(adminSvc)
	admin.RegisterRoutes(v1, adminHandler, cfg)

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

	// ============ FY MODULE ============
	fyRepo := fy.NewRepository(dbConn)
	fySvc := fy.NewService(fyRepo, dbConn, auditSvc)
	fyHandler := fy.NewHandler(fySvc)
	fyGroup := v1.Group("/", middleware.Auth(cfg))
	fy.RegisterRoutes(fyGroup, fyHandler)

	// Temporarily create practitioner service for auth (will be recreated in RegisterPractitionerRoutes)
	adminSubscriptionRepo := adminSubscription.NewRepository(dbConn)
	adminSubscriptionSvc := adminSubscription.NewService(dbConn, adminSubscriptionRepo, auditSvc, stripeClient)
	practitionerSvc := practitioner.NewService(dbConn, practitionerRepo, adminSubscriptionSvc, userSubscriptionSvc, coaRepo, auditSvc, fyRepo, invitationRepo)

	authSvc := auth.NewService(authRepo, cfg, dbConn, practitionerSvc, auditSvc, invitationSvc, practitionerRepo, accountantSvc, adminSvc, fileRepo, preferenceSvc)
	authHandler := auth.NewHandler(authSvc)
	auth.RegisterRoutes(v1, authHandler, middleware.Auth(cfg))

	// ============ CLINIC SERVICE (cross-module dependency) ============
	clinicRepo := clinic.NewRepository(dbConn)
	clinicSvc := clinic.NewService(dbConn, clinicRepo, accountant.NewRepository(dbConn), authRepo, fileRepo, auditSvc, notificationSvc, authSvc, invitationRepo, invitationSvc, adminRepo)

	clinicHandler := clinic.NewHandler(clinicSvc)
	clinic.RegisterRoutes(v1, clinicHandler, cfg, permAdapter)

	// ============ ENGINE MODULES (P&L, BAS, Balance Sheet) ============
	plRepo := pl.NewRepository(dbConn)
	plSvc := pl.NewService(plRepo, clinicRepo, accountantRepo, practitionerSvc, authRepo, auditSvc, invitationRepo, authSvc, notificationSvc, adminRepo)
	plHandler := pl.NewHandler(plSvc, invitationSvc, accountantRepo)
	pl.RegisterRoutes(v1, plHandler, cfg, permAdapter)

	basRepo := bas.NewRepository(dbConn)
	basSvc := bas.NewService(basRepo, accountantRepo, auditSvc, clinicRepo, fyRepo, authRepo, practitionerSvc, invitationRepo, authSvc, notificationSvc, adminRepo)
	basHandler := bas.NewHandler(basSvc, invitationSvc)
	bas.RegisterRoutes(v1, basHandler, cfg)

	// Equity service for automatic owner fund calculations
	equitySvc := equity.NewService(dbConn, fyRepo)
	equityHandler := equity.NewHandler(equitySvc)
	equity.RegisterRoutes(v1, equityHandler, cfg)

	bsRepo := bs.NewRepository(dbConn)
	bsSvc := bs.NewService(bsRepo, equitySvc, dbConn, auditSvc, authRepo, invitationSvc, accountantRepo, practitionerSvc, invitationRepo, authSvc, notificationSvc, adminRepo, fySvc)
	bsHandler := bs.NewHandler(bsSvc, invitationSvc)
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

	// Builder routes will be registered after notificationPublisher is initialized

	// ============ USER SUBSCRIPTION ============
	userSubscriptionHandler := userSubscription.NewHandler(userSubscriptionSvc, dbConn)
	userSubscriptionGroup := v1.Group("/practitioner/subscription", middleware.Auth(cfg))
	userSubscription.RegisterRoutes(userSubscriptionGroup, userSubscriptionHandler)

	// ============ NOTIFICATION ============
	notificationHandler := notification.NewHandler(notificationSvc)
	nft := v1.Group("/notification")
	nft.GET("/ws", middleware.Auth(cfg), notifier.ServeWS(cfg))
	nft.Use(middleware.Auth(cfg))
	notification.RegisterRoutes(nft, notificationHandler)

	preferenceHandler := preference.NewHandler(preferenceSvc)
	preference.RegisterRoutes(nft, preferenceHandler)

	// ============ INVOICE MODULE ============
	// Initialize template container with new architecture
	templateContainer, err := template.NewContainer(cfg, dbConn)
	if err != nil {
		log.Fatalf("failed to initialize template container: %v", err)
	}

	// Inject service factory to avoid circular dependency
	templateContainer.SetServiceFactory(func(
		cfg *config.Config,
		templateRepo templateRepository.ITemplateRepository,
		settingRepo templateRepository.ISettingRepository,
	) template.IService {
		return templateService.NewCompositeServiceWithDB(dbConn, cfg, templateRepo, settingRepo)
	})

	tempSvc := templateContainer.Service()
	RegisterInvoiceRoutes(v1, cfg, dbConn, auditSvc, tempSvc, templateContainer)

	clinicAuthRepo := clinicauth.NewRepository(dbConn)
	clinicAuthSvc := clinicauth.NewService(clinicAuthRepo, cfg, dbConn, auditSvc, tempSvc)

	contactSvc := contact.NewService(contact.NewRepository(dbConn))

	invoiceSvc := invoice.NewService(
		dbConn,
		invoice.NewRepository(dbConn),
		cfg,
		tempSvc,
		clinicAuthSvc,
		templateContainer.SettingRepo(),
		templateContainer.TemplateRepo(),
	)
	RegisterClinicRoutes(v1, cfg, contactSvc, invoiceSvc)

	// Initialize notification consumer (separate from service)
	notificationPublisher := worker.NewPublisher(events)
	notificationConsumer := worker.NewConsumer(events, notificationRepo, notifier, dbConn, notificationPublisher, notificationPrefRepo, authSvc)

	// Wire stream manager to WebSocket hub for real-time delivery
	if events != nil {
		streamManager := notificationConsumer.GetStreamManager()
		notifier.SetStreamManager(streamManager)
		log.Println("✅ NATS stream manager connected to WebSocket hub")
	}

	// ============ FILE SERVICE (requires authSvc, notificationPublisher, invitationRepo) ============
	// Initialize file service with all required dependencies
	fileAuthAdapter := NewFileAuthServiceAdapter(authSvc)
	fileSvc := file.NewService(fileRepo, storage, fileValidator, cfg, dbConn, auditSvc, invitationRepo, fileAuthAdapter, notificationSvc, adminRepo)
	fileHandler := file.NewHandler(fileSvc)

	// Register file routes
	file.RegisterRoutes(v1, fileHandler, middleware.Auth(cfg))

	// ============ BILLING MODULE ============
	RegisterBillingRoutes(r, v1, cfg, dbConn, practitionerRepo, userSubscriptionRepo, stripeClient, auditSvc, notificationSvc, adminRepo)

	// ============ BUILDER ROUTES (requires notificationPublisher) ============
	RegisterBuilderRoutes(v1, cfg, dbConn, clinicSvc, coaSvc, coaRepo, practitionerSvc, accountantRepo, authRepo, auditSvc, invitationSvc, invitationRepo, notificationSvc, adminRepo, authSvc, plRepo)

	return auditSvc, notifier, notificationRepo, notificationSvc, notificationConsumer

}
