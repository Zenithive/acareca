package billing

import (
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

// Handler holds the billing Service dependency.
type Handler struct {
	svc Service
}

// NewHandler constructs a billing Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// Checkout handles POST /billing/checkout
func (h *Handler) Checkout(c *gin.Context) {
	practitionerID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}

	var req RqCheckout
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	session, err := h.svc.CreateCheckoutSession(c.Request.Context(), practitionerID, &req)
	if err != nil {
		switch {
		case errors.Is(err, ErrTrialPlanNotPurchasable):
			response.Error(c, http.StatusBadRequest, err)
		case errors.Is(err, ErrMissingStripePriceID):
			response.Error(c, http.StatusBadRequest, err)
		case errors.Is(err, ErrAlreadyActiveSubscription):
			response.Error(c, http.StatusConflict, err)
		default:
			response.Error(c, http.StatusInternalServerError, err)
		}
		return
	}

	response.JSON(c, http.StatusOK, session, "checkout session created")
}

// Portal handles POST /billing/portal
func (h *Handler) Portal(c *gin.Context) {
	practitionerID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}

	session, err := h.svc.CreatePortalSession(c.Request.Context(), practitionerID)
	if err != nil {
		if errors.Is(err, ErrNoBillingAccount) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, session, "portal session created")
}

// BillingHistory handles GET /admin/billing/history
func (h *Handler) BillingHistory(c *gin.Context) {
	var f BillingHistoryFilter
	if err := util.BindAndValidate(c, &f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	list, err := h.svc.ListBillingHistory(c.Request.Context(), &f)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, list, "billing history retrieved")
}

// Webhook handles POST /webhooks/stripe (raw body, no JWT)
func (h *Handler) Webhook(c *gin.Context) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	sigHeader := c.GetHeader("Stripe-Signature")

	if err := h.svc.HandleWebhook(c.Request.Context(), payload, sigHeader); err != nil {
		if errors.Is(err, ErrInvalidWebhookSignature) {
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		log.Printf("webhook processing error: %v", err)
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, nil, "webhook processed")
}

// Subscriptions handles GET /billing/subscriptions — public list of active plans
func (h *Handler) Subscriptions(c *gin.Context) {
	plans, err := h.svc.ListSubscriptions(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, plans, "subscriptions retrieved")
}
