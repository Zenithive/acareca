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

// EntityResolver defines the interface for resolving child entity IDs to their parent entity IDs for permission checking
// For example: entry ID -> form version ID
type EntityResolver interface {
	ResolveParentEntityID(ctx context.Context, childID uuid.UUID) (uuid.UUID, error)
}

// ChildEntityPermissionMiddleware checks if a user has permission for a child entity by resolving its parent
// This is used when permissions are inherited from parent entities (e.g., entry inherits from form)
// resolver: function to resolve the child entity ID to the parent entity ID for permission checking
func ChildEntityPermissionMiddleware(checker PermissionChecker, resolver EntityResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse child entity ID from URL parameter (typically :id)
		childIDParam := c.Param("id")
		if childIDParam == "" {
			// Fallback to other common parameter names if :id not found
			for _, param := range []string{"entry_id", "clinic_id", "form_id", "version_id"} {
				if val := c.Param(param); val != "" {
					childIDParam = val
					break
				}
			}
		}

		if childIDParam == "" {
			response.Error(c, http.StatusBadRequest, errors.New("entity id not found in path"))
			c.Abort()
			return
		}

		childID, err := uuid.Parse(childIDParam)
		if err != nil {
			response.Error(c, http.StatusBadRequest, errors.New("invalid entity id format"))
			c.Abort()
			return
		}

		// Resolve the parent entity ID that will be used for permission checking
		parentEntityID, err := resolver.ResolveParentEntityID(c.Request.Context(), childID)
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			c.Abort()
			return
		}

		if parentEntityID == uuid.Nil {
			response.Error(c, http.StatusNotFound, errors.New("entity not found"))
			c.Abort()
			return
		}

		// Get user role
		role, ok := c.Get("role")
		if !ok {
			response.Error(c, http.StatusForbidden, errors.New("role not in context"))
			c.Abort()
			return
		}

		// Only check permissions for accountants (practitioners have full access)
		if strings.EqualFold(role.(string), util.RoleAccountant) {
			// Get accountant ID from context
			entityId, ok := c.Get(util.EntityIDKey)
			if !ok {
				response.Error(c, http.StatusForbidden, errors.New("accountant id not in context"))
				c.Abort()
				return
			}

			accountantID, ok := entityId.(uuid.UUID)
			if !ok {
				response.Error(c, http.StatusForbidden, errors.New("invalid accountant id type"))
				c.Abort()
				return
			}

			// Map HTTP method to required permission
			var requiredAction string
			switch c.Request.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				requiredAction = "read"
			case http.MethodPost:
				requiredAction = "create"
			case http.MethodPut, http.MethodPatch:
				requiredAction = "update"
			case http.MethodDelete:
				requiredAction = "delete"
			default:
				response.Error(c, http.StatusMethodNotAllowed, errors.New("method not allowed"))
				c.Abort()
				return
			}

			// Check permissions using the parent entity ID
			perm, err := checker.GetPermissionsForAccountant(c.Request.Context(), accountantID, parentEntityID)
			if err != nil || perm == nil {
				response.Error(c, http.StatusForbidden, errors.New("you do not have access to this resource"))
				c.Abort()
				return
			}

			if !perm.HasAccess(requiredAction) {
				response.Error(c, http.StatusForbidden, errors.New("you do not have permission to "+requiredAction+" this resource"))
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// Entry-specific middleware helpers
// These are used to resolve entry and version entities to their parent forms for permission checking

// EntryFormPermissionResolver resolves entry ID → form version ID → form ID for permission checks
// This is needed because permissions are stored by form_id
func NewEntryFormPermissionMiddleware(checker PermissionChecker, entryResolver EntityResolver) gin.HandlerFunc {
	return ChildEntityPermissionMiddleware(checker, entryResolver)
}

// VersionFormPermissionResolver resolves form version ID → form ID for permission checks
// This is needed because permissions are stored by form_id, not form_version_id
func NewVersionFormPermissionMiddleware(checker PermissionChecker, versionResolver EntityResolver) gin.HandlerFunc {
	return ChildEntityPermissionMiddleware(checker, versionResolver)
}
