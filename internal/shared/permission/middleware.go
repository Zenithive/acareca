package permission

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func Middleware(service Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		permCtx, err := ExtractContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: " + err.Error()})
			c.Abort()
			return
		}

		c.Set("permission_context", permCtx)
		c.Set("permission_service", service)

		c.Next()
	}
}

func RequirePermission(service Service, resource Resource, action Action) gin.HandlerFunc {
	return func(c *gin.Context) {
		permCtx, err := ExtractContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: " + err.Error()})
			c.Abort()
			return
		}

		result := service.Check(c.Request.Context(), permCtx, resource, action)
		if !result.Allowed {
			c.JSON(http.StatusForbidden, gin.H{
				"error":    "permission denied",
				"resource": resource,
				"action":   action,
				"reason":   result.Reason,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func RequireAnyPermission(service Service, checks ...PermissionCheck) gin.HandlerFunc {
	return func(c *gin.Context) {
		permCtx, err := ExtractContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: " + err.Error()})
			c.Abort()
			return
		}

		for _, check := range checks {
			result := service.Check(c.Request.Context(), permCtx, check.Resource, check.Action)
			if result.Allowed {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"error":  "permission denied",
			"reason": "none of the required permissions are granted",
		})
		c.Abort()
	}
}

func RequireAllPermissions(service Service, checks ...PermissionCheck) gin.HandlerFunc {
	return func(c *gin.Context) {
		permCtx, err := ExtractContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: " + err.Error()})
			c.Abort()
			return
		}

		for _, check := range checks {
			result := service.Check(c.Request.Context(), permCtx, check.Resource, check.Action)
			if !result.Allowed {
				c.JSON(http.StatusForbidden, gin.H{
					"error":    "permission denied",
					"resource": check.Resource,
					"action":   check.Action,
					"reason":   result.Reason,
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

func ExtractContext(c *gin.Context) (Context, error) {
	if ctx, exists := c.Get("permission_context"); exists {
		if permCtx, ok := ctx.(Context); ok {
			return permCtx, nil
		}
	}

	userIDStr := c.GetString("user_id")
	if userIDStr == "" {
		return Context{}, &PermissionError{Reason: "user_id not found in context"}
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return Context{}, &PermissionError{Reason: "invalid user_id format"}
	}

	role := c.GetString("role")
	if role == "" {
		return Context{}, &PermissionError{Reason: "role not found in context"}
	}

	permCtx := Context{
		UserID:   userID,
		Role:     Role(strings.ToLower(role)),
		Metadata: make(map[string]interface{}),
	}

	if accountantIDStr := c.GetString("accountant_id"); accountantIDStr != "" {
		if accountantID, err := uuid.Parse(accountantIDStr); err == nil {
			permCtx.AccountantID = &accountantID
		}
	}

	if entityIDStr := c.Param("id"); entityIDStr != "" {
		if entityID, err := uuid.Parse(entityIDStr); err == nil {
			permCtx.EntityID = &entityID
		}
	} else if entityIDStr := c.Query("entity_id"); entityIDStr != "" {
		if entityID, err := uuid.Parse(entityIDStr); err == nil {
			permCtx.EntityID = &entityID
		}
	}

	if entityType := c.Query("entity_type"); entityType != "" {
		permCtx.EntityType = &entityType
	}

	return permCtx, nil
}

func GetPermissionContext(c *gin.Context) (Context, bool) {
	ctx, exists := c.Get("permission_context")
	if !exists {
		return Context{}, false
	}

	permCtx, ok := ctx.(Context)
	return permCtx, ok
}

func GetPermissionService(c *gin.Context) (Service, bool) {
	svc, exists := c.Get("permission_service")
	if !exists {
		return nil, false
	}

	permSvc, ok := svc.(Service)
	return permSvc, ok
}

func CheckPermission(c *gin.Context, resource Resource, action Action) CheckResult {
	service, ok := GetPermissionService(c)
	if !ok {
		return Deny("permission service not found")
	}

	permCtx, ok := GetPermissionContext(c)
	if !ok {
		return Deny("permission context not found")
	}

	return service.Check(c.Request.Context(), permCtx, resource, action)
}

func EnforcePermission(c *gin.Context, resource Resource, action Action) error {
	service, ok := GetPermissionService(c)
	if !ok {
		return &PermissionError{Reason: "permission service not found"}
	}

	permCtx, ok := GetPermissionContext(c)
	if !ok {
		return &PermissionError{Reason: "permission context not found"}
	}

	return service.Enforce(c.Request.Context(), permCtx, resource, action)
}
