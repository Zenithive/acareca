package form

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/limits"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	GetById(c *gin.Context)
	CreateFormWithFields(c *gin.Context)
	UpdateFormWithFields(c *gin.Context)
	GetFormWithFields(c *gin.Context)
	List(c *gin.Context)
	Delete(c *gin.Context)
	UpdateFormStatus(c *gin.Context)
}

type handler struct {
	svc IService
}

func NewHandler(svc IService) IHandler {
	return &handler{svc: svc}
}

// // @Summary Bulk sync fields
// // @Description Synchronize multiple fields for a practitioner
// // @Tags form
// // @Accept json
// // @Produce json
// // @Param request body RqBulkSyncFields true "Sync request"
// // @Success 200 {object} RsBulkSyncFields
// // @Failure 400 {object} response.RsError
// // @Failure 500 {object} response.RsError
// // @Router /form/sync [post]
// func (h *handler) Sync(c *gin.Context) {
// 	practitionerID, ok := util.GetPractitionerID(c)
// 	if !ok {
// 		return
// 	}

// 	var req RqBulkSyncFields
// 	if err := util.BindAndValidate(c, &req); err != nil {
// 		response.Error(c, http.StatusBadRequest, err)
// 		return
// 	}
// 	result, err := h.svc.BulkSyncFields(c.Request.Context(), practitionerID, &req)
// 	if err != nil {
// 		response.Error(c, http.StatusInternalServerError, err)
// 		return
// 	}
// 	response.JSON(c, http.StatusOK, result, "Fields synchronized successfully")
// }

// @Summary Get form by ID (basic)
// @Description fetch form detail
// @Tags form
// @Accept json
// @Produce json
// @Param id path string true "Form ID"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /form/{id} [get]
func (h *handler) GetById(c *gin.Context) {
	var formId uuid.UUID
	var ok bool
	formId, ok = util.ParseUuidID(c, "id")
	if !ok {
		response.Error(c, http.StatusBadRequest, errors.New("invaild form id"))
		return
	}

	form, err := h.svc.GetFormByID(c.Request.Context(), formId)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, gin.H{"form": form}, "Form fetch successfully")
}

