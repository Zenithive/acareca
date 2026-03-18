package calculation

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/modules/builder/entry"
	"github.com/iamarpitzala/acareca/internal/modules/builder/version"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	Calculation(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) IHandler {
	return &handler{
		svc: svc,
	}
}

// Calculation godoc
// @Summary Run calculation for a form
// @Description Calculate results for a specific form by ID
// @Tags calculation
// @Produce json
// @Param id path string true "Form ID"
// @Param super_component query number false "Super component value override"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /calculate/{id} [get]
// Calculation implements [IHandler].
func (h *handler) Calculation(c *gin.Context) {
	ctx := c.Request.Context()

	formID, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}

	var filter NetFilter

	if superComponent := c.Query("super_component"); superComponent != "" {
		val, err := strconv.ParseFloat(superComponent, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, fmt.Errorf("invalid super_component"))
			return
		}
		filter.SuperComponent = &val
	}

	result, err := h.svc.Calculate(ctx, formID, &filter)
	if err != nil {
		if errors.Is(err, entry.ErrNotFound) || errors.Is(err, version.ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Calculation completed successfully")
}
