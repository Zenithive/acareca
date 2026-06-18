package coa

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	Create(c *gin.Context)
	Update(c *gin.Context)
	GetById(c *gin.Context)
	List(c *gin.Context)
}

type Handler struct {
	service IService
}

func NewHandler(service IService) IHandler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) Create(c *gin.Context) {
	var rq RqAccountTemplate

	entitiyId, role, ok := util.GetRoleBasedID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "entity id not found",
		})
	}

	if err := c.ShouldBindJSON(&rq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": err.Error(),
		})
		return
	}

	switch role {
	case util.RoleAdmin:
		rq.CreatedBy = *entitiyId
	}

	if err := h.service.Create(c.Request.Context(), rq); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}

	c.Status(http.StatusCreated)
}

func (h *Handler) Update(c *gin.Context) {
	var rq RqUpdateAccountTemplate
	entitiyId, role, ok := util.GetRoleBasedID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "entity id not found",
		})
	}

	if err := c.ShouldBindJSON(&rq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": err.Error(),
		})
		return
	}

	switch role {
	case util.RoleAdmin:
		rq.UpdatedBy = entitiyId
	}

	if err := h.service.Update(c.Request.Context(), rq); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}

	c.Status(http.StatusOK)
}

func (h *Handler) GetById(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "invalid id",
		})
		return
	}

	rs, err := h.service.GetById(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, rs)
}

func (h *Handler) List(c *gin.Context) {
	rs, err := h.service.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, rs)
}
