package accountant

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/response"
)

type IHandler interface {
	ListUsers(c *gin.Context)
}

type Handler struct {
	svc IService
}

func NewHandler(svc IService) *Handler {
	return &Handler{svc: svc}
}

// ListUsers godoc
// @Summary      Fetch all users
// @Description  Retrieves a list of all registered users for the accountant view.
// @Tags         accountant
// @Accept       json
// @Produce      json
// @Security     BearerToken
// @Success      200  {array}   RsAccountantUser
// @Failure      401  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Router       /accountant/ [get]
func (h *Handler) ListUsers(c *gin.Context) {
	users, err := h.svc.ListUsers(c)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, users, "Users retrieved successfully")
}
