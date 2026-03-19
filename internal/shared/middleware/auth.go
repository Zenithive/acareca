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

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			response.Error(c, http.StatusUnauthorized, errUnauthorized)
			c.Abort()
			return
		}

		tokenStr := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))

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

		if claims.Subject == "" || claims.PractitionerID == "" {
			response.Error(c, http.StatusUnauthorized, errUnauthorized)
			c.Abort()
			return
		}

		practitionerUUID, err := uuid.Parse(claims.PractitionerID)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, errUnauthorized)
			c.Abort()
			return
		}

		c.Set(util.UserIDKey, claims.Subject)
		c.Set(util.PractitionerIDKey, practitionerUUID)

		c.Next()
	}
}
