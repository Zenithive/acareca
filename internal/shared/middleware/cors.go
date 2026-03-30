package middleware

import (
	"log"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/pkg/config"
)

// CORS returns a gin CORS middleware configured from the environment.
// Allowed origins are read from ALLOWED_ORIGINS (comma-separated).
// Falls back to cfg.LocalUrl when the variable is not set.
func CORS(cfg *config.Config) gin.HandlerFunc {
	allowedOrigins := cfg.AllowedOrigins
	var origins []string
	allowCredentials := true

	if allowedOrigins == "" {
		origins = []string{cfg.LocalUrl}
		log.Printf("[WARN] ALLOWED_ORIGINS not set — falling back to LOCAL_API_URL=%q\n", cfg.LocalUrl)
	} else {
		for _, o := range strings.Split(allowedOrigins, ",") {
			trimmed := strings.TrimSpace(o)
			if trimmed == "" {
				continue
			}
			origins = append(origins, trimmed)
			if trimmed == "*" {
				// credentials cannot be used with a wildcard origin
				allowCredentials = false
			}
		}
	}

	return cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept", "X-Requested-With"},
		ExposeHeaders:    []string{"Authorization", "token"},
		AllowCredentials: allowCredentials,
		MaxAge:           12 * time.Hour,
	})
}
