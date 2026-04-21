package route

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/auth"
	"github.com/iamarpitzala/acareca/internal/modules/builder/detail"
	"github.com/iamarpitzala/acareca/internal/modules/builder/entry"
	"github.com/iamarpitzala/acareca/internal/modules/builder/field"
	"github.com/iamarpitzala/acareca/internal/modules/builder/form"
	"github.com/iamarpitzala/acareca/internal/modules/builder/version"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/business/clinic"
	"github.com/iamarpitzala/acareca/internal/modules/business/coa"
	"github.com/iamarpitzala/acareca/internal/modules/business/invitation"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	"github.com/iamarpitzala/acareca/internal/modules/business/shared/events"
	"github.com/iamarpitzala/acareca/internal/modules/engine/calculation"
	"github.com/iamarpitzala/acareca/internal/modules/engine/formula"
	"github.com/iamarpitzala/acareca/internal/modules/engine/method"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
)

func RegisterBuilderRoutes(
	v1 *gin.RouterGroup,
	cfg *config.Config,
	dbConn *sqlx.DB,
	clinicSvc clinic.Service,
	coaSvc coa.Service,
	practitionerSvc practitioner.IService,
	accountantRepo accountant.Repository,
	authRepo auth.Repository,
	auditSvc audit.Service,
	eventsSvc events.Service,
	invitationSvc invitation.Service,
) {
	// Create permission adapter for feature-based permissions
	// Wrap the service methods to convert *Permissions to FeaturePermissions interface
	permAdapter := middleware.NewPermissionAdapterFromFuncs(
		func(ctx context.Context, accountantID uuid.UUID, practitionerID uuid.UUID) (middleware.FeaturePermissions, error) {
			perms, err := invitationSvc.GetPermissionsForAccountant(ctx, accountantID, practitionerID)
			if err != nil || perms == nil {
				return nil, err
			}
			return perms, nil // *Permissions implements FeaturePermissions
		},
		invitationSvc.IsAccountantLinkedToPractitioner,
	)

	// Initialize repositories
	clinicRepo := clinic.NewRepository(dbConn)
	detailRepo := detail.NewRepository(dbConn)
	versionRepo := version.NewRepository(dbConn)
	fieldRepo := field.NewRepository(dbConn)
	entryRepo := entry.NewRepository(dbConn)
	formulaRepo := formula.NewRepository(dbConn)

	// Initialize services
	versionSvc := version.NewService(dbConn, versionRepo, clinicSvc)
	detailSvc := detail.NewService(dbConn, detailRepo, versionSvc, clinicRepo, invitationSvc)
	fieldSvc := field.NewService(fieldRepo, coaSvc, clinicSvc, practitionerSvc, versionSvc)
	formulaSvc := formula.NewService(formulaRepo)
	formSvc := form.NewService(dbConn, detailSvc, versionSvc, fieldSvc, formulaSvc, entryRepo, coaSvc, auditSvc, eventsSvc, accountantRepo, authRepo, clinicSvc, invitationSvc)
	formHandler := form.NewHandler(formSvc)

	// Form routes
	formGroup := v1.Group("/form", middleware.Auth(cfg), middleware.AuditContext())
	form.RegisterRoutes(formGroup, formHandler, permAdapter)

	// Entry routes
	entriesRepo := entry.NewRepository(dbConn)
	entriesSvc := entry.NewService(dbConn, entriesRepo, fieldRepo, method.NewService(), detailSvc, versionSvc, auditSvc, eventsSvc, accountantRepo, authRepo, clinicRepo, clinicSvc, formulaSvc, fieldSvc, invitationSvc, detailRepo)
	entriesHandler := entry.NewHandler(entriesSvc)

	entryGroup := v1.Group("/entry", middleware.Auth(cfg), middleware.AuditContext())
	entry.RegisterRoutes(entryGroup, entriesHandler, permAdapter)

	// Calculation routes
	calculationGroup := v1.Group("")
	calculationGroup.Use(middleware.Auth(cfg))
	calculationRepo := calculation.NewRepository(dbConn)
	calculationSvc := calculation.NewServiceWithFormula(calculationRepo, formSvc, versionSvc, fieldSvc, entriesSvc, formulaSvc)
	calculationHandler := calculation.NewHandler(calculationSvc)
	calculation.RegisterRoutes(calculationGroup, calculationHandler, permAdapter)
}
