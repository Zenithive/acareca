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

// @Summary Get practitioner by ID
// @Tags practitioner
// @Produce json
// @Param id path string true "Practitioner ID"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Security BearerToken
// @Router /practitioner/{id} [get]
func (h *Handler) GetPractitioner(c *gin.Context) {
	id, err := h.parseUUID(c, c.Param("id"), "invalid practitioner id")
	if err != nil {
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
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /practitioner [get]
func (h *Handler) ListPractitioners(c *gin.Context) {
	actorID, role, err := h.getActorContext(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, err)
		return
	}

	var filter Filter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	if actorID != nil && role == util.RoleAccountant {
		filter.AccountantID = actorID
	}

	list, err := h.svc.ListPractitioners(c, &filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list, "")
}

// @Summary Get Practitioner Lock Date
// @Description Retrieve the current financial lock date for the authenticated practitioner or associated practitioners.
// @Tags practitioner-lock-date
// @Produce json
// @Param practitioner_id query string false "Practitioner ID (required for accountants)"
// @Param financial_year_id query string true "Financial Year ID"
// @Success 200 {object} util.RsList
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 403 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /practitioner/lock-date [get]
func (h *Handler) GetLockDate(c *gin.Context) {
	_, role, err := h.getActorContext(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, err)
		return
	}

	var practitionerID uuid.UUID

	switch role {
	case util.RolePractitioner:
		pID, ok := util.GetPractitionerID(c)
		if !ok {
			response.Error(c, http.StatusUnauthorized, errors.New("practitioner not found"))
			return
		}
		practitionerID = pID

	case util.RoleAccountant:
		pIDStrs := c.QueryArray("practitioner_id")
		if len(pIDStrs) == 0 {
			response.Error(c, http.StatusBadRequest, errors.New("practitioner_id is required for accountants"))
			return
		}

		for _, idStr := range pIDStrs {
			pID, err := h.parseUUID(c, idStr, "invalid ID format")
			if err != nil {
				return
			}
			practitionerID = pID
		}
	}

	fyID, err := h.parseUUID(c, c.Query("financial_year_id"), "invalid financial_year_id")
	if err != nil {
		return
	}

	results, err := h.svc.GetLockDate(c.Request.Context(), practitionerID, fyID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, results, "Lock dates fetched successfully")
}

type UpdateLockDateRequest struct {
	FinancialYearID string  `json:"financial_year_id" binding:"required"`
	LockDate        *string `json:"lock_date"`
}

// @Summary Update Practitioner Lock Date
// @Description Set or remove the financial lock date for the authenticated practitioner.
// @Tags practitioner-lock-date
// @Accept json
// @Produce json
// @Param request body UpdateLockDateRequest true "Lock Date Update"
// @Success 200 {object} util.RsList
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /practitioner/lock-date [patch]
func (h *Handler) UpdateLockDate(c *gin.Context) {
	var req UpdateLockDateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	practitionerID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}

	fyID, err := h.parseUUID(c, req.FinancialYearID, "invalid financial_year_id format")
	if err != nil {
		return
	}

	err = h.svc.UpdateLockDate(c.Request.Context(), practitionerID, fyID, req.LockDate)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, nil, "Lock date updated successfully")
}

func (h *Handler) getActorContext(c *gin.Context) (*uuid.UUID, string, error) {
	actorID, role, ok := util.GetRoleBasedID(c)
	if !ok {
		return nil, "", errors.New("user role not authorized")
	}
	return actorID, role, nil
}

func (h *Handler) parseUUID(c *gin.Context, rawStr string, errMsg string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(rawStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New(errMsg))
		return uuid.Nil, err
	}
	return parsed, nil
}
