package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	Create(c *gin.Context)
	GetById(c *gin.Context)
}

type handler struct {
	svc IService
}

func NewHandler(svc IService) IHandler {
	return &handler{svc: svc}
}

// @Summary Create admin profile and user
// @Description Creates a new entry in tbl_user with Role=ADMIN and a corresponding entry in tbl_admin within a single transaction.
// @Tags admin
// @Accept json
// @Produce json
// @Param request body RqCreateAdmin true "Admin and User details"
// @Success 201 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /admin [post]
func (h *handler) Create(c *gin.Context) {
	var req RqCreateAdmin
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	res, err := h.svc.CreateAdmin(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusCreated, res, "Admin and User created successfully")
}

// @Summary Get Admin Detail
// @Description Get nested Admin and User information by Admin ID
// @Tags admin
// @Param id path string true "Admin ID"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Security BearerToken
// @Router /admin/{id} [get]
func (h *handler) GetById(c *gin.Context) {
	adminID, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}

	res, err := h.svc.GetAdminByID(c.Request.Context(), adminID)
	if err != nil {
		response.Error(c, http.StatusNotFound, err)
		return
	}

	response.JSON(c, http.StatusOK, res, "Admin fetched successfully")
}
