package practitioner

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// @Summary List all practitioners with active subscriptions (Admin)
// @Description Fetch a paginated list of all practitioners with their active subscription details. Admin only.
// @Tags admin-practitioner
// @Produce json
// @Param email query string false "Filter by exact email address"
// @Param name query string false "Filter by practitioner full name (partial match)"
// @Param phone query string false "Filter by phone number (partial match)"
// @Param has_active_subscription query bool false "Filter by active subscription (true=with active, false=without active)"
// @Param subscription_name query string false "Filter by subscription plan name (partial match)"
// @Param search query string false "Search across practitioner full name and email"
// @Param limit query int false "Number of records per page (default: 10, max: 100)"
// @Param offset query int false "Number of records to skip (default: 0)"
// @Param sort_by query string false "Sort field: id, first_name, last_name, email, created_at (default: created_at)"
// @Param order_by query string false "Sort order: ASC or DESC (default: DESC)"
// @Success 200 {object} util.RsList
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/practitioner [get]
func (h *Handler) ListPractitionersWithSubscriptions(c *gin.Context) {
	var filter Filter
	if err := util.BindAndValidate(c, &filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	list, err := h.svc.ListPractitionersWithSubscriptions(c.Request.Context(), &filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, list, "Practitioners with subscriptions fetched successfully")
}

// @Summary Get practitioner by ID with subscription (Admin)
// @Description Fetch a single practitioner with their active subscription details by ID. Admin only.
// @Tags admin-practitioner
// @Produce json
// @Param id path string true "Practitioner ID (UUID)"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/practitioner/{id} [get]
func (h *Handler) GetPractitionerWithSubscription(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid practitioner id"))
		return
	}

	practitioner, err := h.svc.GetPractitionerWithSubscription(c.Request.Context(), id)
	if err != nil {
		response.Error(c, http.StatusNotFound, errors.New("practitioner not found"))
		return
	}

	response.JSON(c, http.StatusOK, practitioner, "Practitioner fetched successfully")
}