// @Summary Create form with fields
// @Description Create a new custom form along with its associated fields
// @Tags form
// @Accept json
// @Produce json
// @Param request body RqCreateFormWithFields true "Form creation request"
// @Success 201 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /form [post]
func (h *handler) CreateFormWithFields(c *gin.Context) {
	// practitionerID, ok := util.GetPractitionerID(c)
	// if !ok {
	// 	return
	// }

	// Get Role and appropriate ID
	role := c.GetString("role")
	var actorID uuid.UUID
	var ok bool

	if strings.EqualFold(role, util.RoleAccountant) {
		actorID, ok = util.GetAccountantID(c)
	} else {
		actorID, ok = util.GetPractitionerID(c)
	}

	if !ok {
		response.Error(c, http.StatusUnauthorized, nil)
		return
	}
	var req RqCreateFormWithFields
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	if req.Status == "" {
		req.Status = StatusDraft
	}

	if err := req.ValidateShares(); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	form, syncResult, err := h.svc.CreateWithFields(c.Request.Context(), &req, actorID)

	if err != nil {
		if errors.Is(err, limits.ErrLimitReached) {
			response.Error(c, http.StatusForbidden, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusCreated, gin.H{"form": form, "fields_sync": syncResult}, "Form created successfully")
}

// @Summary Update form with fields
// @Description Update an existing form and sync its fields
// @Tags form
// @Accept json
// @Produce json
// @Param id path string true "Form ID"
// @Param request body RqUpdateFormWithFields true "Form update request"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /form/{id} [patch]
func (h *handler) UpdateFormWithFields(c *gin.Context) {
	var actorID, formID uuid.UUID
	var ok bool
	formID, ok = util.ParseUuidID(c, "id")
	if !ok {
		return
	}

	if formID == uuid.Nil {
		response.Error(c, http.StatusBadRequest, errors.New("form id is required"))
		return
	}

	// practitionerID, ok := util.GetPractitionerID(c)
	// if !ok {
	// 	return
	// }

	role := c.GetString("role")

	if strings.EqualFold(role, util.RoleAccountant) {
		actorID, ok = util.GetAccountantID(c)
	} else {
		actorID, ok = util.GetPractitionerID(c)
	}

	if !ok {
		response.Error(c, http.StatusUnauthorized, nil)
		return
	}

	var req RqUpdateFormWithFields
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	for i := range req.Update {
		req.Update[i].Sanitize()
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	if err := util.ValidateStruct(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	req.ID = &formID

	if err := req.ValidateShares(); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	form, syncResult, err := h.svc.UpdateWithFields(c.Request.Context(), &req, actorID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, gin.H{"form": form, "fields_sync": syncResult}, "Form updated successfully")
}

// @Summary Get form by ID
// @Description Retrieve a specific form and its fields by ID
// @Tags form
// @Accept json
// @Produce json
// @Param id path string true "Form ID"
// @Success 200 {object} response.RsBase
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /form/{id} [get]
func (h *handler) GetFormWithFields(c *gin.Context) {
	var formID uuid.UUID
	var ok bool
	formID, ok = util.ParseUuidID(c, "id")
	if !ok {
		return
	}

	out, err := h.svc.GetFormWithFields(c.Request.Context(), formID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, out, "Form fetched successfully")
}

// @Summary List forms
// @Description List all forms belonging to the practitioner's clinics. Optionally filter by clinic, method, or status. If clinic_id is omitted, all clinics are included.
// @Tags form
// @Produce json
// @Param practitioner_id  query string false "Filter by practitioner ID (UUID)"
// @Param clinic_ids  query []string false "Filter by clinic ID (UUID)"
// @Param form_name  query string false "Filter by form name (partial match)"
// @Param method     query string false "Filter by method" Enums(INDEPENDENT_CONTRACTOR, SERVICE_FEE)
// @Param status     query string false "Filter by status" Enums(DRAFT, PUBLISHED, ARCHIVED)
// @Param search     query string false "General search keyword"
// @Param sort_by    query string false "Field to sort by" Enums(status, method, clinic_id, created_at)
// @Param order_by query string false "Sort direction" Enums(ASC, DESC)
// @Param limit      query int    false "Page size"
// @Param offset     query int    false "Page offset"
// @Success 200 {object} util.RsList
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /form [get]
func (h *handler) List(c *gin.Context) {
	role := c.GetString("role")
	var actorID uuid.UUID
	if strings.EqualFold(role, util.RoleAccountant) {
		id, ok := util.GetAccountantID(c)
		if !ok {
			return
		}
		actorID = id
	} else {
		id, ok := util.GetPractitionerID(c)
		if !ok {
			return
		}
		actorID = id
	}

	var filter Filter
	if err := util.BindAndValidate(c, &filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	list, err := h.svc.List(c.Request.Context(), filter, actorID, role)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list, "Forms fetched successfully")
}

// @Summary Delete form
// @Description Remove a form by its ID
// @Tags form
// @Accept json
// @Produce json
// @Param id path string true "Form ID"
// @Success 204 "No Content"
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /form/{id} [delete]
func (h *handler) Delete(c *gin.Context) {
	var formID uuid.UUID
	var ok bool
	formID, ok = util.ParseUuidID(c, "id")
	if !ok {
		return
	}

	if err := h.svc.Delete(c.Request.Context(), formID); err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusNoContent, nil, "Form deleted successfully")
}

type RqUpdateFormStatus struct {
	Status string `json:"status" validate:"required,oneof=DRAFT PUBLISHED"`
}

// @Summary Update form status
// @Description Toggle form status between DRAFT and PUBLISHED
// @Tags form
// @Accept json
// @Produce json
// @Param id path string true "Form ID"
// @Param request body RqUpdateFormStatus true "Status update request"
// @Success 200 {object} response.RsBase
// @Security BearerToken
// @Router /form/{id}/status [patch]
func (h *handler) UpdateFormStatus(c *gin.Context) {
	var formID uuid.UUID
	var ok bool
	formID, ok = util.ParseUuidID(c, "id")
	if !ok {
		return
	}

	if formID == uuid.Nil {
		response.Error(c, http.StatusBadRequest, errors.New("form id is required"))
		return
	}

	var req RqUpdateFormStatus
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	form, err := h.svc.UpdateFormStatus(c.Request.Context(), formID, req.Status)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, gin.H{"form": form}, "Form status updated successfully")
}
