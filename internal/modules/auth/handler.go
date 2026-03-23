package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	Register(c *gin.Context)
	Login(c *gin.Context)
	Logout(c *gin.Context)
	GoogleAuthURL(c *gin.Context)
	GoogleCallback(c *gin.Context)
	UpdateProfile(c *gin.Context)
	DeleteUser(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) IHandler {
	return &handler{svc: svc}
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
// @Failure 500 {object} response.RsError
// @Router /auth/user/profile [put]
func (h *handler) UpdateProfile(c *gin.Context) {

	userIDPtr := auditctx.GetUserID(c.Request.Context())

	// 2. Check if the pointer is nil (Unauthorized)
	if userIDPtr == nil {
		response.Error(c, http.StatusUnauthorized, errors.New("unauthorized: user not found in context"))
		return
	}

	// 3. Parse the string value into a uuid.UUID for the service layer
	userID, err := uuid.Parse(*userIDPtr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid user id format"))
		return
	}

	// 4. Bind and validate the update request body
	var req RqUpdateUser
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// 5. Call service layer with the parsed UUID
	user, err := h.svc.UpdateProfile(c.Request.Context(), userID, &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, user, "Profile updated successfully")
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

// GoogleCallback godoc
// @Summary Handle Google OAuth callback
// @Description handle Google OAuth callback and return tokens
// @Tags auth
// @Produce json
// @Param code query string true "OAuth authorization code"
// @Success 200 {object} response.RsBase
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

	response.JSON(c, http.StatusOK, token, "Google OAuth callback handled successfully")
}

// Logout godoc
// @Summary Logout a user
// @Description revoke the current session using the refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Router /auth/logout [post]
func (h *handler) Logout(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	if err := h.svc.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		response.Error(c, http.StatusUnauthorized, err)
		return
	}
	response.JSON(c, http.StatusOK, nil, "Logged out successfully")
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
