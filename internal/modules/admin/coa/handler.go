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
	Delete(c *gin.Context)
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

// Create provisions a brand new account template chart configuration
// @Summary      Create Account Template
// @Description  Creates a new chart of account blueprint record within the DB storage pool.
// @Tags         Chart of Accounts
// @Accept       json
// @Produce      json
// @Param        request  body      RqAccountTemplate  true  "Account baseline specifications structure payload"
// @Success      201      {string}  string             "Created"
// @Failure      400      {object}  map[string]string  "Invalid input request body error context parameters"
// @Failure      500      {object}  map[string]string  "Internal system storage engine baseline failure"
// @Security     BearerToken
// @Router       /coa/templates [post]
func (h *Handler) Create(c *gin.Context) {
	var rq RqAccountTemplate

	entitiyId, role, ok := util.GetRoleBasedID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"message": "entity id not found"})
		return
	}

	if err := c.ShouldBindJSON(&rq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	switch role {
	case util.RoleAdmin:
		rq.CreatedBy = *entitiyId
	}

	if err := h.service.Create(c.Request.Context(), rq); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.Status(http.StatusCreated)
}

// Update mutates configuration options mapped inside an existing template ID target
// @Summary      Update Account Template
// @Description  Overwrites and updates a target chart structure mapped inside the active repository.
// @Tags         Chart of Accounts
// @Accept       json
// @Produce      json
// @Param        request  body      RqUpdateAccountTemplate  true  "Mutation values specifications struct bundle wrapper"
// @Success      200      {string}  string                   "OK"
// @Failure      400      {object}  map[string]string        "Structural transformation or payload validation parameter failure"
// @Failure      500      {object}  map[string]string        "Underlying relational mapping updating failure"
// @Security     BearerToken
// @Router       /coa/templates [put]
func (h *Handler) Update(c *gin.Context) {
	var rq RqUpdateAccountTemplate
	entitiyId, role, ok := util.GetRoleBasedID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"message": "entity id not found"})
		return
	}

	if err := c.ShouldBindJSON(&rq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	switch role {
	case util.RoleAdmin:
		rq.UpdatedBy = entitiyId
	}

	if err := h.service.Update(c.Request.Context(), rq); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

// Delete soft-deletes a master template and cleanses downstream practitioner alignments
// @Summary      Delete Account Template
// @Description  Removes a global chart blueprint record and removes/decouples matching active records across downstream practitioners.
// @Tags         Chart of Accounts
// @Produce      json
// @Param        id   path      string  true  "Target account template string parsed UUID"
// @Success      200  {object}  map[string]string "The template configuration was successfully removed and cascade transformations executed"
// @Failure      400  {object}  map[string]string "Path parameters missing matching conversion standards or authorization missing"
// @Failure      500  {object}  map[string]string "Cascading database deletions or records severance operation processing failure"
// @Security     BearerToken
// @Router       /coa/templates/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}

	entityId, _, ok := util.GetRoleBasedID(c)
	if !ok || entityId == nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "admin identity context missing"})
		return
	}

	if err := h.service.Delete(c.Request.Context(), id, *entityId); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "account template deleted successfully"})
}

// GetById retrieves full chart blueprint fields filtered by unique ID string path value
// @Summary      Get Account Template By ID
// @Description  Queries and returns a single account template baseline context view record.
// @Tags         Chart of Accounts
// @Produce      json
// @Param        id   path      string  true  "Valid string parsed UUID pattern match filter"
// @Success      200  {object}  RsAccountTemplate "Matching records successfully unpacked and transformed from storage"
// @Failure      400  {object}  map[string]string "Path variables missing precise conversion requirements"
// @Failure      404  {object}  map[string]string "The requested configuration item does not exist inside the target index"
// @Security     BearerToken
// @Router       /coa/templates/{id} [get]
func (h *Handler) GetById(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}

	rs, err := h.service.GetById(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rs)
}

// List resolves an indexed array sequence mapping all accessible template rules
// @Summary      List Account Templates
// @Description  Gathers a generalized collection indexing active charts mapped to system rules.
// @Tags         Chart of Accounts
// @Produce      json
// @Success      200  {array}   RsAccountTemplate "An array matching structural index configuration items"
// @Failure      500  {object}  map[string]string "Internal scanning array processing sequence broken"
// @Security     BearerToken
// @Router       /coa/templates [get]
func (h *Handler) List(c *gin.Context) {
	rs, err := h.service.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rs)
}
