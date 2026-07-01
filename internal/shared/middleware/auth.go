package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
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

		c.Next()
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
