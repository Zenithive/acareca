package template

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

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
	GetInvoiceSetting(c *gin.Context)
	UpdateSetting(c *gin.Context)
	GeneratePDF(c *gin.Context)
	DownloadPDF(c *gin.Context)
	BulkUpdateDefaultsHandler(c *gin.Context)
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
// @Param        request  body      RqGlobalTemplate  true  "Global Template Schema Payload"
// @Success      201      {object}  RsTemplate        "Global template created successfully"
// @Failure      400      {object}  map[string]string "Invalid request JSON payload"
// @Failure      500      {object}  map[string]string "Internal Server Error"
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
// @Param        id       path      string            true  "Template UUID ID"
// @Param        request  body      RqGlobalTemplate  true  "Updated Configuration Payload"
// @Success      200      {object}  RsTemplate        "Template updated successfully!"
// @Failure      400      {object}  map[string]string "Invalid Request parameters"
// @Failure      500      {object}  map[string]string "Internal Server Error"
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
// @Param        id   path      string      true  "Template UUID ID"
// @Success      200  {object}  RsTemplate  "Success payload containing raw blueprint configurations"
// @Failure      400  {object}  map[string]string "Invalid configuration criteria identifier"
// @Failure      404  {object}  map[string]string "Target document or template context completely absent"
// @Failure      500  {object}  map[string]string "Internal Server Error"
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

// List returns all active global templates within the DB pool
// @Summary      List Global Templates
// @Description  Gathers paginated index parameters tracking active engine documents
// @Tags         Templates
// @Produce      json
// @Param        type    query     string  false  "Filter by document types (comma-separated: Calculation Statement, Tax Invoice, Remittance Advice)"
// @Success      200  {object}  util.RsList "Collection index values mapped to the configuration array"
// @Failure      500  {object}  map[string]string "Internal Server Error"
// @Security BearerToken
// @Router       /templates [get]
func (h *Handler) List(c *gin.Context) {
	typeParam := c.Query("type")
	var types []string
	if typeParam != "" {
		for _, t := range strings.Split(typeParam, ",") {
			trimmed := strings.TrimSpace(t)
			if trimmed != "" {
				types = append(types, trimmed)
			}
		}
	}

	rs, err := h.svc.List(c.Request.Context(), types)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	response.JSON(c, http.StatusOK, rs, "")
}

// GetSetting yields fallback visual style configuration profile specifications
// @Summary      Get Default Template Settings
// @Description  Queries UI visual presets (e.g., brand colors, fonts, margins) tracked down to structural design blocks
// @Tags         Templates
// @Produce      json
// @Param        id   path      string     true  "Template UUID ID"
// @Success      200  {object}  RsSetting  "Style settings specifications map"
// @Failure      400  {object}  map[string]string "Invalid profile lookup request values"
// @Failure      500  {object}  map[string]string "Internal Server Error"
// @Security BearerToken
// @Router       /templates/{id}/settings [get]
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

// GetInvoiceSetting yields fallback visual style configuration profile specifications matching an active invoice context
// @Summary      Get Invoice-Specific Template Settings
// @Description  Queries UI visual presets prioritizing custom invoice overrides, falling back to global defaults automatically
// @Tags         Templates
// @Produce      json
// @Param        id          path      string     true  "Template UUID ID"
// @Param        invoiceId   query     string     true  "Invoice UUID ID Context"
// @Success      200  {object}  RsSetting  "Resolved style settings specifications map"
// @Failure      400  {object}  map[string]string "Invalid parameters profile lookup request values"
// @Failure      500  {object}  map[string]string "Internal Server Error"
// @Security BearerToken
// @Router       /templates/invoice-settings [get]
func (h *Handler) GetInvoiceSetting(c *gin.Context) {
	clinicId, ok := util.GetEntityID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid clinic id"})
		return
	}

	invoiceId, err := uuid.Parse(c.Query("invoiceId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or missing invoiceId query parameter"})
		return
	}

	// supports ?templateId=<uuid>&templateId=<uuid>...
	rawIds := c.QueryArray("templateId")
	if len(rawIds) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing templateId query parameter"})
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

	rs, err := h.svc.GetInvoiceSetting(c.Request.Context(), clinicId, invoiceId, templateIds)
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
// @Param        id       path      string           true  "Template UUID ID"
// @Param        request  body      RqUpdateSetting  true  "Updated Layout Target Settings"
// @Success      200      {object}  RsSetting        "Settings modified context mapping properties updated cleanly"
// @Failure      400      {object}  map[string]string "Validation failure errors"
// @Failure      500      {object}  map[string]string "Internal Server Error"
// @Security BearerToken
// @Router       /templates/{id}/settings [put]
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

// GeneratePDF generates a PDF from dynamic invoice input context
// @Summary      Preview PDF Generation
// @Description  Takes raw arbitrary runtime invoice context and passes it to headless rendering layers instantly
// @Tags         Templates
// @Accept       json
// @Produce      application/pdf
// @Param        id       path      string       true  "Template UUID ID"
// @Param        request  body      InvoiceData  true  "Dynamic structural template values variables"
// @Success      200      {file}    string       "application/pdf Binary context stream"
// @Failure      400      {object}  map[string]string "Context assignment mapping values parsing discrepancies"
// @Failure      404      {object}  map[string]string "Target document base unavailable"
// @Failure      500      {object}  map[string]string "Internal Server Error"
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

	var data InvoiceData
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
// @Param        invoice_id  path      string    true  "Target Invoice Entity Context Index UUID"
// @Param        templateId  query     []string  true  "Array of Template UUID IDs to render" collectionFormat(multi)
// @Success      200         {file}    file      "Target invoice document byte stream file object matches"
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
func (h *Handler) BulkUpdateDefaultsHandler(c *gin.Context) {
	err := h.svc.BulkUpdateDefaults(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "HTML/CSS global layouts synchronized cleanly to central template stores",
	})
}
