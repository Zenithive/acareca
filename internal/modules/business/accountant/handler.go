package accountant

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/response"
)

type IHandler interface {
	ListUsers(c *gin.Context)
	ListClinics(c *gin.Context)
	ListForms(c *gin.Context)
	Analytics(c *gin.Context)
}

type Handler struct {
	svc IService
}

func NewHandler(svc IService) IHandler {
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

// ListClinics godoc
// @Summary      Fetch assigned clinics
// @Description  Retrieves all clinics belonging to practitioners who have a completed invitation with this accountant.
// @Tags         accountant
// @Accept       json
// @Produce      json
// @Security     BearerToken
// @Success      200  {array}   ClinicDetail
// @Failure      401  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Router       /accountant/clinics [get]
func (h *Handler) ListClinics(c *gin.Context) {
	clinics, err := h.svc.ListClinics(c)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, clinics, "Clinics retrieved successfully")
}

// ListForms godoc
// @Summary      Fetch assigned forms
// @Description  Retrieves all forms belonging to clinics that this accountant has access to via completed invitations.
// @Tags         accountant
// @Accept       json
// @Produce      json
// @Security     BearerToken
// @Success      200  {array}   RsAccountantForm
// @Failure      401  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Router       /accountant/forms [get]
func (h *Handler) ListForms(c *gin.Context) {
	forms, err := h.svc.ListForms(c)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, forms, "Forms retrieved successfully")
}

// Analytics godoc
// @Summary      Fetch analytics
// @Description  Retrieves analytics for the accountant.
// @Tags         accountant
// @Accept       json
// @Produce      json
// @Security     BearerToken
// @Success      200  {object}   RsAnalytics
// @Failure      401  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Router       /accountant/analytics [get]
func (h *Handler) Analytics(c *gin.Context) {
	var filter FilterAnalytics
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	analytics, err := h.svc.Analytics(c, &filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, analytics, "Analytics retrieved successfully")
}
