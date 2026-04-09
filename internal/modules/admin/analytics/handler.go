package analytics

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	sharedAnalytics "github.com/iamarpitzala/acareca/internal/shared/analytics"
	"github.com/iamarpitzala/acareca/internal/shared/response"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// @Summary Get user growth and retention metrics
// @Description Returns user growth statistics including total users, new users, active users, growth rate, and retention rate with timeline
// @Tags admin-analytics
// @Produce json
// @Param start_date query string false "Start date (YYYY-MM-DD)"
// @Param end_date query string false "End date (YYYY-MM-DD)"
// @Success 200 {object} RsUserGrowth
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/analytics/user-growth [get]
func (h *Handler) GetUserGrowth(c *gin.Context) {
	var filter Filter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.GetUserGrowth(c.Request.Context(), &filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "User growth metrics fetched successfully")
}

// @Summary Get subscription distribution and MRR
// @Description Returns subscription metrics including MRR, ARR, ARPU, churn rate, and distribution by plan
// @Tags admin-analytics
// @Produce json
// @Success 200 {object} RsSubscriptionMetrics
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/analytics/subscriptions [get]
func (h *Handler) GetSubscriptionMetrics(c *gin.Context) {
	result, err := h.svc.GetSubscriptionMetrics(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Subscription metrics fetched successfully")
}

// @Summary Get daily/weekly/monthly active users
// @Description Returns DAU, WAU, MAU metrics with DAU/MAU ratio and timeline
// @Tags admin-analytics
// @Produce json
// @Param start_date query string false "Start date (YYYY-MM-DD)"
// @Param end_date query string false "End date (YYYY-MM-DD)"
// @Success 200 {object} RsActiveUsers
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/analytics/active-users [get]
func (h *Handler) GetActiveUsers(c *gin.Context) {
	var filter Filter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.GetActiveUsers(c.Request.Context(), &filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Active users metrics fetched successfully")
}

// @Summary Get practitioner details with clinics and accountants
// @Description Returns detailed practitioner information including subscription, clinics, and associated accountants
// @Tags admin-analytics
// @Produce json
// @Param id path string true "Practitioner ID"
// @Success 200 {object} RsPractitionerDetail
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/analytics/practitioners/{id} [get]
func (h *Handler) GetPractitionerDetails(c *gin.Context) {
	idParam := c.Param("id")
	practitionerID, err := uuid.Parse(idParam)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.GetPractitionerDetails(c.Request.Context(), practitionerID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Practitioner details fetched successfully")
}

// @Summary List all practitioners with clinics and accountants
// @Description Returns paginated and filtered list of practitioners with their clinics and associated accountants
// @Tags admin-analytics
// @Produce json
// @Param email query string false "Filter by exact email"
// @Param name query string false "Filter by name (partial match)"
// @Param phone query string false "Filter by phone (partial match)"
// @Param has_active_subscription query bool false "Filter by active subscription"
// @Param subscription_name query string false "Filter by subscription plan name"
// @Param search query string false "Search across name and email"
// @Param limit query int false "Records per page (default: 10, max: 100)"
// @Param offset query int false "Records to skip (default: 0)"
// @Param sort_by query string false "Sort field: created_at, email, first_name, last_name (default: created_at)"
// @Param order_by query string false "Sort order: ASC or DESC (default: DESC)"
// @Success 200 {object} util.RsList
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/analytics/practitioners [get]
func (h *Handler) ListPractitionersWithDetails(c *gin.Context) {
	var filter PractitionerFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.ListPractitionersWithDetails(c.Request.Context(), &filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Practitioners list fetched successfully")
}

// Dashboard Handler Methods

// @Summary Get practitioner dashboard overview
// @Description Returns practitioner KPIs and user bifurcation
// @Tags admin-dashboard
// @Produce json
// @Success 200 {object} RsPractitionerOverview
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/analytics/practitioner/overview [get]
func (h *Handler) GetPractitionerOverview(c *gin.Context) {
	result, err := h.svc.GetPractitionerOverview(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Practitioner dashboard overview fetched")
}

// @Summary Get resource analytics
// @Description Returns resource analytics grouped by entity type with action counts
// @Tags admin-dashboard
// @Produce json
// @Param from query string false "Start date (YYYY-MM-DD)"
// @Param to query string false "End date (YYYY-MM-DD)"
// @Param group_by query string false "Group by: entity_type, action"
// @Success 200 {object} RsResourceAnalytics
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/analytics/practitioner/resource-analytics [get]
func (h *Handler) GetResourceAnalytics(c *gin.Context) {
	var filter ResourceAnalyticsFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// Validate filter
	if err := validateResourceAnalyticsFilter(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.GetResourceAnalytics(c.Request.Context(), &filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Practitioner resource analytics fetched")
}

// @Summary Get accountant dashboard overview
// @Description Returns accountant KPIs and invite status distribution
// @Tags admin-dashboard
// @Produce json
// @Success 200 {object} RsAccountantOverview
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/analytics/accountant/overview [get]
func (h *Handler) GetAccountantOverview(c *gin.Context) {
	result, err := h.svc.GetAccountantOverview(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Accountant dashboard overview fetched")
}

// @Summary Get resource access timeseries
// @Description Returns accountant resource access over time by resource type
// @Tags admin-dashboard
// @Produce json
// @Param from query string false "Start date (YYYY-MM-DD)"
// @Param to query string false "End date (YYYY-MM-DD)"
// @Param bucket query string false "Time bucket: day, week, month"
// @Success 200 {object} RsResourceAccessTimeseries
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/analytics/accountant/resource-access-timeseries [get]
func (h *Handler) GetResourceAccessTimeseries(c *gin.Context) {
	var filter DateRangeFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// Validate filter
	if err := validateDateRangeFilter(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.GetResourceAccessTimeseries(c.Request.Context(), &filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Accountant resource access timeseries fetched")
}

// @Summary Get platform revenue
// @Description Returns platform revenue over time
// @Tags admin-dashboard
// @Produce json
// @Param from query string false "Start date (YYYY-MM-DD)"
// @Param to query string false "End date (YYYY-MM-DD)"
// @Param bucket query string false "Time bucket: day, week, month"
// @Success 200 {object} RsPlatformRevenue
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/analytics/billing/platform-revenue [get]
func (h *Handler) GetPlatformRevenue(c *gin.Context) {
	var filter DateRangeFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// Validate filter
	if err := validateDateRangeFilter(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.GetPlatformRevenue(c.Request.Context(), &filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Platform revenue fetched")
}

// @Summary List subscription records
// @Description Returns paginated and filtered list of subscription records
// @Tags admin-dashboard
// @Produce json
// @Param search query string false "Search by practitioner name or email"
// @Param plan_name query string false "Filter by plan name"
// @Param status query string false "Filter by status"
// @Param from query string false "Filter by start date (YYYY-MM-DD)"
// @Param to query string false "Filter by end date (YYYY-MM-DD)"
// @Param limit query int false "Records per page (default: 20, max: 100)"
// @Param offset query int false "Records to skip (default: 0)"
// @Param sort_by query string false "Sort field: created_at, start_date, end_date (default: created_at)"
// @Param order_by query string false "Sort order: ASC or DESC (default: DESC)"
// @Success 200 {object} util.RsList
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/analytics/billing/subscriptions [get]
func (h *Handler) ListSubscriptionRecords(c *gin.Context) {
	var filter SubscriptionRecordFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// Validate filter
	if err := validateSubscriptionRecordFilter(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.ListSubscriptionRecords(c.Request.Context(), &filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Subscription records fetched")
}

// @Summary Get plan distribution
// @Description Returns plan distribution with historical revenue and subscription counts
// @Tags admin-dashboard
// @Produce json
// @Param from query string false "Start date (YYYY-MM-DD)"
// @Param to query string false "End date (YYYY-MM-DD)"
// @Param bucket query string false "Time bucket: day, week, month"
// @Success 200 {object} RsPlanDistribution
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/analytics/billing/plan-distribution [get]
func (h *Handler) GetPlanDistribution(c *gin.Context) {
	var filter DateRangeFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// Validate filter
	if err := validateDateRangeFilter(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.GetPlanDistribution(c.Request.Context(), &filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Plan distribution fetched")
}

// Validation helper functions

func validateDateRangeFilter(filter *DateRangeFilter) error {
	if filter == nil {
		return nil
	}

	var from, to string
	if filter.From != nil {
		from = *filter.From
	}
	if filter.To != nil {
		to = *filter.To
	}

	if err := sharedAnalytics.ValidateDateRange(from, to); err != nil {
		return err
	}

	var bucket string
	if filter.Bucket != nil {
		bucket = *filter.Bucket
	}

	return sharedAnalytics.ValidateBucket(bucket)
}

func validateResourceAnalyticsFilter(filter *ResourceAnalyticsFilter) error {
	if filter == nil {
		return nil
	}

	var from, to string
	if filter.From != nil {
		from = *filter.From
	}
	if filter.To != nil {
		to = *filter.To
	}

	return sharedAnalytics.ValidateDateRange(from, to)
}

func validateSubscriptionRecordFilter(filter *SubscriptionRecordFilter) error {
	if filter == nil {
		return nil
	}

	if err := sharedAnalytics.SanitizeSearchTerm(filter.Search); err != nil {
		return err
	}

	if err := sharedAnalytics.ValidatePagination(filter.Limit, filter.Offset); err != nil {
		return err
	}

	var from, to string
	if filter.From != nil {
		from = *filter.From
	}
	if filter.To != nil {
		to = *filter.To
	}

	return sharedAnalytics.ValidateDateRange(from, to)
}
