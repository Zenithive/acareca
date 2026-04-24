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
	actorID, role, ok := util.GetRoleBasedID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, errors.New("user role not authorized"))
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
// @Description Retrieve the current financial lock date for the authenticated practitioner.
// @Tags practitioner-lock-date
// @Produce json
// @Param financial_year_id query string true "Financial Year ID"
// @Success 200 {object} util.RsList
// @Failure 401 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /practitioner/lock-date [get]
func (h *Handler) GetLockDate(c *gin.Context) {

	// 2. Parse the UUID
	practitionerID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}

	// 2. Get Financial Year ID from Query Params
	fyIDStr := c.Query("financial_year_id")
	if fyIDStr == "" {
		response.Error(c, http.StatusBadRequest, errors.New("financial_year_id is required"))
		return
	}

	fyID, err := uuid.Parse(fyIDStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid financial_year_id format"))
		return
	}

	lockDate, err := h.svc.GetLockDate(c.Request.Context(), practitionerID, fyID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, gin.H{"lock_date": lockDate}, "Lock date fetched successfully")
}

type UpdateLockDateRequest struct {
	// Use *time.Time to allow null values for removing the lock date
	FinancialYearID string  `json:"financial_year_id" binding:"required"`
	LockDate        *string `json:"lock_date"`
}

// @Summary Update Practitioner Lock Date
// @Description Set or remove (by sending null) the financial lock date for the authenticated practitioner.
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

	// fyIDStr := c.Query("financial_year_id")
	// if fyIDStr == "" {
	// 	response.Error(c, http.StatusBadRequest, errors.New("financial_year_id is required"))
	// 	return
	// }

	fyID, err := uuid.Parse(req.FinancialYearID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid financial_year_id format"))
		return
	}

	err = h.svc.UpdateLockDate(c.Request.Context(), practitionerID, fyID, req.LockDate)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, nil, "Lock date updated successfully")
}
