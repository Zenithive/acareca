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
	ChangePassword(c *gin.Context)
	UpdateProfile(c *gin.Context)
	DeleteClinic(c *gin.Context)
	ForgotPassword(c *gin.Context)
	ResetPassword(c *gin.Context)
}

type handler struct {
	svc Service
	cfg config.Config
}

func NewHandler(svc Service) IHandler {
	cfg, err := config.NewConfig()
	if err != nil {
		panic(err)
	}
	return &handler{svc: svc, cfg: *cfg}
}

// Register godoc
// @Summary Register a new invoice clinic
// @Description registers an isolated invoice clinic with its primary address and contacts
// @Tags invoice-clinic
// @Accept json
// @Produce json
// @Param request body RqRegister true "Clinic Registration Data"
// @Success 201 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 409 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /clinic/register [post]
func (h *handler) Register(c *gin.Context) {
	var req RqRegister
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
// @Tags invoice-clinic
// @Accept json
// @Produce json
// @Param request body RqLogin true "Clinic Login Credentials"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 411 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /clinic/login [post]
func (h *handler) Login(c *gin.Context) {
	var req RqLogin
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
// @Tags invoice-clinic
// @Accept json
// @Produce json
// @Security BearerToken
// @Param request body RqLogout true "Clinic Session Logout Payload"
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

	var req RqLogout
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
// @Tags invoice-clinic
// @Produce json
// @Security BearerToken
// @Success 200 {object} response.RsBase
// @Failure 401 {object} response.RsError
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

// VerifyEmail godoc
// @Summary Verify clinic email address
// @Description Validates the UUID token sent via email. If valid, marks the clinic as verified and the token as used.
// @Tags invoice-clinic
// @Produce json
// @Param token query string true "Verification Token (UUID)"
// @Success 200 {object} response.RsBase "Email verified successfully"
// @Failure 400 {object} response.RsError "Invalid token format or token already used"
// @Failure 500 {object} response.RsError "Internal server error"
// @Router /clinic/verify-email [get]
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

// ChangePassword godoc
// @Summary Change clinic password
// @Description Updates the password for the authenticated clinic
// @Tags invoice-clinic
// @Accept json
// @Produce json
// @Security BearerToken
// @Param request body RqChangePassword true "Password Change Data"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /clinic/change-password [put]
func (h *handler) ChangePassword(c *gin.Context) {
	clinicID, ok := util.GetEntityID(c)
	if !ok {
		return
	}

	var req RqChangePassword
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	if err := h.svc.ChangePassword(c.Request.Context(), clinicID, &req); err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, nil, "Password changed successfully")
}

// UpdateProfile godoc
// @Summary Update clinic profile
// @Description Update the profile details of the authenticated clinic
// @Tags invoice-clinic
// @Accept json
// @Produce json
// @Security BearerToken
// @Param request body RqUpdate true "Update Data"
// @Success 200 {object} RsClinic
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /clinic/profile [put]
func (h *handler) UpdateProfile(c *gin.Context) {
	clinicID, ok := util.GetEntityID(c)
	if !ok {
		return
	}

	var req RqUpdate
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	req.Id = clinicID

	clinic, err := h.svc.UpdateProfile(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, clinic, "Clinic profile updated successfully")
}

// DeleteClinic godoc
// @Summary Delete clinic account
// @Description Soft delete the currently authenticated clinic's account
// @Tags invoice-clinic
// @Produce json
// @Security BearerToken
// @Success 200 {object} response.RsBase
// @Failure 401 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /clinic [delete]
func (h *handler) DeleteClinic(c *gin.Context) {
	clinicID, ok := util.GetEntityID(c)
	if !ok {
		return
	}

	if err := h.svc.DeleteClinic(c.Request.Context(), clinicID); err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, nil, "Clinic account deleted successfully")
}

// ForgotPassword godoc
// @Summary Initiate clinic password reset
// @Description Sends a reset link to the clinic's email if it exists
// @Tags invoice-clinic
// @Accept json
// @Produce json
// @Param request body RqForgotPassword true "Email"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Router /clinic/forgot-password [post]
func (h *handler) ForgotPassword(c *gin.Context) {
	var req RqForgotPassword
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	if err := h.svc.ForgotPassword(c.Request.Context(), &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	response.JSON(c, http.StatusOK, nil, "A password reset link has been sent to your email address.")
}

// ResetPassword godoc
// @Summary Reset clinic password using token
// @Description Updates the clinic's password using the token received via email
// @Tags invoice-clinic
// @Accept json
// @Produce json
// @Param request body RqResetPassword true "Token and New Password"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /clinic/reset-password [post]
func (h *handler) ResetPassword(c *gin.Context) {
	var req RqResetPassword
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	if err := h.svc.ResetPassword(c.Request.Context(), &req); err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, nil, "Password has been reset successfully.")
}
