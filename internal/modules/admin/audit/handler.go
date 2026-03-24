package audit

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type Handler interface {
	ListAuditLogs(c *gin.Context)
	GetAuditLog(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) Handler {
	return &handler{svc: svc}
}

// @Summary List audit logs
// @Description Query audit logs with filters
// @Tags audit
// @Accept json
// @Produce json
// @Param practice_id query string false "Filter by Practice ID"
// @Param user_id query string false "Filter by User ID"
// @Param module query string false "Filter by Module name"
// @Param action query string false "Filter by Action name"
// @Param entity_type query string false "Filter by Entity type"
// @Param entity_id query string false "Filter by Entity ID"
// @Param start_date query string false "Start date (e.g. 2026-03-18T00:00:00Z)"
// @Param end_date query string false "End date (e.g. 2026-03-18T23:59:59Z)"
// @Param search query string false "Search across module and action"
// @Param sort_by query string false "Sort field (created_at, action)"
// @Param order_by query string false "Sort direction (ASC, DESC)"
// @Param limit query int false "Page size" default(100)
// @Param offset query int false "Page offset" default(0)
// @Success 200 {object} util.RsList
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/audit [get]
func (h *handler) ListAuditLogs(c *gin.Context) {
	var filter Filter
	if err := util.BindAndValidate(c, &filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	logs, err := h.svc.Query(c.Request.Context(), &filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, logs, "Audit logs fetched successfully")
}

// @Summary Get audit log by ID
// @Description Get a specific audit log entry
// @Tags audit
// @Accept json
// @Produce json
// @Param id path string true "Audit Log ID"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/audit/{id} [get]
func (h *handler) GetAuditLog(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.Error(c, http.StatusBadRequest, nil)
		return
	}

	log, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, log, "Audit log fetched successfully")
}
