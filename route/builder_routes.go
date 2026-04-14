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

// builderPermissionAdapter adapts invitation.Service to middleware.PermissionChecker
type builderPermissionAdapter struct {
	invSvc invitation.Service
}

func (a *builderPermissionAdapter) ListAccountantPermission(ctx context.Context, accId uuid.UUID) ([]middleware.PermissionItem, error) {
	perms, _, err := a.invSvc.ListAccountantPermission(ctx, accId)
	if err != nil {
		return nil, err
	}

	result := make([]middleware.PermissionItem, 0, len(*perms))
	for i := range *perms {
		result = append(result, &(*perms)[i])
	}
	return result, nil
}

func (a *builderPermissionAdapter) GetPermissionsForAccountant(ctx context.Context, accountantID uuid.UUID, entityID uuid.UUID) (middleware.PermissionItem, error) {
	perm, err := a.invSvc.GetPermissionsForAccountant(ctx, accountantID, entityID)
	if err != nil {
		return nil, err
	}
	return perm, nil
}

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
	permChecker := &builderPermissionAdapter{invSvc: invitationSvc}
	formGroup.Use(middleware.MethodBasedPermission(permChecker))
	form.RegisterRoutes(formGroup, formHandler, permChecker)

	// Entry routes
	entriesRepo := entry.NewRepository(dbConn)
	entriesSvc := entry.NewService(dbConn, entriesRepo, fieldRepo, method.NewService(), detailSvc, versionSvc, auditSvc, eventsSvc, accountantRepo, authRepo, clinicRepo, clinicSvc, formulaSvc, fieldSvc, invitationSvc)
	entriesHandler := entry.NewHandler(entriesSvc)

	entryGroup := v1.Group("/entry", middleware.Auth(cfg), middleware.AuditContext())
	// DO NOT apply MethodBasedPermission here - let entry.RegisterRoutes handle middleware per route
	// This is important because version/:version_id and :id routes need resolvers to map to form_id
	entry.RegisterRoutes(entryGroup, entriesHandler, permChecker, entryRepo, versionSvc)

	// Calculation routes
	calculationGroup := v1.Group("")
	calculationGroup.Use(middleware.Auth(cfg))
	calculationSvc := calculation.NewServiceWithFormula(formSvc, versionSvc, fieldSvc, entriesSvc, formulaSvc)
	calculationHandler := calculation.NewHandler(calculationSvc)
	calculation.RegisterRoutes(calculationGroup, calculationHandler)
}
