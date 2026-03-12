package route

import (
	"context"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/subscription"
	"github.com/iamarpitzala/acareca/internal/modules/auth"
	"github.com/iamarpitzala/acareca/internal/modules/business/clinic"
	"github.com/iamarpitzala/acareca/internal/modules/business/coa"
	"github.com/iamarpitzala/acareca/internal/modules/business/fy"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	userSubscription "github.com/iamarpitzala/acareca/internal/modules/business/subscription"
	"github.com/iamarpitzala/acareca/internal/modules/engine/calculation"
	"github.com/iamarpitzala/acareca/internal/modules/engine/method"
	formdetail "github.com/iamarpitzala/acareca/internal/modules/form/detail"
	formentry "github.com/iamarpitzala/acareca/internal/modules/form/entry"
	formfield "github.com/iamarpitzala/acareca/internal/modules/form/field"
	formorchestrate "github.com/iamarpitzala/acareca/internal/modules/form/orchestrate"
	formversion "github.com/iamarpitzala/acareca/internal/modules/form/version"
	"github.com/iamarpitzala/acareca/internal/shared/db"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
)

func RegisterRoutes(r *gin.Engine, cfg *config.Config) {
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
	subscriptionSvc := subscription.NewService(subscriptionRepo)
	userSubscriptionSvc := userSubscription.NewService(userSubscriptionRepo)
	practitionerSvc := practitioner.NewService(practitionerRepo, subscriptionSvc, userSubscriptionSvc, coaRepo)

	authSvc := auth.NewService(authRepo, cfg, practitionerSvc)
	authHandler := auth.NewHandler(authSvc)
	auth.RegisterRoutes(v1, authHandler)

	calculationRepo := calculation.NewRepository(dbConn)
	calculationSvc := calculation.NewService(calculationRepo, method.NewService())
	calculationHandler := calculation.NewHandler(calculationSvc)
	calculation.RegisterRoutes(v1, calculationHandler)

	superadminCheck := func(ctx context.Context, userID uuid.UUID) (bool, error) {
		u, err := authRepo.FindByID(ctx, userID)
		if err != nil {
			return false, err
		}
		return u.IsSuperadmin != nil && *u.IsSuperadmin, nil
	}
	adminGroup := v1.Group("/admin")
	subscriptionGroup := adminGroup.Group("/subscription")
	subscriptionGroup.Use(middleware.Auth(cfg), middleware.RequireSuperadmin(func(ctx context.Context, userID string) (bool, error) {
		id, err := util.ParseUUID(userID)
		if err != nil {
			return false, err
		}
		return superadminCheck(ctx, id)
	}))
	subscriptionHandler := subscription.NewHandler(subscriptionSvc)
	subscription.RegisterRoutes(subscriptionGroup, subscriptionHandler)

	// clinic
	clinicRepo := clinic.NewRepository(dbConn)
	clinicSvc := clinic.NewService(clinicRepo)
	clinicHandler := clinic.NewHandler(clinicSvc)
	clinic.RegisterRoutes(v1, clinicHandler, cfg)

	coaSvc := coa.NewService(coaRepo)
	coaHandler := coa.NewHandler(coaSvc)
	coa.RegisterRoutes(v1.Group("/coa"), coaHandler)
	fyRepo := fy.NewRepository(dbConn)
	fySvc := fy.NewService(fyRepo)
	fyHandler := fy.NewHandler(fySvc)
	fy.RegisterRoutes(v1, fyHandler)

	// form (detail + version + field + entry) – clinic-scoped
	formDetailRepo := formdetail.NewRepository(dbConn)
	formVersionRepo := formversion.NewRepository(dbConn)
	formVersionSvc := formversion.NewService(formVersionRepo, clinicSvc)
	formDetailSvc := formdetail.NewService(formDetailRepo, formVersionSvc)
	formEntryRepo := formentry.NewRepository(dbConn)
	formFieldRepo := formfield.NewRepository(dbConn)
	formFieldSvc := formfield.NewService(formFieldRepo, coaSvc, clinicSvc, practitionerSvc, formEntryRepo, formVersionSvc, formDetailSvc)
	formOrch := formorchestrate.NewOrchestrator(formDetailSvc, formVersionSvc, formFieldSvc)
	formDetailHandler := formdetail.NewHandler(formDetailSvc, formOrch)

	formDetailGroup := v1.Group("/clinic/:id")
	formGroup := formDetailGroup.Group("/form")
	formGroup.Use(middleware.Auth(cfg))
	formdetail.RegisterRoutes(formGroup, formDetailHandler)

	formVersionHandler := formversion.NewHandler(formVersionSvc)
	formIdGroup := formGroup.Group("/:id")
	formVersionGroup := formIdGroup.Group("/version")
	formversion.RegisterRoutes(formVersionGroup, formVersionHandler)

	formFieldHandler := formfield.NewHandler(formFieldSvc)
	formfield.RegisterRoutes(formIdGroup.Group("/field"), formFieldHandler)

	formVersionGroup.Group("/:version_id/field").PUT("/sync", formFieldHandler.Sync)

	formEntrySvc := formentry.NewService(formEntryRepo)
	formEntryHandler := formentry.NewHandler(formEntrySvc)
	formentry.RegisterRoutes(formVersionGroup.Group("/:id/entry"), formEntryHandler)
}
