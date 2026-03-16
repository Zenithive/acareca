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

// Calculation implements [IHandler].
func (h *handler) Calculation(c *gin.Context) {
	ctx := c.Request.Context()

	formID, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}

	var filter NetFilter

	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	superComponent := c.Query("super_component")
	if superComponent != "" {
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

	c.JSON(http.StatusOK, result)
}
