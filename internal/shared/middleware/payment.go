package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenScope string

const (
	ScopeFullAccess  TokenScope = "full_access"
	ScopePaymentOnly TokenScope = "payment_only"
)

type JWTClaims struct {
	UserID uuid.UUID  `json:"user_id"`
	Scope  TokenScope `json:"scope"`
	jwt.RegisteredClaims
}

const claimsKey = "claims"

// ===============================
// Generate Payment Token
// ===============================
func GeneratePaymentToken(userID uuid.UUID, secret string) (string, error) {
	claims := JWTClaims{
		UserID: userID,
		Scope:  ScopePaymentOnly,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ===============================
//
//	Bearer Token Auth Middleware
//
// ===============================
func JWTAuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		tokenStr := parts[1]

		token, err := jwt.ParseWithClaims(tokenStr, &JWTClaims{}, func(token *jwt.Token) (any, error) {
			// ensure signing method is HMAC
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("invalid signing method")
			}
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(*JWTClaims)
		if !ok {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// store claims in context
		c.Set(claimsKey, claims)

		c.Next()
	}
}

// ===============================
//
//	Payment Scope Middleware
//
// ===============================
func PaymentAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		claimsValue, exists := c.Get(claimsKey)
		if !exists {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		claims, ok := claimsValue.(*JWTClaims)
		if !ok {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		if claims.Scope != ScopeFullAccess &&
			claims.Scope != ScopePaymentOnly {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		//  SET USER ID INTO CONTEXT (IMPORTANT PART)
		c.Set("user_id", claims.UserID)
		c.Set("scope", claims.Scope)

		c.Next()
	}
}
