package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

// AuditContext enriches the request context with audit metadata
// This middleware should be placed after Auth and ClientInfo middlewares
func AuditContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Extract user ID from auth middleware
		if userID, exists := c.Get(util.UserIDKey); exists {
			if id, ok := userID.(string); ok && id != "" {
				ctx = audit.WithUserID(ctx, id)
			}
		}

		// 2. ADD THIS: Extract Role and set it as UserType
		if role, exists := c.Get("role"); exists {
			if r, ok := role.(string); ok && r != "" {
				// This is the missing link!
				ctx = audit.WithUserType(ctx, r)
			}
		}

		// Extract practitioner ID (used as practice_id)
		if practitionerID, exists := c.Get(util.EntityIDKey); exists {
			if id, ok := practitionerID.(uuid.UUID); ok && id != uuid.Nil {
				idStr := id.String()
				ctx = audit.WithPracticeID(ctx, idStr)
			}
		}

		// Extract IP address from client_info middleware
		if ip := IPFromCtx(ctx); ip != "" {
			ctx = audit.WithIPAddress(ctx, ip)
		}

		// Extract user agent from client_info middleware
		if ua := UserAgentFromCtx(ctx); ua != "" {
			ctx = audit.WithUserAgent(ctx, ua)
		}

		// Update request context
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
