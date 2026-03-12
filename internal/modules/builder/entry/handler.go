package entry

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	Create(c *gin.Context)
	Get(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	List(c *gin.Context)
}

type handler struct {
	svc IService
}

func NewHandler(svc IService) IHandler {
	return &handler{svc: svc}
}

// @Summary Create a new form entry
// @Description Create a new entry for a specific form version
// @Tags entry
// @Accept json
// @Produce json
// @Param version_id path string true "Version ID"
// @Param request body RqFormEntry true "Entry details"
// @Success 201 {object} RsFormEntry
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /entry/version/{version_id} [post]
func (h *handler) Create(c *gin.Context) {
	versionID, ok := util.ParseUuidID(c, "version_id")
	if !ok {
		return
	}
	var req RqFormEntry
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	var submittedBy *uuid.UUID
	created, err := h.svc.Create(c.Request.Context(), versionID, &req, submittedBy)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusCreated, created)
}

// @Summary Get a form entry by ID
// @Description Fetch details of a specific entry
// @Tags entry
// @Accept json
// @Produce json
// @Param id path string true "Entry ID"
// @Success 200 {object} RsFormEntry
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /entry/{id} [get]
func (h *handler) Get(c *gin.Context) {
	id, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}
	e, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, e)
}

// @Summary Update a form entry
// @Description Update data for an existing entry
// @Tags entry
// @Accept json
// @Produce json
// @Param id path string true "Entry ID"
// @Param request body RqUpdateFormEntry true "Updated details"
// @Success 200 {object} RsFormEntry
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /entry/{id} [put]
func (h *handler) Update(c *gin.Context) {
	id, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}

	var req RqUpdateFormEntry
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	var submittedBy *uuid.UUID
	updated, err := h.svc.Update(c.Request.Context(), id, &req, submittedBy)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, updated)
}

// @Summary Delete a form entry
// @Description Remove an entry from the system
// @Tags entry
// @Accept json
// @Produce json
// @Param id path string true "Entry ID"
// @Success 204 "No Content"
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /entry/{id} [delete]
func (h *handler) Delete(c *gin.Context) {
	id, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusNoContent, nil)
}

// @Summary List form entries
// @Description List all entries for a specific version and clinic
// @Tags entry
// @Accept json
// @Produce json
// @Param version_id path string true "Version ID"
// @Success 200 {array} RsFormEntry
// @Failure 500 {object} response.RsError
// @Router /entry/version/{version_id} [get]
func (h *handler) List(c *gin.Context) {
	versionID, ok := util.ParseUuidID(c, "version_id")
	if !ok {
		return
	}

	var filter Filter
	if err := util.BindAndValidate(c, &filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	list, err := h.svc.List(c.Request.Context(), versionID, filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list)
}
