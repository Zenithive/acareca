package billing

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
)

// RegisterRoutes registers JWT-protected billing routes on the provided v1 group.
func RegisterRoutes(v1 *gin.RouterGroup, h *Handler, cfg *config.Config) {
	// Public plan listing — no auth required
	v1.GET("/billing/subscriptions", h.Subscriptions)

	// Practitioner-facing billing routes
	billing := v1.Group("/billing")
	billing.Use(middleware.Auth(cfg))
	billing.POST("/checkout", h.Checkout)
	billing.POST("/portal", h.Portal)

	// Admin billing history
	admin := v1.Group("/admin/billing")
	admin.Use(middleware.Auth(cfg))
	admin.GET("/history", h.BillingHistory)
}

// RegisterWebhookRoute registers the public Stripe webhook route.
// This MUST be called before any JSON body-parsing middleware is applied,
// because Stripe signature verification requires the raw request body.
func RegisterWebhookRoute(webhookGroup *gin.RouterGroup, h *Handler) {
	webhookGroup.POST("/stripe", h.Webhook)
}
