package route

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/modules/admin/subscription"
	"github.com/iamarpitzala/acareca/internal/modules/auth"
	"github.com/iamarpitzala/acareca/internal/modules/engine/calculation"
	"github.com/iamarpitzala/acareca/internal/modules/engine/method"
	tentant "github.com/iamarpitzala/acareca/internal/modules/tentant/setting"
	tentantSub "github.com/iamarpitzala/acareca/internal/modules/tentant/subscription"
	"github.com/iamarpitzala/acareca/internal/shared/db"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
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
	tentantRepo := tentant.NewRepository(dbConn)
	tentantSubRepo := tentantSub.NewRepository(dbConn)

	onUserCreated := func(ctx context.Context, userID string) error {
		existing, err := tentantRepo.GetByUserID(ctx, userID)
		if err == nil && existing != nil {
			return nil
		}
		t, err := tentantRepo.Create(ctx, &tentant.Tentant{UserID: userID})
		if err != nil {
			log.Printf("onboarding: create tentant for user %s: %v", userID, err)
			return err
		}
		trial, err := subscriptionRepo.FindByName(ctx, "Trial")
		if err != nil {
			log.Printf("onboarding: find Trial subscription: %v", err)
			return err
		}
		start := time.Now()
		end := start.AddDate(0, 0, trial.DurationDays)
		_, err = tentantSubRepo.Create(ctx, &tentantSub.TentantSubscription{
			TentantID:      t.ID,
			SubscriptionID: trial.ID,
			StartDate:      start,
			EndDate:        end,
			Status:         tentantSub.StatusActive,
		})
		if err != nil {
			log.Printf("onboarding: create trial subscription for tentant %d: %v", t.ID, err)
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

	superadminCheck := func(ctx context.Context, userID string) (bool, error) {
		u, err := authRepo.FindByID(ctx, userID)
		if err != nil {
			return false, err
		}
		return u.IsSuperadmin != nil && *u.IsSuperadmin, nil
	}
	adminGroup := v1.Group("/admin")
	subscriptionGroup := adminGroup.Group("/subscription")
	subscriptionGroup.Use(middleware.Auth(cfg), middleware.RequireSuperadmin(superadminCheck))
	subscriptionSvc := subscription.NewService(subscriptionRepo)
	subscriptionHandler := subscription.NewHandler(subscriptionSvc)
	subscription.RegisterRoutes(subscriptionGroup, subscriptionHandler)

	tentantSvc := tentant.NewService(tentantRepo)
	tentantHandler := tentant.NewHandler(tentantSvc)
	tentantGroup := v1.Group("/tentant")
	tentant.RegisterRoutes(tentantGroup, tentantHandler)
	tentantSubSvc := tentantSub.NewService(tentantSubRepo)
	tentantSubHandler := tentantSub.NewHandler(tentantSubSvc)
	tentantSub.RegisterRoutes(tentantGroup.Group("/:id/subscription"), tentantSubHandler)
}
