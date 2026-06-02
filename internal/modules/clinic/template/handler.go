package template

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	Create(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	Get(c *gin.Context)
	List(c *gin.Context)
	GetSetting(c *gin.Context)
	UpdateSetting(c *gin.Context)
}

type Handler struct {
	svc IService
}

func NewHandler(svc IService) IHandler {
	return &Handler{svc: svc}
}

// Create implements [IHandler].
// @Summary Create a new template for a clinic
// @Tags template
// @Accept json
// @Produce json
// @Param request body RqTemplate true "Template Data"
// @Success 201 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /template [post]
func (h *Handler) Create(c *gin.Context) {
	var rq RqTemplate
	if err := c.ShouldBindJSON(&rq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rs, err := h.svc.Create(c.Request.Context(), rq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	response.JSON(c, http.StatusCreated, rs, "template created successfully")
}

// Update implements [IHandler].
// @Summary Update an template by ID
// @Tags template
// @Accept json
// @Produce json
// @Param id path string true "Template ID"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /template/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var rq RqTemplate
	if err := c.ShouldBindJSON(&rq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rq.Id = id
	rs, err := h.svc.Update(c.Request.Context(), rq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	response.JSON(c, http.StatusOK, rs, "template updated successfully!")
}

// Delete implements [IHandler].
// @Summary Delete an template by ID
// @Tags template
// @Accept json
// @Produce json
// @Param id path string true "Template ID"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /template/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var clinicId uuid.UUID
	var ok bool
	if clinicId, ok = util.GetEntityID(c); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid clinic id"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), clinicId, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	response.JSON(c, http.StatusNoContent, nil, "template delete successfully")
}

// Get implements [IHandler].
// @Summary Get an template by ID
// @Tags template
// @Accept json
// @Produce json
// @Param id path string true "Template ID"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /template/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var clinicId uuid.UUID
	var ok bool
	if clinicId, ok = util.GetEntityID(c); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid clinic id"})
		return
	}

	rs, err := h.svc.Get(c.Request.Context(), clinicId, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	response.JSON(c, http.StatusOK, rs, "")
}

// Get implements [IHandler].
// @Summary Get an template by ID
// @Tags template
// @Accept json
// @Produce json
// @Success 200 {object} util.RsList
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /template [get]
func (h *Handler) List(c *gin.Context) {
	clinicId, ok := util.GetEntityID(c)
	if !ok {
		response.Error(c, http.StatusBadRequest, errors.New("clinic not found"))
		return
	}

	rs, err := h.svc.List(c.Request.Context(), clinicId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	response.JSON(c, http.StatusOK, rs, "")
}

// Get implements [IHandler].
// @Summary Get an template settings by ID
// @Tags template
// @Accept json
// @Produce json
// @Param id path string true "Template ID"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /template/{id}/setting [get]
func (h *Handler) GetSetting(c *gin.Context) {
	templateId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid template id"})
		return
	}
	rs, err := h.svc.GetSetting(c.Request.Context(), templateId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	response.JSON(c, http.StatusOK, rs, "")
}

// Update implements [IHandler].
// @Summary Update an template setting by ID
// @Tags template
// @Accept json
// @Produce json
// @Param id path string true "Template ID"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /template/{id}/setting [put]
func (h *Handler) UpdateSetting(c *gin.Context) {
	templateId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid template id"})
		return
	}
	var rq RqUpdateSetting
	if err := c.ShouldBindJSON(&rq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rq.TemplateId = templateId
	rs, err := h.svc.UpdateSetting(c.Request.Context(), rq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	response.JSON(c, http.StatusOK, rs, "")
}
