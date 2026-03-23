package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	Register(c *gin.Context)
	Login(c *gin.Context)
	Logout(c *gin.Context)
	GoogleAuthURL(c *gin.Context)
	GoogleCallback(c *gin.Context)
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
	// Store state in a short-lived, HttpOnly cookie so the callback can verify it
	c.SetCookie("oauth_state", state, 300, "/", "", true, true)
	result := h.svc.GoogleAuthURL(state)
	response.JSON(c, http.StatusOK, result, "Google OAuth consent-screen URL fetched successfully")
}

// GoogleCallback godoc
// @Summary Handle Google OAuth callback
// @Description handle Google OAuth callback and return tokens
// @Tags auth
// @Produce json
// @Param code query string true "OAuth authorization code"
// @Param state query string true "OAuth state (CSRF token)"
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

	// Validate state to prevent CSRF
	stateParam := c.Query("state")
	stateCookie, err := c.Cookie("oauth_state")
	if err != nil || stateParam == "" || stateParam != stateCookie {
		response.Error(c, http.StatusBadRequest, errors.New("invalid oauth state"))
		return
	}
	// Consume the state cookie
	c.SetCookie("oauth_state", "", -1, "/", "", true, true)

	token, err := h.svc.GoogleCallback(c.Request.Context(), code)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, token, "Google OAuth callback handled successfully")
}
