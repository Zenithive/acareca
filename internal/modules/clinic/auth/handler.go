package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
)

type IHandler interface {
	Register(c *gin.Context)
	Login(c *gin.Context)
	Logout(c *gin.Context)
	GetProfile(c *gin.Context)
	VerifyEmail(c *gin.Context)
}

type handler struct {
	svc Service
	cfg config.Config
}

func NewHandler(svc Service) IHandler {
	cfg := config.NewConfig()
	return &handler{svc: svc, cfg: *cfg}
}

// Register godoc
// @Summary Register a new invoice clinic
// @Description registers an isolated invoice clinic with its primary address and contacts
// @Tags invoice
// @Accept json
// @Produce json
// @Param request body RqRegisterClinic true "Clinic Registration Data"
// @Success 201 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 409 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /clinic/register [post]
func (h *handler) Register(c *gin.Context) {
	var req RqRegisterClinic
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	clinic, err := h.svc.Register(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, ErrEmailTaken) {
			response.Error(c, http.StatusConflict, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusCreated, clinic, "Clinic registered successfully")
}

// Login godoc
// @Summary Login an invoice clinic
// @Description authenticates an invoice clinic profile and issues access tokens
// @Tags invoice
// @Accept json
// @Produce json
// @Param request body RqLoginClinic true "Clinic Login Credentials"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 411 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /clinic/login [post]
func (h *handler) Login(c *gin.Context) {
	var req RqLoginClinic
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	authData, err := h.svc.Login(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, ErrInvalidPassword) {
			response.Error(c, http.StatusUnauthorized, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, authData, "Clinic logged in successfully")
}

// Logout godoc
// @Summary Logout current invoice clinic session
// @Description Invalidates a provided refresh token and tears down the tracking database session state
// @Tags invoice
// @Accept json
// @Produce json
// @Security BearerToken
// @Param request body RqLogoutClinic true "Clinic Session Logout Payload"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /clinic/logout [post]
func (h *handler) Logout(c *gin.Context) {
	clinicID, ok := util.GetEntityID(c)
	if !ok {
		return
	}

	var req RqLogoutClinic
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	err := h.svc.Logout(c.Request.Context(), clinicID, req.RefreshToken)
	if err != nil {
		if err.Error() == "unauthorized session access" {
			response.Error(c, http.StatusUnauthorized, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, nil, "Clinic logged out successfully")
}

// GetProfile godoc
// @Summary Get current invoice clinic profile
// @Description Returns the profile details of the authenticated clinic
// @Tags invoice
// @Produce json
// @Security BearerToken
// @Success 200 {object} response.RsBase
// @Failure 401 {object} response.RsError
// @Failure 403 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /clinic/profile [get]
func (h *handler) GetProfile(c *gin.Context) {
	clinicID, ok := util.GetEntityID(c)
	if !ok {
		return
	}

	clinic, err := h.svc.GetProfile(c.Request.Context(), clinicID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, clinic, "Clinic profile fetched successfully")
}

// @Summary Verify clinic email address
// @Description Validates the UUID token sent via email. If valid, marks the clinic as verified and the token as used.
// @Tags invoice
// @Produce json
// @Param token query string true "Verification Token (UUID)"
// @Success 200 {object} response.RsBase "Email verified successfully"
// @Failure 400 {object} response.RsError "Invalid token format or token already used"
// @Failure 410 {object} response.RsError "Token has expired"
// @Failure 500 {object} response.RsError "Internal server error"
// @Router /clinic/verify [get]
func (h *handler) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		response.Error(c, http.StatusBadRequest, errors.New("token query parameter is required"))
		return
	}

	if err := h.svc.VerifyEmail(c.Request.Context(), token); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	response.JSON(c, http.StatusOK, nil, "Email verified successfully. You can now log in.")
}
