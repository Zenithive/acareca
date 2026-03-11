package route

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/subscription"
	"github.com/iamarpitzala/acareca/internal/modules/auth"
	"github.com/iamarpitzala/acareca/internal/modules/business/clinic"
	"github.com/iamarpitzala/acareca/internal/modules/business/coa"
	"github.com/iamarpitzala/acareca/internal/modules/engine/calculation"
	"github.com/iamarpitzala/acareca/internal/modules/engine/method"
	formdetail "github.com/iamarpitzala/acareca/internal/modules/form/detail"
	formversion "github.com/iamarpitzala/acareca/internal/modules/form/version"
	practitioner "github.com/iamarpitzala/acareca/internal/modules/practitioner/setting"
	practitionerSub "github.com/iamarpitzala/acareca/internal/modules/practitioner/subscription"
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
	practitionerSubRepo := practitionerSub.NewRepository(dbConn)

	onUserCreated := func(ctx context.Context, userID string) error {
		existing, err := practitionerRepo.GetByUserID(ctx, userID)
		if err == nil && existing != nil {
			return nil
		}
		t, err := practitionerRepo.Create(ctx, &practitioner.Practitioner{UserID: userID})
		if err != nil {
			log.Printf("onboarding: create practitioner for user %s: %v", userID, err)
			return err
		}
		trial, err := subscriptionRepo.FindByName(ctx, "Trial")
		if err != nil {
			log.Printf("onboarding: find Trial subscription: %v", err)
			return err
		}
		start := time.Now()
		end := start.AddDate(0, 0, trial.DurationDays)
		_, err = practitionerSubRepo.Create(ctx, &practitionerSub.PractitionerSubscription{
			PractitionerID: t.ID,
			SubscriptionID: trial.ID,
			StartDate:      start,
			EndDate:        end,
			Status:         practitionerSub.StatusActive,
		})
		if err != nil {
			log.Printf("onboarding: create trial subscription for practitioner %s: %v", t.ID, err)
			return err
		}
		return nil
	}

	authSvc := auth.NewService(authRepo, cfg, onUserCreated)
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
	subscriptionSvc := subscription.NewService(subscriptionRepo)
	subscriptionHandler := subscription.NewHandler(subscriptionSvc)
	subscription.RegisterRoutes(subscriptionGroup, subscriptionHandler)

	practitionerSvc := practitioner.NewService(practitionerRepo)
	practitionerHandler := practitioner.NewHandler(practitionerSvc)
	practitionerGroup := v1.Group("/practitioner")
	practitioner.RegisterRoutes(practitionerGroup, practitionerHandler)
	practitionerSubSvc := practitionerSub.NewService(practitionerSubRepo)
	practitionerSubHandler := practitionerSub.NewHandler(practitionerSubSvc)
	practitionerSub.RegisterRoutes(practitionerGroup.Group("/:id/subscription"), practitionerSubHandler)

	// clinic
	clinicRepo := clinic.NewRepository(dbConn)
	clinciSvc := clinic.NewService(clinicRepo)
	clinicHandler := clinic.NewHandler(clinciSvc)
	clinic.RegisterRoutes(v1, clinicHandler)

	coaRepo := coa.NewRepository(dbConn)
	coaSvc := coa.NewService(coaRepo)
	coaHandler := coa.NewHandler(coaSvc)
	coa.RegisterRoutes(v1.Group("/coa"), coaHandler)

	// form (detail + version) – clinic-scoped
	formDetailRepo := formdetail.NewRepository(dbConn)
	formDetailSvc := formdetail.NewService(formDetailRepo)
	formDetailHandler := formdetail.NewHandler(formDetailSvc)
	formGroup := v1.Group("/clinic/:clinic_id/forms")
	formdetail.RegisterRoutes(formGroup, formDetailHandler)

	formVersionRepo := formversion.NewRepository(dbConn)
	formVersionSvc := formversion.NewService(formVersionRepo)
	formVersionHandler := formversion.NewHandler(formVersionSvc)
	formversion.RegisterRoutes(formGroup.Group("/:form_id/versions"), formVersionHandler)
}
