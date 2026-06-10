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

// resolveUserID extracts and parses the authenticated user's UUID from context.
func (h *handler) resolveUserID(c *gin.Context) (uuid.UUID, bool) {
	ptr := auditctx.GetUserID(c.Request.Context())
	if ptr == nil {
		response.Error(c, http.StatusUnauthorized, errors.New("unauthorized: user not found in context"))
		return uuid.Nil, false
	}
	id, err := uuid.Parse(*ptr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid user id format"))
		return uuid.Nil, false
	}
	return id, true
}

// Register godoc
// @Summary Register a new user
// @Description Register a new user account
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
// @Description Authenticate with email and password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RqLogin true "Login Credentials"
// @Success 200 {object} RsToken
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
		var emailVerErr *EmailVerificationRequiredError
		if errors.As(err, &emailVerErr) {
			c.JSON(http.StatusForbidden, RsErrorEmailVerificationRequired{
				Success: false,
				Code:    "EMAIL_VERIFICATION_REQUIRED",
				Message: "Please verify your email address before continuing.",
			})
			return
		}
		var subErr *SubscriptionRequiredError
		if errors.As(err, &subErr) {
			c.JSON(http.StatusForbidden, RsErrorSubscriptionRequired{
				Success:            false,
				Code:               "SUBSCRIPTION_REQUIRED",
				SubscriptionStatus: subErr.SubscriptionStatus,
				Message:            "An active subscription is required to access the application.",
			})
			return
		}
		var paymentErr *PaymentRequiredError
		if errors.As(err, &paymentErr) {
			c.JSON(http.StatusPaymentRequired, RsErrorPaymentRequired{
				Success:            false,
				Code:               "PAYMENT_REQUIRED",
				PaymentStatus:      paymentErr.PaymentStatus,
				SubscriptionStatus: paymentErr.SubscriptionStatus,
				Message:            "Payment is required to access the application. Please complete your payment.",
			})
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
	userID, ok := h.resolveUserID(c)
	if !ok {
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
// @Description Update the profile details of the authenticated user
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
	userID, ok := h.resolveUserID(c)
	if !ok {
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
// @Description Revoke the current session using the refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerToken
// @Param request body RqLogout true "Logout Data"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Router /auth/user/logout [post]
func (h *handler) Logout(c *gin.Context) {
	userID, ok := util.GetUserID(c)
	if !ok {
		return
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

// GoogleAuthURL godoc
// @Summary Get Google OAuth consent-screen URL
// @Description Returns the URL to redirect the user to for Google OAuth
// @Tags auth
// @Produce json
// @Success 200 {object} RsGoogleAuthURL
// @Failure 500 {object} response.RsError
// @Router /auth/google [get]
func (h *handler) GoogleAuthURL(c *gin.Context) {
	state := util.NewUUID()
	result := h.svc.GoogleAuthURL(state)
	response.JSON(c, http.StatusOK, result, "Google OAuth consent-screen URL fetched successfully")
}

// GoogleCallback godoc
// @Summary Handle Google OAuth callback
// @Description Handles the OAuth callback and redirects to the frontend with tokens
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

	frontendURL := h.cfg.FrontendURL
	if h.cfg.Env == "local" {
		frontendURL = h.cfg.LocalUrl
	}

	redirectURL := fmt.Sprintf("%s/auth/callback?access_token=%s&refresh_token=%s",
		frontendURL, token.AccessToken, token.RefreshToken)

	c.Redirect(http.StatusFound, redirectURL)
}

// ChangePassword godoc
// @Summary Change user password
// @Description Updates the password for the authenticated user
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerToken
// @Param request body RqChangePassword true "Password Change Data"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 403 {object} response.RsError
// @Failure 500 {object} response.RsError
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
// @Description Soft-deletes the currently authenticated user's account
// @Tags auth
// @Produce json
// @Security BearerToken
// @Success 200 {object} response.RsBase
// @Failure 401 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /auth/user [delete]
func (h *handler) DeleteUser(c *gin.Context) {
	userID, ok := h.resolveUserID(c)
	if !ok {
		return
	}

	if err := h.svc.DeleteUser(c.Request.Context(), userID); err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, nil, "User account deleted successfully")
}

// VerifyEmail godoc
// @Summary Verify user email address
// @Description Validates the UUID token sent via email and marks the user as verified
// @Tags auth
// @Produce json
// @Param token query string true "Verification Token (UUID)"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 410 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /auth/verify-email [get]
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
// @Summary Initiate password reset
// @Description Sends a reset link to the user's email if the account exists
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RqForgotPassword true "Email"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /auth/forgot-password [post]
func (h *handler) ForgotPassword(c *gin.Context) {
	var req RqForgotPassword
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	if err := h.svc.ForgotPassword(c.Request.Context(), &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	response.JSON(c, http.StatusOK, nil, "If an account exists, a reset link has been sent.")
}

// ResetPassword godoc
// @Summary Reset password using token
// @Description Updates the user's password using the token received via email
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RqResetPassword true "Token and New Password"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /auth/reset-password [post]
func (h *handler) ResetPassword(c *gin.Context) {
	var req RqResetPassword
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	if err := h.svc.ResetPassword(c.Request.Context(), &req); err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, nil, "Password has been reset successfully.")
}
