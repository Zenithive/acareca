package setting

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	CreatePractitioner(c *gin.Context)
	GetPractitioner(c *gin.Context)
	GetPractitionerByUserID(c *gin.Context)
	ListPractitioners(c *gin.Context)
	UpdatePractitioner(c *gin.Context)
	DeletePractitioner(c *gin.Context)
	GetSetting(c *gin.Context)
	UpsertSetting(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) IHandler {
	return &handler{svc: svc}
}

// @Summary      Create a new practitioner
// @Description  Register a new practitioner in the system
// @Tags         setting
// @Accept       json
// @Produce      json
// @Param        request body RqCreatePractitioner true "Practitioner Data"
// @Success      201 {object} RsPractitioner
// @Failure      400 {object} response.RsError
// @Failure      500 {object} response.RsError
// @Security     BearerToken
// @Router       /practitioner [post]
func (h *handler) CreatePractitioner(c *gin.Context) {
	var req RqCreatePractitioner
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	created, err := h.svc.CreatePractitioner(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusCreated, created, "Practitioner created successfully")
}

// @Summary Get a practitioner by ID
// @Description get a practitioner by ID
// @Tags setting
// @Accept json
// @Produce json
// @Success 200 {object} RsPractitioner
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /practitioner/{id} [get]
// @Param id path string true "Practitioner ID"
func (h *handler) GetPractitioner(c *gin.Context) {
	id, ok := util.GetPractitionerID(c)
	fmt.Println("err", id)
	if !ok {
		return
	}
	t, err := h.svc.GetPractitioner(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, t, "Practitioner fetched successfully")
}

// @Summary Get a practitioner by user ID
// @Description get a practitioner by user ID
// @Tags setting
// @Accept json
// @Produce json
// @Success 200 {object} RsPractitioner
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /practitioner/user/{user_id} [get]
// @Param user_id path string true "User ID"
func (h *handler) GetPractitionerByUserID(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		response.Error(c, http.StatusBadRequest, errors.New("user_id required"))
		return
	}
	t, err := h.svc.GetPractitionerByUserID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, t, "Practitioner fetched successfully")
}

// @Summary List practitioners
// @Description list practitioners
// @Tags setting
// @Accept json
// @Produce json
// @Success 200 {object} RsPractitioner
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /practitioner [get]
func (h *handler) ListPractitioners(c *gin.Context) {
	list, err := h.svc.ListPractitioners(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list, "Practitioners fetched successfully")
}

// @Summary Update a practitioner
// @Description update a practitioner
// @Tags setting
// @Accept json
// @Produce json
// @Success 200 {object} RsPractitioner
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /practitioner/{id} [put]
// @Param id path string true "Practitioner ID"
func (h *handler) UpdatePractitioner(c *gin.Context) {
	id, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	var req RqUpdatePractitioner
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	updated, err := h.svc.UpdatePractitioner(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, updated, "Practitioner updated successfully")
}

// @Summary Delete a practitioner
// @Description delete a practitioner
// @Tags setting
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /practitioner/{id} [delete]
// @Param id path string true "Practitioner ID"
func (h *handler) DeletePractitioner(c *gin.Context) {
	id, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	if err := h.svc.DeletePractitioner(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, map[string]string{"message": "deleted"}, "Practitioner deleted successfully")
}

// @Summary Get a setting by practitioner ID
// @Description get a setting by practitioner ID
// @Tags setting
// @Accept json
// @Produce json
// @Success 200 {object} RsPractitionerSetting
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /practitioner/setting/{id} [get]
// @Param id path string true "Practitioner ID"
func (h *handler) GetSetting(c *gin.Context) {
	id, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	setting, err := h.svc.GetSetting(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, setting, "Setting fetched successfully")
}

// @Summary Upsert a setting by practitioner ID
// @Description upsert a setting by practitioner ID
// @Tags setting
// @Accept json
// @Produce json
// @Success 200 {object} RsPractitionerSetting
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /practitioner/setting/{id} [put]
// @Param id path string true "Practitioner ID"
func (h *handler) UpsertSetting(c *gin.Context) {
	id, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	var req RqUpsertPractitionerSetting
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	setting, err := h.svc.UpsertSetting(c.Request.Context(), id, &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, setting, "Setting upserted successfully")
}
