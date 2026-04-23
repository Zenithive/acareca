package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

// FeaturePermissionChecker defines the interface for checking feature-based permissions
type FeaturePermissionChecker interface {
	GetPermissionsForAccountant(ctx context.Context, accountantID uuid.UUID, practitionerID uuid.UUID) (FeaturePermissions, error)
}

// FeaturePermissions defines the interface for feature-based permission checks
type FeaturePermissions interface {
	HasReadAccess(feature string) bool
	HasWriteAccess(feature string) bool
	HasAccess(feature string) bool
}

// Feature constants
const (
	FeatureSalesPurchases = "sales_purchases"
	FeatureLockDates      = "lock_dates"
	FeatureUsers          = "users"
	FeatureReports        = "reports"
)

// Action constants
const (
	ActionRead  = "read"
	ActionWrite = "write"
)

// RequireFeaturePermission creates middleware that checks if the accountant has the required permission for a feature
// Usage:
//   - RequireFeaturePermission(checker, FeatureSalesPurchases, ActionRead)  // Read access to sales/purchases
//   - RequireFeaturePermission(checker, FeatureLockDates, ActionWrite)      // Write access to lock dates
func RequireFeaturePermission(checker FeaturePermissionChecker, feature string, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get role and actor ID
		actorID, role, ok := util.GetRoleBasedID(c)
		if !ok {
			response.Error(c, http.StatusUnauthorized, errors.New("unauthorized: missing role or actor ID"))
			c.Abort()
			return
		}

		// Practitioners have full access - skip permission check
		if role == util.RolePractitioner {
			c.Next()
			return
		}

		// For accountants, check permissions
		if role == util.RoleAccountant {
			// Get practitioner ID from context or request
			practitionerID, err := getPractitionerIDFromContext(c)
			if err != nil {
				response.Error(c, http.StatusBadRequest, errors.New("missing practitioner context"))
				c.Abort()
				return
			}

			// Get permissions for this accountant
			perms, err := checker.GetPermissionsForAccountant(c.Request.Context(), *actorID, practitionerID)
			if err != nil {
				response.Error(c, http.StatusForbidden, errors.New("failed to retrieve permissions"))
				c.Abort()
				return
			}

			if perms == nil {
				response.Error(c, http.StatusForbidden, errors.New("no permissions found"))
				c.Abort()
				return
			}

			// Check the required permission
			var hasPermission bool
			switch action {
			case ActionRead:
				hasPermission = perms.HasReadAccess(feature)
			case ActionWrite:
				hasPermission = perms.HasWriteAccess(feature)
			default:
				hasPermission = perms.HasAccess(feature)
			}

			if !hasPermission {
				response.Error(c, http.StatusForbidden, errors.New("insufficient permissions for this operation"))
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// RequireAnyFeaturePermission checks if the accountant has permission for ANY of the specified features
// Useful for endpoints that can work with multiple features
func RequireAnyFeaturePermission(checker FeaturePermissionChecker, features []string, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, role, ok := util.GetRoleBasedID(c)
		if !ok {
			response.Error(c, http.StatusUnauthorized, errors.New("unauthorized: missing role or actor ID"))
			c.Abort()
			return
		}

		// Practitioners have full access
		if role == util.RolePractitioner {
			c.Next()
			return
		}

		// For accountants, check permissions
		if role == util.RoleAccountant {
			practitionerID, err := getPractitionerIDFromContext(c)
			if err != nil {
				response.Error(c, http.StatusBadRequest, errors.New("missing practitioner context"))
				c.Abort()
				return
			}

			perms, err := checker.GetPermissionsForAccountant(c.Request.Context(), *actorID, practitionerID)
			if err != nil || perms == nil {
				response.Error(c, http.StatusForbidden, errors.New("no permissions found"))
				c.Abort()
				return
			}

			// Check if accountant has permission for ANY of the features
			hasAnyPermission := false
			for _, feature := range features {
				var hasPermission bool
				switch action {
				case ActionRead:
					hasPermission = perms.HasReadAccess(feature)
				case ActionWrite:
					hasPermission = perms.HasWriteAccess(feature)
				default:
					hasPermission = perms.HasAccess(feature)
				}

				if hasPermission {
					hasAnyPermission = true
					break
				}
			}

			if !hasAnyPermission {
				response.Error(c, http.StatusForbidden, errors.New("insufficient permissions for this operation"))
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// RequireAllFeaturePermissions checks if the accountant has permission for ALL of the specified features
// Useful for endpoints that require multiple permissions
func RequireAllFeaturePermissions(checker FeaturePermissionChecker, features []string, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, role, ok := util.GetRoleBasedID(c)
		if !ok {
			response.Error(c, http.StatusUnauthorized, errors.New("unauthorized: missing role or actor ID"))
			c.Abort()
			return
		}

		// Practitioners have full access
		if role == util.RolePractitioner {
			c.Next()
			return
		}

		// For accountants, check permissions
		if role == util.RoleAccountant {
			practitionerID, err := getPractitionerIDFromContext(c)
			if err != nil {
				response.Error(c, http.StatusBadRequest, errors.New("missing practitioner context"))
				c.Abort()
				return
			}

			perms, err := checker.GetPermissionsForAccountant(c.Request.Context(), *actorID, practitionerID)
			if err != nil || perms == nil {
				response.Error(c, http.StatusForbidden, errors.New("no permissions found"))
				c.Abort()
				return
			}

			// Check if accountant has permission for ALL features
			for _, feature := range features {
				var hasPermission bool
				switch action {
				case ActionRead:
					hasPermission = perms.HasReadAccess(feature)
				case ActionWrite:
					hasPermission = perms.HasWriteAccess(feature)
				default:
					hasPermission = perms.HasAccess(feature)
				}

				if !hasPermission {
					response.Error(c, http.StatusForbidden, errors.New("insufficient permissions for this operation"))
					c.Abort()
					return
				}
			}
		}

		c.Next()
	}
}

// getPractitionerIDFromContext extracts practitioner ID from various sources
// Priority: 1. URL param, 2. Query param, 3. Request body, 4. Context, 5. Derive from accountant
func getPractitionerIDFromContext(c *gin.Context) (uuid.UUID, error) {
	// Try URL parameter
	if pidStr := c.Param("practitioner_id"); pidStr != "" {
		return uuid.Parse(pidStr)
	}

	// Try query parameter
	if pidStr := c.Query("practitioner_id"); pidStr != "" {
		return uuid.Parse(pidStr)
	}

	// Try context value (set by previous middleware)
	if pid, exists := c.Get("practitioner_id"); exists {
		if practitionerID, ok := pid.(uuid.UUID); ok {
			return practitionerID, nil
		}
		if practitionerIDStr, ok := pid.(string); ok {
			return uuid.Parse(practitionerIDStr)
		}
	}

	// For accountants: Try to get from their linked practitioner
	// This is a fallback - ideally practitioner_id should be in the request
	actorID, role, ok := util.GetRoleBasedID(c)
	if ok && role == util.RoleAccountant && actorID != nil {
		// Try to get from context if it was set by a previous lookup
		if pid, exists := c.Get("derived_practitioner_id"); exists {
			if practitionerID, ok := pid.(uuid.UUID); ok {
				return practitionerID, nil
			}
		}
		// Note: We can't call the service here as we don't have access to it
		// The practitioner_id MUST be provided in the request for accountants
		// This is by design - accountants should specify which practitioner's data they're accessing
	}

	return uuid.Nil, errors.New("practitioner_id not found in request")
}

// SetPractitionerID is a helper middleware to set practitioner ID in context
// Use this before permission middleware if practitioner ID comes from a different source
func SetPractitionerID(practitionerID uuid.UUID) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("practitioner_id", practitionerID)
		c.Next()
	}
}

// SetPractitionerIDFromAuth sets practitioner ID in context based on authenticated user
// For practitioners: uses their own ID
// For accountants: requires practitioner_id in query/body or uses first linked practitioner
func SetPractitionerIDFromAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, role, ok := util.GetRoleBasedID(c)
		if !ok {
			c.Next()
			return
		}

		// If practitioner, use their own ID
		if role == util.RolePractitioner && actorID != nil {
			c.Set("practitioner_id", *actorID)
			c.Next()
			return
		}

		// If accountant, try to get practitioner_id from request
		if role == util.RoleAccountant {
			// Try query parameter first
			if pidStr := c.Query("practitioner_id"); pidStr != "" {
				if pid, err := uuid.Parse(pidStr); err == nil {
					c.Set("practitioner_id", pid)
					c.Next()
					return
				}
			}

			// Try URL parameter
			if pidStr := c.Param("practitioner_id"); pidStr != "" {
				if pid, err := uuid.Parse(pidStr); err == nil {
					c.Set("practitioner_id", pid)
					c.Next()
					return
				}
			}

			// For accountants without explicit practitioner_id, we'll need to derive it
			// This will be handled by the permission middleware which will return an error
		}

		c.Next()
	}
}

// CheckPermissionInHandler is a helper function to check permissions within a handler
// Use this when you need dynamic permission checks based on request data
func CheckPermissionInHandler(c *gin.Context, checker FeaturePermissionChecker, feature string, action string) error {
	actorID, role, ok := util.GetRoleBasedID(c)
	if !ok {
		return errors.New("unauthorized: missing role or actor ID")
	}

	// Practitioners have full access
	if role == util.RolePractitioner {
		return nil
	}

	// For accountants, check permissions
	if role == util.RoleAccountant {
		practitionerID, err := getPractitionerIDFromContext(c)
		if err != nil {
			return errors.New("missing practitioner context")
		}

		perms, err := checker.GetPermissionsForAccountant(c.Request.Context(), *actorID, practitionerID)
		if err != nil || perms == nil {
			return errors.New("no permissions found")
		}

		var hasPermission bool
		switch action {
		case ActionRead:
			hasPermission = perms.HasReadAccess(feature)
		case ActionWrite:
			hasPermission = perms.HasWriteAccess(feature)
		default:
			hasPermission = perms.HasAccess(feature)
		}

		if !hasPermission {
			return errors.New("insufficient permissions for this operation")
		}
	}

	return nil
}
