package route

import (
	"log"

	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/admin/subscription"
	"github.com/iamarpitzala/acareca/internal/modules/auth"
	"github.com/iamarpitzala/acareca/internal/modules/builder/detail"
	"github.com/iamarpitzala/acareca/internal/modules/builder/entry"
	"github.com/iamarpitzala/acareca/internal/modules/builder/field"
	"github.com/iamarpitzala/acareca/internal/modules/builder/form"
	"github.com/iamarpitzala/acareca/internal/modules/builder/version"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/business/admin"
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
	"github.com/iamarpitzala/acareca/internal/shared/db"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func RegisterRoutes(r *gin.Engine, cfg *config.Config) (audit.Service, *sharednotification.Hub, notification.Repository) {

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

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

	subscriptionSvc := subscription.NewService(dbConn, subscriptionRepo, auditSvc)
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
	invitationHandler := invitation.NewHandler(invitationSvc)
	invitation.RegisterRoutes(v1, invitationHandler, cfg)

	//admin auth
	adminRepo := admin.NewRepository(dbConn)
	adminSvc := admin.NewService(adminRepo, dbConn)
	adminHandler := admin.NewHandler(adminSvc)
	admin.RegisterRoutes(v1, adminHandler, cfg)

	authSvc := auth.NewService(authRepo, cfg, dbConn, practitionerSvc, auditSvc, invitationSvc, practitionerRepo, accountantSvc, adminSvc, invitationRepo)
	authHandler := auth.NewHandler(authSvc)
	auth.RegisterRoutes(v1, authHandler, middleware.Auth(cfg))

	superadminCheck := func(ctx context.Context, userID uuid.UUID) (bool, error) {
		u, err := authRepo.FindByID(ctx, userID)
		if err != nil {
			return false, err
		}
		return u.Role != "" && u.Role == util.RoleAdmin, nil
	}
	adminGroup := v1.Group("/admin")
	subscriptionGroup := adminGroup.Group("/subscription")
	subscriptionGroup.Use(middleware.Auth(cfg), middleware.RequireSuperadmin(func(ctx context.Context, userID string) (bool, error) {
		id, err := util.ParseUUID(userID)
		if err != nil {
			return false, err
		}
		return superadminCheck(ctx, id)
	}), middleware.AuditContext())
	subscriptionHandler := subscription.NewHandler(subscriptionSvc)
	subscription.RegisterRoutes(subscriptionGroup, subscriptionHandler)

	// Audit routes
	auditGroup := adminGroup.Group("/audit")
	auditGroup.Use(middleware.Auth(cfg), middleware.RequireSuperadmin(func(ctx context.Context, userID string) (bool, error) {
		id, err := util.ParseUUID(userID)
		if err != nil {
			return false, err
		}
		return superadminCheck(ctx, id)
	}))
	auditHandler := audit.NewHandler(auditSvc)
	audit.RegisterRoutes(auditGroup, auditHandler)

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
	fyGroup.Use(middleware.Auth(cfg))
	fy.RegisterRoutes(fyGroup, fyHandler)

	formGroup := v1.Group("/form")
	formGroup.Use(middleware.Auth(cfg), middleware.AuditContext())

	detailRepo := detail.NewRepository(dbConn)
	versionRepo := version.NewRepository(dbConn)
	fieldRepo := field.NewRepository(dbConn)
	entryRepo := entry.NewRepository(dbConn)
	detailSvc := detail.NewService(dbConn, detailRepo, version.NewService(dbConn, versionRepo, clinicSvc), clinicRepo)
	fieldSvc := field.NewService(fieldRepo, coaSvc, clinicSvc, practitionerSvc, version.NewService(dbConn, versionRepo, clinicSvc))

	versionSvc := version.NewService(dbConn, versionRepo, clinicSvc)
	formulaRepo := formula.NewRepository(dbConn)
	formulaSvc := formula.NewService(formulaRepo)
	formSvc := form.NewService(dbConn, detailSvc, versionSvc, fieldSvc, formulaSvc, entryRepo, coaSvc, auditSvc, eventsSvc, accountantRepo, authRepo, clinicSvc)
	formHandler := form.NewHandler(formSvc)
	form.RegisterRoutes(formGroup, formHandler)

	entryGroup := v1.Group("/entry")
	entryGroup.Use(middleware.Auth(cfg), middleware.AuditContext())
	entriesRepo := entry.NewRepository(dbConn)
	entriesSvc := entry.NewService(dbConn, entriesRepo, fieldRepo, method.NewService(), detailSvc, versionSvc, auditSvc, eventsSvc, accountantRepo, authRepo, clinicRepo, clinicSvc, formulaSvc, fieldSvc)
	entriesHandler := entry.NewHandler(entriesSvc)

	entry.RegisterRoutes(entryGroup, entriesHandler)

	calculationSvc := calculation.NewServiceWithFormula(formSvc, versionSvc, fieldSvc, entriesSvc, formulaSvc)
	calculationHandler := calculation.NewHandler(calculationSvc)
	calculation.RegisterRoutes(v1, calculationHandler)

	// P&L reporting — engine/pl module
	plRepo := pl.NewRepository(dbConn)
	plSvc := pl.NewService(plRepo)
	plHandler := pl.NewHandler(plSvc)
	pl.RegisterRoutes(v1, plHandler, cfg)

	// BAS reporting — engine/bas module
	basRepo := bas.NewRepository(dbConn)
	basSvc := bas.NewService(basRepo)
	basHandler := bas.NewHandler(basSvc)
	bas.RegisterRoutes(v1, basHandler, cfg)

	settingGroup := v1.Group("/setting")
	settingRepo := setting.NewRepository(dbConn)
	settingSvc := setting.NewService(dbConn, settingRepo, auditSvc)
	settingHandler := setting.NewHandler(settingSvc)

	setting.RegisterRoutes(settingGroup, settingHandler, cfg)

	practitionerHandler := practitioner.NewHandler(practitionerSvc)
	practitioner.RegisterRoutes(v1, practitionerHandler, cfg)

	userSubscriptionHandler := userSubscription.NewHandler(userSubscriptionSvc, dbConn)

	userSubscriptionGroup := v1.Group("/practitioner/subscription")

	userSubscriptionGroup.Use(middleware.Auth(cfg))

	userSubscription.RegisterRoutes(userSubscriptionGroup, userSubscriptionHandler)

	// notification (in-app list + WebSocket)
	notificationHandler := notification.NewHandler(notificationSvc)
	notification.RegisterRoutes(v1, notificationHandler, notifier, cfg)

	accountantHandler := accountant.NewHandler(accountantSvc)

	accountant.RegisterRoutes(v1, accountantHandler, middleware.Auth(cfg))

	return auditSvc, notifier, notificationRepo

}
