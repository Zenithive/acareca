package audit

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/response"
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
// @Param practice_id query string false "Practice ID"
// @Param user_id query string false "User ID"
// @Param module query string false "Module name"
// @Param action query string false "Action name"
// @Param entity_type query string false "Entity type"
// @Param entity_id query string false "Entity ID"
// @Param start_date query string false "Start date (RFC3339)"
// @Param end_date query string false "End date (RFC3339)"
// @Param limit query int false "Limit" default(100)
// @Param offset query int false "Offset" default(0)
// @Success 200 {array} RsAuditLog
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /admin/audit [get]
func (h *handler) ListAuditLogs(c *gin.Context) {
	params := QueryParams{
		Limit:  100,
		Offset: 0,
	}

	// Parse query parameters
	if practiceID := c.Query("practice_id"); practiceID != "" {
		params.PracticeID = &practiceID
	}
	if userID := c.Query("user_id"); userID != "" {
		params.UserID = &userID
	}
	if module := c.Query("module"); module != "" {
		params.Module = &module
	}
	if action := c.Query("action"); action != "" {
		params.Action = &action
	}
	if entityType := c.Query("entity_type"); entityType != "" {
		params.EntityType = &entityType
	}
	if entityID := c.Query("entity_id"); entityID != "" {
		params.EntityID = &entityID
	}

	if startDate := c.Query("start_date"); startDate != "" {
		t, err := time.Parse(time.RFC3339, startDate)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		params.StartDate = &t
	}

	if endDate := c.Query("end_date"); endDate != "" {
		t, err := time.Parse(time.RFC3339, endDate)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		params.EndDate = &t
	}

	if limit := c.Query("limit"); limit != "" {
		l, err := strconv.Atoi(limit)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		params.Limit = l
	}

	if offset := c.Query("offset"); offset != "" {
		o, err := strconv.Atoi(offset)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		params.Offset = o
	}

	logs, err := h.svc.Query(c.Request.Context(), params)
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
// @Success 200 {object} RsAuditLog
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
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
