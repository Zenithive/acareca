package auth

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
)

type IHandler interface {
	Register(c *gin.Context)
	Login(c *gin.Context)
	Logout(c *gin.Context)
	GoogleAuthURL(c *gin.Context)
	GoogleCallback(c *gin.Context)

	VerifyEmail(c *gin.Context)
	ChangePassword(c *gin.Context)
	GetProfile(c *gin.Context)
	UpdateProfile(c *gin.Context)
	DeleteUser(c *gin.Context)
	ForgotPassword(c *gin.Context)
	ResetPassword(c *gin.Context)
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
// @Summary Register a new user
// @Description register a new user
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RqUser true "Registration Data"
// @Success 201 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 409 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /auth/register [post]
func (h *handler) Register(c *gin.Context) {
	var req RqUser
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	user, err := h.svc.Register(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, ErrEmailTaken) {
			response.Error(c, http.StatusConflict, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusCreated, user, "User registered successfully")
}

// Login godoc
// @Summary Login a user
// @Description login a user
// @Tags auth
// @Accept json
// @Produce json
// @Param        request  body      RqLogin  true  "Login Credentials"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /auth/login [post]
func (h *handler) Login(c *gin.Context) {
	var req RqLogin
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	token, err := h.svc.Login(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, ErrInvalidPassword) || errors.Is(err, ErrOAuthOnly) {
			response.Error(c, http.StatusUnauthorized, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, token, "User logged in successfully")
}

// GetProfile godoc
// @Summary Get current user profile
// @Description Returns the profile of the authenticated user including role-specific fields (abn for practitioners, license_no for accountants)
// @Tags auth
// @Produce json
// @Security BearerToken
// @Success 200 {object} RsUser
// @Failure 401 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /auth/user/profile [get]
func (h *handler) GetProfile(c *gin.Context) {
	userIDPtr := auditctx.GetUserID(c.Request.Context())
	if userIDPtr == nil {
		response.Error(c, http.StatusUnauthorized, errors.New("unauthorized: user not found in context"))
		return
	}

	userID, err := uuid.Parse(*userIDPtr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid user id format"))
		return
	}

	user, err := h.svc.GetProfile(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, user, "Profile fetched successfully")
}

// UpdateProfile godoc
// @Summary Update user profile
// @Description Update the profile details of the user (email, names, phone)
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerToken
// @Param request body RqUpdateUser true "Update Data"
// @Success 200 {object} RsUser
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 409 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /auth/user/profile [put]
func (h *handler) UpdateProfile(c *gin.Context) {

	userIDPtr := auditctx.GetUserID(c.Request.Context())

	if userIDPtr == nil {
		response.Error(c, http.StatusUnauthorized, errors.New("unauthorized: user not found in context"))
		return
	}

	userID, err := uuid.Parse(*userIDPtr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid user id format"))
		return
	}

	var req RqUpdateUser
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	user, err := h.svc.UpdateProfile(c.Request.Context(), userID, &req)
	if err != nil {
		if errors.Is(err, ErrEmailTaken) {
			response.Error(c, http.StatusConflict, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, user, "Profile updated successfully")
}

// Logout godoc
// @Summary Logout a user
// @Description revoke the current session using the refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RqLogout true "Logout Data"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Security BearerToken
// @Router /auth/user/logout [post]
func (h *handler) Logout(c *gin.Context) {
	// Get UserID from context (set by middleware)
	userID, ok := util.GetUserID(c)
	if !ok {
		return // GetUserID handles the error response
	}

	var req RqLogout
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	if err := h.svc.Logout(c.Request.Context(), userID, req.RefreshToken); err != nil {
		response.Error(c, http.StatusUnauthorized, err)
		return
	}
	response.JSON(c, http.StatusOK, nil, "Logged out successfully")
}

// GoogleLogin godoc
// @Summary Get Google OAuth consent-screen URL
// @Description get Google OAuth consent-screen URL
// @Tags auth
// @Produce json
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /auth/google [get]
func (h *handler) GoogleLogin(c *gin.Context) {
	state := util.NewUUID()
	result := h.svc.GoogleAuthURL(state)
	response.JSON(c, http.StatusOK, result, "Google OAuth consent-screen URL fetched successfully")
}

// GoogleAuthURL godoc
// @Summary Get Google OAuth consent-screen URL
// @Description get Google OAuth consent-screen URL
// @Tags auth
// @Produce json
// @Success 200 {object} RsGoogleAuthURL
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /auth/google [get]
func (h *handler) GoogleAuthURL(c *gin.Context) {
	state := util.NewUUID()
	result := h.svc.GoogleAuthURL(state)
	response.JSON(c, http.StatusOK, result, "Google OAuth consent-screen URL fetched successfully")
}

// GoogleCallback godoc
// @Summary Handle Google OAuth callback
// @Description handle Google OAuth callback and redirect to frontend with tokens
// @Tags auth
// @Produce json
// @Param code query string true "OAuth authorization code"
// @Success 302 {string} string "Redirect to frontend"
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /auth/google/callback [get]
func (h *handler) GoogleCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		response.Error(c, http.StatusBadRequest, errors.New("missing oauth code"))
		return
	}

	token, err := h.svc.GoogleCallback(c.Request.Context(), code)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	var frontendURL string
	if h.cfg.Env == "local" {
		frontendURL = h.cfg.LocalUrl
	} else {
		frontendURL = h.cfg.FrontendURL
	}

	redirectURL := fmt.Sprintf("%s/auth/callback?access_token=%s&refresh_token=%s",
		frontendURL,
		token.AccessToken,
		token.RefreshToken)

	c.Redirect(http.StatusFound, redirectURL)
}

// ChangePassword godoc
// @Summary Change user password
// @Description updates the password for the authenticated user
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RqChangePassword true "Password Change Data"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 403 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /auth/user/change-password [put]
func (h *handler) ChangePassword(c *gin.Context) {
	pracID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}

	var req RqChangePassword
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	if err := h.svc.ChangePassword(c.Request.Context(), pracID, &req); err != nil {
		if errors.Is(err, ErrOAuthOnly) {
			response.Error(c, http.StatusForbidden, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, nil, "Password changed successfully")
}

// DeleteUser godoc
// @Summary Delete user account
// @Description Soft delete the currently authenticated user's account
// @Tags auth
// @Produce json
// @Security BearerToken
// @Success 200 {object} response.RsBase
// @Failure 401 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /auth/user [delete]
func (h *handler) DeleteUser(c *gin.Context) {

	userIDPtr := auditctx.GetUserID(c.Request.Context())

	// 2. Check if the pointer is nil (Unauthorized)
	if userIDPtr == nil {
		response.Error(c, http.StatusUnauthorized, errors.New("unauthorized: user not found in context"))
		return
	}

	userID, err := uuid.Parse(*userIDPtr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid user id format"))
		return
	}

	// 4. Call service layer to perform the soft delete
	if err := h.svc.DeleteUser(c.Request.Context(), userID); err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, nil, "User account deleted successfully")
}

// @Summary Verify user email address
// @Description Validates the UUID token sent via email. If valid, marks the user as verified and the token as used.
// @Tags auth
// @Produce json
// @Param token query string true "Verification Token (UUID)"
// @Success 200 {object} response.RsBase "Email verified successfully"
// @Failure 400 {object} response.RsError "Invalid token format or token already used"
// @Failure 410 {object} response.RsError "Token has expired"
// @Failure 500 {object} response.RsError "Internal server error"
// @Router /auth/verify [get]
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

// ForgotPassword godoc
// @Summary      Initiate password reset
// @Description  Sends a reset link to the user's email if they exist.
// @Tags         auth
// @Param        request  body      RqForgotPassword  true  "Email"
// @Success      200      {object}  response.RsBase
// @Router       /auth/forgot-password [post]
func (h *handler) ForgotPassword(c *gin.Context) {
	var req RqForgotPassword
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// Call service (Service will handle token gen and Resend email)
	err := h.svc.ForgotPassword(c.Request.Context(), &req)
	if err != nil {
		// We log the error but return success to avoid "User Enumeration"
		// log.Printf("Forgot password error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response.JSON(c, http.StatusOK, nil, "If an account exists, a reset link has been sent.")
}

// ResetPassword godoc
// @Summary      Reset password using token
// @Description  Updates the user's password using the token received via email.
// @Tags         auth
// @Param        request  body      RqResetPassword  true  "Token and New Password"
// @Success      200      {object}  response.RsBase
// @Router       /auth/reset-password [post]
func (h *handler) ResetPassword(c *gin.Context) {
	var req RqResetPassword
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	err := h.svc.ResetPassword(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, nil, "Password has been reset successfully.")
}
