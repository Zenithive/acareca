package accountant

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// @Summary List all accountants with their practitioners (Admin)
// @Description Fetch a paginated list of all accountants with their associated practitioners. Admin only.
// @Tags admin-accountant
// @Produce json
// @Param email query string false "Filter by exact email address"
// @Param name query string false "Filter by accountant full name (partial match)"
// @Param phone query string false "Filter by phone number (partial match)"
// @Param search query string false "Search across accountant full name and email"
// @Param limit query int false "Number of records per page (default: 10, max: 100)"
// @Param offset query int false "Number of records to skip (default: 0)"
// @Param sort_by query string false "Sort field: id, first_name, last_name, email (default: id)"
// @Param order_by query string false "Sort order: ASC or DESC (default: DESC)"
// @Success 200 {object} util.RsList
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/accountant [get]
func (h *Handler) ListAccountantsWithPractitioners(c *gin.Context) {
	var filter Filter
	if err := util.BindAndValidate(c, &filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	list, err := h.svc.ListAccountantsWithPractitioners(c.Request.Context(), &filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, list, "Accountants with practitioners fetched successfully")
}
