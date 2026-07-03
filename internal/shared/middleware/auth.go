package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
)

var (
	errUnauthorized = errors.New("unauthorized")
	errForbidden    = errors.New("forbidden")
)

type SuperadminChecker func(ctx context.Context, userID string) (bool, error)

func RequireSuperadmin(check SuperadminChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get(util.UserIDKey)
		if !exists {
			response.Error(c, http.StatusUnauthorized, errUnauthorized)
			return
		}
		id, ok := userID.(string)
		if !ok || id == "" {
			response.Error(c, http.StatusUnauthorized, errUnauthorized)
			return
		}
		isAdmin, err := check(c.Request.Context(), id)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		if !isAdmin {
			response.Error(c, http.StatusForbidden, errForbidden)
			return
		}
		c.Next()
	}
}

func Auth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenStr string
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		} else if q := c.Query("token"); q != "" {
			tokenStr = q
		} else {
			response.Error(c, http.StatusUnauthorized, errUnauthorized)
			c.Abort()
			return
		}

		claims := &util.CustomClaims{}

		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
			if t.Method != jwt.SigningMethodHS256 {
				return nil, errUnauthorized
			}
			return []byte(cfg.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			response.Error(c, http.StatusUnauthorized, errUnauthorized)
			c.Abort()
			return
		}

		if claims.Subject == "" || claims.ID == "" || claims.Role == "" {
			response.Error(c, http.StatusUnauthorized, errUnauthorized)
			c.Abort()
			return
		}

		entityUUID, err := util.ParseUUID(claims.ID)

		if err != nil {
			response.Error(c, http.StatusUnauthorized, errUnauthorized)
			c.Abort()
			return
		}

		c.Set(util.UserIDKey, claims.Subject)
		c.Set(util.EntityIDKey, entityUUID)
		c.Set("role", claims.Role)
		c.Set(util.SubscriptionStatusKey, claims.SubscriptionStatus)

		c.Next()
	}
}

// RequireActiveSubscription blocks practitioner requests when subscription_status is PENDING.
// If JWT says PENDING, checks the database as a fallback — if DB says COMPLETE, allows through.
// This handles the stale token case after a successful Stripe payment.
// Apply this only on business routes — NOT on auth, billing, or subscription routes.
func RequireActiveSubscription(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role != util.RolePractitioner {
			c.Next()
			return
		}
		
		status, _ := c.Get(util.SubscriptionStatusKey)
		
		// If JWT says COMPLETE, allow immediately (hot path)
		if status == util.SubscriptionStatusComplete {
			c.Next()
			return
		}
		
		// JWT says PENDING — check DB as fallback (handles stale token after payment)
		entityID, exists := c.Get(util.EntityIDKey)
		if !exists {
			response.Error(c, http.StatusUnauthorized, errors.New("entity id not found"))
			c.Abort()
			return
		}
		
		practitionerID, ok := entityID.(uuid.UUID)
		if !ok {
			response.Error(c, http.StatusInternalServerError, errors.New("invalid entity id type"))
			c.Abort()
			return
		}
		
		// Query DB for latest subscription_status
		var dbStatus string
		query := `SELECT subscription_status FROM tbl_practitioner WHERE id = $1 AND deleted_at IS NULL`
		if err := db.GetContext(c.Request.Context(), &dbStatus, query, practitionerID); err != nil {
			// DB error — reject to be safe
			response.Error(c, http.StatusInternalServerError, errors.New("failed to verify subscription status"))
			c.Abort()
			return
		}
		
		// If DB says COMPLETE, allow through even though JWT is stale
		if dbStatus == util.SubscriptionStatusComplete {
			c.Next()
			return
		}
		
		// Both JWT and DB say PENDING — block
		response.Error(c, http.StatusPaymentRequired, errors.New("subscription payment required"))
		c.Abort()
	}
}

func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get role from context
		role, exists := c.Get("role")
		if !exists {
			response.Error(c, http.StatusUnauthorized, nil)
			c.Abort()
			return
		}

		userRole := role.(string)
		for _, r := range allowedRoles {
			if r == userRole {
				c.Next()
				return
			}
		}

		response.Error(c, http.StatusForbidden, nil)
		c.Abort()
	}
}
