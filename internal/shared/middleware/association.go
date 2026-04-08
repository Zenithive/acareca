package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

// PermissionItem defines the interface for a single permission
type PermissionItem interface {
	HasAccess(action string) bool
}

// PermissionChecker defines the interface for checking accountant permissions
type PermissionChecker interface {
	ListAccountantPermission(ctx context.Context, accId uuid.UUID) ([]PermissionItem, error)
	GetPermissionsForAccountant(ctx context.Context, accountantID uuid.UUID, entityID uuid.UUID) (PermissionItem, error)
}

// var errUnauthorized = errors.New("unauthorized")
// requiredAction can be: "read", "create", "update", "delete", or "all"
func PermissionMiddleware(checker PermissionChecker, requiredAction string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// Get entity ID (practitioner or accountant ID)
		entityId, ok := ctx.Get(util.EntityIDKey)
		if !ok {
			response.Error(ctx, http.StatusUnauthorized, errUnauthorized)
			ctx.Abort()
			return
		}

		// Get user role
		role, ok := ctx.Get("role")
		if !ok {
			response.Error(ctx, http.StatusUnauthorized, errUnauthorized)
			ctx.Abort()
			return
		}

		// Only check permissions for accountants (practitioners have full access)
		if strings.EqualFold(role.(string), util.RoleAccountant) {
			accId, ok := entityId.(uuid.UUID)
			if !ok {
				response.Error(ctx, http.StatusUnauthorized, errors.New("invalid accountant id"))
				ctx.Abort()
				return
			}

			// Get all permissions for this accountant
			perms, err := checker.ListAccountantPermission(ctx, accId)
			if err != nil {
				response.Error(ctx, http.StatusUnauthorized, errors.New("authentication error: "+err.Error()))
				ctx.Abort()
				return
			}

			// Check if accountant has the required permission
			hasPermission := false

			if perms != nil {
				for _, v := range perms {
					if v.HasAccess(requiredAction) {
						hasPermission = true
						break
					}
				}
			}

			if !hasPermission {
				actionMsg := requiredAction
				if requiredAction == "all" {
					actionMsg = "full access"
				}
				response.Error(ctx, http.StatusForbidden, errors.New("you do not have permission to "+actionMsg+" for this resource"))
				ctx.Abort()
				return
			}
		}

		ctx.Next()
	}
}

// RequirePermission is a convenience wrapper for common permission checks
func RequirePermission(checker PermissionChecker, action string) gin.HandlerFunc {
	return PermissionMiddleware(checker, action)
}

// MethodBasedPermission checks permissions based on HTTP method
// GET -> read, POST -> create, PUT/PATCH -> update, DELETE -> delete
func MethodBasedPermission(checker PermissionChecker) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// Get entity ID (practitioner or accountant ID)
		entityId, ok := ctx.Get(util.EntityIDKey)
		if !ok {
			response.Error(ctx, http.StatusUnauthorized, errUnauthorized)
			ctx.Abort()
			return
		}

		// Get user role
		role, ok := ctx.Get("role")
		if !ok {
			response.Error(ctx, http.StatusUnauthorized, errUnauthorized)
			ctx.Abort()
			return
		}

		// Only check permissions for accountants (practitioners have full access)
		if strings.EqualFold(role.(string), util.RoleAccountant) {
			accId, ok := entityId.(uuid.UUID)
			if !ok {
				response.Error(ctx, http.StatusUnauthorized, errors.New("invalid accountant id"))
				ctx.Abort()
				return
			}

			// Map HTTP method to required permission
			var requiredAction string
			switch ctx.Request.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				requiredAction = "read"
			case http.MethodPost:
				requiredAction = "create"
			case http.MethodPut, http.MethodPatch:
				requiredAction = "update"
			case http.MethodDelete:
				requiredAction = "delete"
			default:
				response.Error(ctx, http.StatusMethodNotAllowed, errors.New("method not allowed"))
				ctx.Abort()
				return
			}

			// Try to get the target entity ID from URL parameters
			// Common patterns: /:id, /form/:id, /version/:version_id
			var targetEntityID uuid.UUID
			var err error
			
			// Try common parameter names
			if idParam := ctx.Param("id"); idParam != "" {
				targetEntityID, err = uuid.Parse(idParam)
				if err != nil {
					response.Error(ctx, http.StatusBadRequest, errors.New("invalid entity id"))
					ctx.Abort()
					return
				}
			} else if versionID := ctx.Param("version_id"); versionID != "" {
				targetEntityID, err = uuid.Parse(versionID)
				if err != nil {
					response.Error(ctx, http.StatusBadRequest, errors.New("invalid version id"))
					ctx.Abort()
					return
				}
			} else {
				// No entity ID in URL - this might be a list operation
				// For list operations, we check if accountant has ANY permission
				perms, err := checker.ListAccountantPermission(ctx, accId)
				if err != nil {
					response.Error(ctx, http.StatusUnauthorized, errors.New("authentication error: "+err.Error()))
					ctx.Abort()
					return
				}

				hasPermission := false
				if perms != nil {
					for _, v := range perms {
						if v.HasAccess(requiredAction) {
							hasPermission = true
							break
						}
					}
				}

				if !hasPermission {
					response.Error(ctx, http.StatusForbidden, errors.New("you do not have permission to "+requiredAction+" this resource"))
					ctx.Abort()
					return
				}
				
				ctx.Next()
				return
			}

			// Check permissions for the specific entity
			perm, err := checker.GetPermissionsForAccountant(ctx, accId, targetEntityID)
			if err != nil || perm == nil {
				response.Error(ctx, http.StatusForbidden, errors.New("you do not have access to this resource"))
				ctx.Abort()
				return
			}

			if !perm.HasAccess(requiredAction) {
				response.Error(ctx, http.StatusForbidden, errors.New("you do not have permission to "+requiredAction+" this resource"))
				ctx.Abort()
				return
			}
		}

		ctx.Next()
	}
}
