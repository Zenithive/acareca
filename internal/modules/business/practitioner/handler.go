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
	svc IService
}

func NewHandler(svc IService) *Handler {
	return &Handler{svc: svc}
}

// GetPractitioner godoc
// @Summary Get practitioner by ID
// @Tags practitioner
// @Produce json
// @Param id path string true "Practitioner ID"
// @Success 200 {object} RsPractitioner
// @Router /practitioner/{id} [get]
// @Security BearerToken
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

// ListPractitioners godoc
// @Summary List all practitioners
// @Tags practitioner
// @Produce json
// @Success 200 {object} util.RsList
// @Router /practitioner [get]
// @Security BearerToken
func (h *Handler) ListPractitioners(c *gin.Context) {
	list, err := h.svc.ListPractitioners(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, util.RsList{Items: list, Total: len(list)}, "")
}
