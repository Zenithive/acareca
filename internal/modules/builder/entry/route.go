package entry

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/builder/version"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
)

// entryFormPermissionResolver resolves entry ID → form version ID → form ID for permission checks
// Permissions are stored by form_id, so we need to traverse: entry → version → form
type entryFormPermissionResolver struct {
	repo       IRepository
	versionSvc version.IService
}

func (r *entryFormPermissionResolver) ResolveParentEntityID(ctx context.Context, entryID uuid.UUID) (uuid.UUID, error) {
	// Entry → FormVersion → FormID (where permissions are stored)
	entry, _, err := r.repo.GetByID(ctx, entryID)
	if err != nil {
		return uuid.Nil, err
	}
	if entry == nil {
		return uuid.Nil, nil
	}

	// Get form ID from version
	v, err := r.versionSvc.GetByID(ctx, entry.FormVersionID)
	if err != nil {
		return uuid.Nil, err
	}
	if v == nil {
		return uuid.Nil, nil
	}

	return v.FormId, nil
}

// versionFormResolver resolves form version ID → form ID for permission checks
// Permissions are stored by form_id, not form_version_id
type versionFormResolver struct {
	versionSvc version.IService
}

func (r *versionFormResolver) ResolveParentEntityID(ctx context.Context, versionID uuid.UUID) (uuid.UUID, error) {
	v, err := r.versionSvc.GetByID(ctx, versionID)
	if err != nil {
		return uuid.Nil, err
	}
	if v == nil {
		return uuid.Nil, nil
	}
	return v.FormId, nil
}

func RegisterRoutes(rg *gin.RouterGroup, h IHandler, permChecker middleware.PermissionChecker, repo IRepository, versionSvc version.IService) {
	// Version-based routes: Resolve version ID to form ID for permission checking
	versionResolver := &versionFormResolver{versionSvc: versionSvc}
	versionRoutes := rg.Group("/version/:version_id")
	versionRoutes.Use(middleware.ChildEntityPermissionMiddleware(permChecker, versionResolver))
	{
		versionRoutes.GET("", h.List)
		versionRoutes.POST("", h.Create)
	}

	// Transaction route: Check form permissions for transaction operations
	transactionRoutes := rg.Group("")
	transactionRoutes.Use(middleware.MethodBasedPermission(permChecker))
	{
		transactionRoutes.GET("/transactions", h.ListTransactions)
	}

	// COA-grouped routes: New endpoints for master-detail grid
	coaRoutes := rg.Group("/coa-entries")
	coaRoutes.Use(middleware.MethodBasedPermission(permChecker))
	{
		coaRoutes.GET("", h.ListCoaEntries)
		coaRoutes.GET("/:coa_id/entries", h.ListCoaEntryDetails)
		coaRoutes.GET("/export", h.HandleExport)
	}

	// ID-based routes: Entry permissions inherit from associated form
	// Uses ChildEntityPermissionMiddleware which resolves entry ID → form version ID → form ID
	// for permission checking (permissions are stored by form_id)
	entryPermResolver := &entryFormPermissionResolver{repo: repo, versionSvc: versionSvc}
	idRoutes := rg.Group("/:id")
	idRoutes.Use(middleware.ChildEntityPermissionMiddleware(permChecker, entryPermResolver))
	{
		idRoutes.GET("", h.Get)
		idRoutes.PATCH("", h.Update)
		idRoutes.DELETE("", h.Delete)
	}
}
