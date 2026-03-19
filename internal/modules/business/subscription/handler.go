package subscription

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type IHandler interface {
	Create(c *gin.Context)
	GetByID(c *gin.Context)
	ListByPractitionerID(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
}

type handler struct {
	svc Service
	db  *sqlx.DB
}

func NewHandler(svc Service, db *sqlx.DB) IHandler {
	return &handler{svc: svc, db: db}
}

// @Summary Create a subscription
// @Description create a subscription for a practitioner
// @Tags subscription
// @Accept json
// @Produce json
// @Success 201 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /practitioner/subscription [post]
func (h *handler) Create(c *gin.Context) {
	practitionerID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	var req RqCreatePractitionerSubscription
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	var created *RsPractitionerSubscription
	err := util.RunInTransaction(c, h.db, func(ctx context.Context, tx *sqlx.Tx) error {
		Created, err := h.svc.Create(c.Request.Context(), practitionerID, &req, tx)
		if err != nil {
			return err
		}
		created = Created
		return nil
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusCreated, created, "Subscription created successfully")
}

// @Summary Get a subscription by ID
// @Description get a subscription by ID
// @Tags subscription
// @Accept json
// @Produce json
// @Param sub_id path int true "Subscription ID"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /practitioner/subscription/{sub_id} [get]
func (h *handler) GetByID(c *gin.Context) {
	id, ok := util.ParseIntID(c, "sub_id")
	if !ok {
		return
	}
	sub, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, sub, "Subscription fetched successfully")
}

// @Summary List subscriptions by practitioner ID
// @Description list subscriptions by practitioner ID
// @Tags subscription
// @Accept json
// @Produce json
// @Param status query string false "Filter by Status (ACTIVE, CANCELLED, etc.)"
// @Param subscription_id query int false "Filter by specific Subscription ID"
// @Param search query string false "Search within subscriptions"
// @Param limit query int false "Pagination limit"
// @Param offset query int false "Pagination offset"
// @Success 200 {object} util.RsList
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /practitioner/subscription [get]
func (h *handler) ListByPractitionerID(c *gin.Context) {
	practitionerID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	var f Filter
	if err := util.BindAndValidate(c, &f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	list, err := h.svc.ListByPractitionerID(c.Request.Context(), practitionerID, &f)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list, "Subscriptions fetched successfully")
}

// @Summary Update a subscription
// @Description update a subscription
// @Tags subscription
// @Accept json
// @Produce json
// @Param sub_id path int true "Subscription ID"
// @Param request body RqUpdatePractitionerSubscription true "Updated Subscription Data"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /practitioner/subscription/{sub_id} [patch]
func (h *handler) Update(c *gin.Context) {
	id, ok := util.ParseIntID(c, "sub_id")
	if !ok {
		return
	}
	var req RqUpdatePractitionerSubscription
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	updated, err := h.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, updated, "Subscription updated successfully")
}

// @Summary Delete a subscription
// @Description delete a subscription
// @Tags subscription
// @Accept json
// @Produce json
// @Param sub_id path int true "Subscription ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /practitioner/subscription/{sub_id} [delete]
func (h *handler) Delete(c *gin.Context) {
	id, ok := util.ParseIntID(c, "sub_id")
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
	response.JSON(c, http.StatusOK, map[string]string{"message": "deleted"}, "Subscription deleted successfully")
}
