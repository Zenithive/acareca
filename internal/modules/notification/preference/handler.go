package preference

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	Get(c *gin.Context)
	Update(c *gin.Context)
}

type handler struct {
	svc IService
}

func NewHandler(svc IService) IHandler {
	return &handler{svc: svc}
}

// @Summary      Get notification preferences
// @Description  Returns the current channel preferences for the authenticated user.
// @Tags         notification
// @Success      200  {object}  response.RsBase
// @Failure      401  {object}  response.RsError
// @Failure      404  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Security     BearerToken
// @Router       /notification/preferences [get]
func (h *handler) Get(c *gin.Context) {
	userID, ok := util.GetUserID(c)
	if !ok {
		return
	}

	prefs, err := h.svc.Get(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, prefs, "Notification preferences fetched successfully")
}

// @Summary      Update notification preference
// @Description  Updates or creates a preference for a specific event type.
// @Tags         notification
// @Param        body  body  RqUpdatePreference  true  "Preference Update"
// @Success      200  {object}  response.RsBase
// @Failure      400  {object}  response.RsError
// @Failure      401  {object}  response.RsError
// @Failure      404  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Security     BearerToken
// @Router       /notification/preferences [put]
func (h *handler) Update(c *gin.Context) {
	userID, okUser := util.GetUserID(c)
	actorID, okEntity := util.GetEntityID(c)
	if !okUser || !okEntity {
		return
	}

	role := c.GetString("role")

	var rq RqUpdatePreference
	if err := c.ShouldBindJSON(&rq); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	err := h.svc.Update(c.Request.Context(), userID, actorID, role, rq)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, nil, "Notification preferences updated successfully")
}
