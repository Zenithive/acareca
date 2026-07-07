package template

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	Create(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	Get(c *gin.Context)
	List(c *gin.Context)
	GetInvoiceSetting(c *gin.Context)
	UpdateSetting(c *gin.Context)
	GeneratePDF(c *gin.Context)
	DownloadPDF(c *gin.Context)
	BulkSyncDefaultsHandler(c *gin.Context)
}

type Handler struct {
	svc IService
}

func NewHandler(svc IService) IHandler {
	return &Handler{svc: svc}
}

// Create handles global system template generation
// @Summary      Create Global Template
// @Description  Generates a new global HTML/CSS template standard layout block
// @Tags         Templates
// @Accept       json
// @Produce      json
// @Param        request  body      RqGlobalTemplate       true  "Global Template Schema Payload"
// @Success      201      {object}  template.RsTemplate    "Global template created successfully"
// @Failure      400      {object}  map[string]string      "Invalid request JSON payload"
// @Failure      500      {object}  map[string]string      "Internal Server Error"
// @Security BearerToken
// @Router       /templates [post]
func (h *Handler) Create(c *gin.Context) {
	var rq RqGlobalTemplate
	if err := c.ShouldBindJSON(&rq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rs, err := h.svc.Create(c.Request.Context(), rq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	response.JSON(c, http.StatusCreated, rs, "Global template created successfully")
}

// Update changes standard properties on a global layout definition
// @Summary      Update Global Template
// @Description  Modifies the foundational attributes, HTML blueprints, or styling tags of an existing configuration profile
// @Tags         Templates
// @Accept       json
// @Produce      json
// @Param        id       path      string                 true  "Template UUID ID"
// @Param        request  body      RqGlobalTemplate       true  "Updated Configuration Payload"
// @Success      200      {object}  template.RsTemplate    "Template updated successfully!"
// @Failure      400      {object}  map[string]string      "Invalid Request parameters"
// @Failure      500      {object}  map[string]string      "Internal Server Error"
// @Security BearerToken
// @Router       /templates/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var rq RqGlobalTemplate
	if err := c.ShouldBindJSON(&rq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rs, err := h.svc.Update(c.Request.Context(), id, rq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	response.JSON(c, http.StatusOK, rs, "Template updated successfully!")
}

// Delete marks a global system template row context as soft-deleted
// @Summary      Delete Global Template
// @Description  Removes or flags a template definition from active usage routing context pools
// @Tags         Templates
// @Param        id   path      string  true  "Template UUID ID"
// @Success      24   "Template deleted successfully"
// @Failure      400  {object}  map[string]string "Invalid dynamic parameters"
// @Failure      500  {object}  map[string]string "Internal Server Error"
// @Security BearerToken
// @Router       /templates/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	response.JSON(c, http.StatusNoContent, nil, "Template deleted successfully")
}

// Get finds a single global template layout schema pattern block
// @Summary      Get Global Template
// @Description  Returns the decoupled layout configuration values for a designated template ID
// @Tags         Templates
// @Produce      json
// @Param        id   path      string                 true  "Template UUID ID"
// @Success      200  {object}  template.RsTemplate    "Success payload containing raw blueprint configurations"
// @Failure      400  {object}  map[string]string      "Invalid configuration criteria identifier"
// @Failure      404  {object}  map[string]string      "Target document or template context completely absent"
// @Failure      500  {object}  map[string]string      "Internal Server Error"
// @Security BearerToken
// @Router       /templates/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	rs, err := h.svc.Get(c.Request.Context(), id)
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

// List returns all active global templates within the DB pool filtered by an invoicing method
// @Summary      List Global Templates
// @Description  Gathers pagination index tracking active engine documents matching invoice methods
// @Tags         Templates
// @Produce      json
// @Param        method  query     string  false  "Filter by invoice method : SFA_CLINIC_COLLECTS(A), SFA_DENTIST_COLLECTS(B) or INDEPENDENT_CONTRACTOR(C)"
// @Success      200  {object}  util.RsList "Collection index values mapped to the configuration array"
// @Failure      500  {object}  map[string]string "Internal Server Error"
// @Security BearerToken
// @Router       /templates [get]
func (h *Handler) List(c *gin.Context) {
	methodParam := strings.TrimSpace(strings.ToUpper(c.Query("method")))

	rs, err := h.svc.List(c.Request.Context(), methodParam)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	response.JSON(c, http.StatusOK, rs, "")
}

// GetInvoiceSetting yields fallback visual style configuration profile specifications matching an active invoice context
// @Summary      Get Invoice-Specific Template Settings
// @Description  Queries UI visual presets prioritizing custom invoice overrides, falling back to global defaults automatically
// @Tags         Templates
// @Produce      json
// @Param        invoiceId   query     string                false  "Invoice ID (blank for global system defaults)"
// @Success      200  {object}  common.RsSetting        "Resolved style settings specifications model payload"
// @Failure      400  {object}  map[string]string       "Invalid parameters profile lookup request values"
// @Failure      500  {object}  map[string]string       "Internal Server Error"
// @Security BearerToken
// @Router       /templates/invoice-settings [get]
func (h *Handler) GetInvoiceSetting(c *gin.Context) {
	_, ok := util.GetEntityID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid clinic id"})
		return
	}

	// Default to a zero value UUID to support the global fallback mechanism safely
	invoiceId := uuid.Nil
	invoiceIdStr := c.Query("invoiceId")

	// Only parse if the parameter is explicitly passed in the query string
	if invoiceIdStr != "" {
		parsedId, err := uuid.Parse(invoiceIdStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid format for invoiceId query parameter; must be a valid UUID string"})
			return
		}
		invoiceId = parsedId
	}

	rs, err := h.svc.GetInvoiceSetting(c.Request.Context(), invoiceId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response.JSON(c, http.StatusOK, rs, "")
}

// UpdateSetting dynamically overrides default style rules
// @Summary      Update Template Settings
// @Description  Overrides layout details, font schemas, branding assets, or invoice structural rules dynamically
// @Tags         Templates
// @Accept       json
// @Produce      json
// @Param        id       path      string                  true  "Invoice ID"
// @Param        request  body      RqUpdateSetting         true  "Updated Layout Target Settings"
// @Success      200      {object}  common.RsSetting        "Settings modified context mapping properties updated cleanly"
// @Failure      400      {object}  map[string]string       "Validation failure errors"
// @Failure      500      {object}  map[string]string       "Internal Server Error"
// @Security BearerToken
// @Router       /templates/{id}/settings [put]
func (h *Handler) UpdateSetting(c *gin.Context) {
	invoiceId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid template id"})
		return
	}
	var rq RqUpdateSetting
	if err := c.ShouldBindJSON(&rq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rq.InvoiceId = &invoiceId
	rs, err := h.svc.UpdateSetting(c.Request.Context(), rq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	response.JSON(c, http.StatusOK, rs, "")
}

// GeneratePDF generates a PDF from dynamic invoice input context
// @Summary      Preview PDF Generation
// @Description  Takes raw arbitrary runtime invoice context and passes it to headless rendering layers instantly
// @Tags         Templates
// @Accept       json
// @Produce      application/pdf
// @Param        id       path      string              true  "Template UUID ID"
// @Param        request  body      common.Invoice      true  "Dynamic structural template values variables"
// @Success      200      {file}    string              "application/pdf Binary context stream"
// @Failure      400      {object}  map[string]string   "Context assignment mapping values parsing discrepancies"
// @Failure      404      {object}  map[string]string   "Target document base unavailable"
// @Failure      500      {object}  map[string]string   "Internal Server Error"
// @Security BearerToken
// @Router       /templates/{id}/preview-pdf [post]
func (h *Handler) GeneratePDF(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	clinicId, ok := util.GetEntityID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid clinic id"})
		return
	}

	var data common.Invoice
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pdf, err := h.svc.GeneratePDF(c.Request.Context(), RqGeneratePDF{
		TemplateId: id,
		ClinicId:   clinicId,
		Data:       data,
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", "attachment; filename=invoice.pdf")
	c.Data(http.StatusOK, "application/pdf", pdf)
}

// DownloadPDF resolves an actual database entity context to download a generated PDF statement
// @Summary      Download Compiled Invoice PDF
// @Description  Queries static database invoice documents, evaluates values natively against dynamic parameters, and streams a file binary response
// @Tags         Templates
// @Produce      application/pdf
// @Param        invoice_id  path      string  true  "Target Invoice Entity Context Index UUID"
// @Param        templateId  query     string  true  "Template UUID ID (can be repeated for multiple templates)"
// @Success      200         {file}    string  "Target invoice document byte stream file object matches"
// @Failure      400         {object}  map[string]string "Target routing value errors or profile validation flaws"
// @Failure      404         {object}  map[string]string "Target entities unavailable"
// @Failure      500         {object}  map[string]string "Internal Server Error"
// @Security     BearerToken
// @Router       /templates/invoices/{invoice_id}/download [get]
func (h *Handler) DownloadPDF(c *gin.Context) {
	invoiceId, err := uuid.Parse(c.Param("invoice_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid invoice id"})
		return
	}

	clinicId, ok := util.GetEntityID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid clinic id"})
		return
	}

	rawIds := c.QueryArray("templateId")
	if len(rawIds) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing templateId query parameter"})
		return
	}

	// Limit template IDs to prevent abuse
	const maxTemplateIds = 10
	if len(rawIds) > maxTemplateIds {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("too many template IDs provided, maximum is %d", maxTemplateIds)})
		return
	}

	templateIds := make([]uuid.UUID, 0, len(rawIds))
	for _, raw := range rawIds {
		id, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid templateId: " + raw})
			return
		}
		templateIds = append(templateIds, id)
	}

	pdf, filename, err := h.svc.DownloadPDF(c.Request.Context(), clinicId, templateIds, invoiceId)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		if errors.Is(err, ErrInvoiceNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, ErrUnauthorized) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.pdf", filename))
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Length", fmt.Sprintf("%d", len(pdf)))
	c.Data(http.StatusOK, "application/pdf", pdf)
}

// BulkUpdateDefaultsHandler triggers application initialization routines globally
// @Summary      Synchronize Global Default Layouts
// @Description  Forces state evaluations to verify, sync, or seed layout definitions directly to internal storage
// @Tags         Templates
// @Produce      json
// @Success      200  {object}  map[string]string "Initialization completion message mappings"
// @Failure      500  {object}  map[string]string "Internal Server Error"
// @Security BearerToken
// @Router       /templates/sync-defaults [post]
func (h *Handler) BulkSyncDefaultsHandler(c *gin.Context) {
	err := h.svc.BulkSyncDefaults(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "All invoice templates have been synced succesfully",
	})
}
