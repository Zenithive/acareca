package practitioner

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/response"
)

type Handler struct {
	svc IService
}

func NewHandler(svc IService) *Handler {
	return &Handler{svc: svc}
}

// @Summary Get practitioner by ID
// @Tags practitioner
// @Produce json
// @Param id path string true "Practitioner ID"
// @Success 200 {object} response.RsBase
// @Security BearerToken
// @Router /practitioner/{id} [get]
func (h *Handler) GetPractitioner(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid practitioner id"))
		return
	}
	p, err := h.svc.GetPractitioner(c.Request.Context(), id)
	if err != nil {
		response.Error(c, http.StatusNotFound, errors.New("practitioner not found"))
		return
	}
	response.JSON(c, http.StatusOK, p, "")
}

// @Summary List all practitioners
// @Description Fetch a list of practitioners.
// @Tags practitioner
// @Produce json
// @Param id query string false "Filter by Practitioner UUID"
// @Param email query string false "Filter by exact email"
// @Param first_name query string false "Filter by exact first name"
// @Param last_name query string false "Filter by exact last name"
// @Param phone query string false "Filter by exact phone number"
// @Param search query string false "Fuzzy search across name and email"
// @Param limit query int false "Limit for pagination (default 20)"
// @Param offset query int false "Offset for pagination"
// @Success 200 {object} util.RsList
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /practitioner [get]
func (h *Handler) ListPractitioners(c *gin.Context) {
	list, err := h.svc.ListPractitioners(c.Request.Context(), &Filter{})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list, "")
}
