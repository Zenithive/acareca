package clinic

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/limits"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	Create(c *gin.Context)
	List(c *gin.Context)
	GetByID(c *gin.Context)
	Update(c *gin.Context)
	BulkUpdate(c *gin.Context)
	Delete(c *gin.Context)
	BulkDelete(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) IHandler {
	return &handler{svc: svc}
}

// parseClinicID parses the "id" path param and writes a 400 on failure.
func parseClinicID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid clinic id"))
		return uuid.Nil, false
	}
	return id, true
}

// handleServiceError handles common service errors and returns appropriate status codes.
func handleServiceError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrNotFound) {
		response.Error(c, http.StatusNotFound, err)
		return true
	}
	if errors.Is(err, limits.ErrLimitReached) {
		response.Error(c, http.StatusForbidden, err)
		return true
	}
	response.Error(c, http.StatusInternalServerError, err)
	return true
}

// @Summary Create a new clinic
// @Tags clinic
// @Accept json
// @Produce json
// @Security BearerToken
// @Param request body RqCreateClinic true "Clinic Data"
// @Success 201 {object} RsClinic
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 403 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /clinic [post]
func (h *handler) Create(c *gin.Context) {
	actorID, role, ok := util.GetRoleBasedID(c)
	if !ok {
		return
	}

	var req RqCreateClinic
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	clinic, err := h.svc.CreateClinic(c.Request.Context(), *actorID, role, &req)
	if handleServiceError(c, err) {
		return
	}

	response.JSON(c, http.StatusCreated, clinic, "Clinic created successfully")
}

// @Summary List clinics for the logged-in user
// @Tags clinic
// @Produce json
// @Security BearerToken
// @Param name query string false "Filter by clinic name"
// @Param id query string false "Filter by clinic ID"
// @Param is_active query boolean false "Filter by active status"
// @Param search query string false "Search across name, abn, description"
// @Param sort_by query string false "Sort field (name, is_active, created_at)"
// @Param order_by query string false "Sort direction (ASC, DESC)"
// @Param limit query int false "Page size (default 10, max 100)"
// @Param offset query int false "Page offset"
// @Success 200 {object} util.RsList
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 403 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /clinic [get]
func (h *handler) List(c *gin.Context) {
	actorID, role, ok := util.GetRoleBasedID(c)
	if !ok {
		return
	}

	var filter Filter
	if err := util.BindAndValidate(c, &filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	var (
		clinics interface{}
		err     error
	)

	switch role {
	case util.RoleAccountant:
		clinics, err = h.svc.ListClinicsForAccountant(c.Request.Context(), *actorID, filter)
	case util.RolePractitioner:
		clinics, err = h.svc.ListClinic(c.Request.Context(), *actorID, filter)
	default:
		response.Error(c, http.StatusForbidden, errors.New("invalid role"))
		return
	}

	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, clinics, "Clinics fetched successfully")
}

// @Summary Get clinic by ID
// @Tags clinic
// @Produce json
// @Security BearerToken
// @Param id path string true "Clinic UUID"
// @Success 200 {object} RsClinic
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 403 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /clinic/{id} [get]
func (h *handler) GetByID(c *gin.Context) {
	actorID, _, ok := util.GetRoleBasedID(c)
	if !ok {
		return
	}

	id, ok := parseClinicID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, errors.New("profile not found"))
		return
	}

	clinic, err := h.svc.GetClinicByID(c.Request.Context(), *actorID, id)
	if handleServiceError(c, err) {
		return
	}

	response.JSON(c, http.StatusOK, clinic, "Clinic fetched successfully")
}

// @Summary Update clinic details
// @Tags clinic
// @Accept json
// @Produce json
// @Security BearerToken
// @Param id path string true "Clinic UUID"
// @Param request body RqUpdateClinic true "Updated Clinic Data"
// @Success 200 {object} RsClinic
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 403 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /clinic/{id} [put]
func (h *handler) Update(c *gin.Context) {
	actorID, role, ok := util.GetRoleBasedID(c)
	if !ok {
		return
	}

	id, ok := parseClinicID(c)
	if !ok {
		return
	}

	var req RqUpdateClinic
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	clinic, err := h.svc.UpdateClinic(c.Request.Context(), *actorID, role, id, &req)
	if handleServiceError(c, err) {
		return
	}

	response.JSON(c, http.StatusOK, clinic, "Clinic updated successfully")
}

// @Summary Delete a clinic
// @Tags clinic
// @Produce json
// @Security BearerToken
// @Param id path string true "Clinic UUID"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 403 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /clinic/{id} [delete]
func (h *handler) Delete(c *gin.Context) {
	actorID, ok := util.GetUserID(c)
	if !ok {
		return
	}

	id, ok := parseClinicID(c)
	if !ok {
		return
	}

	if handleServiceError(c, h.svc.DeleteClinic(c.Request.Context(), actorID, id)) {
		return
	}

	response.JSON(c, http.StatusOK, nil, "Clinic deleted successfully")
}

// @Summary Bulk update clinics
// @Tags clinic
// @Accept json
// @Produce json
// @Security BearerToken
// @Param request body RqBulkUpdateClinic true "Bulk Update Data"
// @Success 200 {object} util.RsList
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 403 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /clinic/bulk-update [put]
func (h *handler) BulkUpdate(c *gin.Context) {
	practID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}

	var req RqBulkUpdateClinic
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	clinics, err := h.svc.BulkUpdateClinics(c.Request.Context(), practID, &req)
	if handleServiceError(c, err) {
		return
	}

	response.JSON(c, http.StatusOK, util.RsList{Items: clinics, Total: len(clinics)}, "Clinics updated successfully")
}

// @Summary Bulk delete clinics
// @Tags clinic
// @Accept json
// @Produce json
// @Security BearerToken
// @Param request body RqBulkDeleteClinic true "Bulk Delete Data"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 403 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /clinic/bulk-delete [delete]
func (h *handler) BulkDelete(c *gin.Context) {
	practID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}

	var req RqBulkDeleteClinic
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	if handleServiceError(c, h.svc.BulkDeleteClinics(c.Request.Context(), practID, &req)) {
		return
	}

	response.JSON(c, http.StatusOK, nil, "Clinics deleted successfully")
}
