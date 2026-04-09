package analytics

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

// User Growth & Retention Models

type RsUserGrowth struct {
	TotalUsers         int               `json:"total_users" db:"total_users"`
	TotalPractitioners int               `json:"total_practitioners" db:"total_practitioners"`
	TotalAccountants   int               `json:"total_accountants" db:"total_accountants"`
	NewUsers30Days     int               `json:"new_users_30_days" db:"new_users_30_days"`
	ActiveUsers30Days  int               `json:"active_users_30_days" db:"active_users_30_days"`
	GrowthRate         float64           `json:"growth_rate_percentage"`
	RetentionRate      float64           `json:"retention_rate_percentage"`
	Timeline           []UserGrowthPoint `json:"timeline"`
}

type UserGrowthPoint struct {
	Date        string `json:"date"`
	NewUsers    int    `json:"new_users"`
	TotalUsers  int    `json:"total_users"`
	ActiveUsers int    `json:"active_users"`
}

// Subscription Distribution & MRR Models

type RsSubscriptionMetrics struct {
	TotalActiveSubscriptions int                        `json:"total_active_subscriptions"`
	MRR                      float64                    `json:"mrr"`
	ARR                      float64                    `json:"arr"`
	ARPU                     float64                    `json:"arpu"`
	ChurnRate                float64                    `json:"churn_rate_percentage"`
	Distribution             []SubscriptionDistribution `json:"distribution"`
}

type SubscriptionDistribution struct {
	PlanID     int     `json:"plan_id"`
	PlanName   string  `json:"plan_name"`
	Count      int     `json:"count"`
	Revenue    float64 `json:"revenue"`
	Percentage float64 `json:"percentage"`
}

// Daily/Monthly Active Users Models

type RsActiveUsers struct {
	DAU           int                `json:"dau"`
	WAU           int                `json:"wau"`
	MAU           int                `json:"mau"`
	DAUToMAURatio float64            `json:"dau_to_mau_ratio"`
	Timeline      []ActiveUsersPoint `json:"timeline"`
}

type ActiveUsersPoint struct {
	Date        string `json:"date"`
	ActiveUsers int    `json:"active_users"`
}

// Practitioner with Clinics and Accountants Models

type RsPractitionerDetail struct {
	ID               uuid.UUID         `json:"id"`
	Name             string            `json:"name"`
	Email            string            `json:"email"`
	Phone            *string           `json:"phone"`
	CreatedAt        time.Time         `json:"created_at" db:"created_at"`
	Subscription     *SubscriptionInfo `json:"subscription,omitempty"`
	Clinics          []ClinicInfo      `json:"clinics"`
	TotalClinics     int               `json:"total_clinics"`
	TotalAccountants int               `json:"total_accountants"`
}

type SubscriptionInfo struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
}

type ClinicInfo struct {
	ID          uuid.UUID        `json:"id"`
	Name        string           `json:"name"`
	ABN         *string          `json:"abn,omitempty"`
	Accountants []AccountantInfo `json:"accountants"`
}

type AccountantInfo struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	Phone       *string   `json:"phone,omitempty"`
	Permissions string    `json:"permissions"`
}

// Filter for analytics queries
type Filter struct {
	StartDate *time.Time `form:"start_date"`
	EndDate   *time.Time `form:"end_date"`
	Interval  *string    `form:"interval"` // day, week, month
}

// PractitionerFilter for listing practitioners with details
type PractitionerFilter struct {
	Email                 *string `form:"email"`
	Name                  *string `form:"name"`
	Phone                 *string `form:"phone"`
	HasActiveSubscription *bool   `form:"has_active_subscription"`
	SubscriptionName      *string `form:"subscription_name"`
	common.Filter
}

var practitionerColumns = map[string]string{
	"id":                "p.id",
	"email":             "u.email",
	"first_name":        "u.first_name",
	"last_name":         "u.last_name",
	"name":              "CONCAT(u.first_name, ' ', u.last_name)",
	"phone":             "u.phone",
	"subscription_name": "s.name",
	"created_at":        "p.created_at",
}

var practitionerSearchCols = []string{
	"u.email",
	"CONCAT(u.first_name, ' ', u.last_name)",
}

func (f *PractitionerFilter) MapToFilter() common.Filter {
	filters := map[string]interface{}{}
	operators := map[string]common.Operator{}

	if f.Email != nil && *f.Email != "" {
		filters["email"] = *f.Email
	}

	if f.Name != nil && *f.Name != "" {
		filters["name"] = "%" + *f.Name + "%"
		operators["name"] = common.OpLike
	}

	if f.Phone != nil && *f.Phone != "" {
		filters["phone"] = "%" + *f.Phone + "%"
		operators["phone"] = common.OpLike
	}

	if f.SubscriptionName != nil && *f.SubscriptionName != "" {
		filters["subscription_name"] = "%" + *f.SubscriptionName + "%"
		operators["subscription_name"] = common.OpLike
	}

	return common.NewFilter(f.Search, filters, operators, f.Limit, f.Offset, f.SortBy, f.OrderBy)
}

// Practitioner Dashboard Models

type RsPractitionerOverview struct {
	KPIs            PractitionerKPIs `json:"kpis"`
	UserBifurcation UserBifurcation  `json:"user_bifurcation"`
}

type PractitionerKPIs struct {
	TotalPractitioners  int `json:"total_practitioners"`
	ActiveSubscriptions int `json:"active_subscriptions"`
	NoActivePlan        int `json:"no_active_plan"`
	TotalInvites        int `json:"total_invites"`
}

type UserBifurcation struct {
	Total  int         `json:"total"`
	ByRole []RoleCount `json:"by_role"`
}

type RoleCount struct {
	Role  string `json:"role"`
	Count int    `json:"count"`
}

type RsResourceAnalytics struct {
	Meta ResourceAnalyticsMeta `json:"meta"`
	Rows []ResourceRow         `json:"rows"`
}

type ResourceAnalyticsMeta struct {
	From    string `json:"from"`
	To      string `json:"to"`
	GroupBy string `json:"group_by"`
}

type ResourceRow struct {
	EntityType string       `json:"entity_type"`
	Actions    ActionCounts `json:"actions"`
	Total      int          `json:"total"`
}

type ActionCounts struct {
	Create int `json:"create"`
	Read   int `json:"read"`
	Update int `json:"update"`
	Delete int `json:"delete"`
}

// Accountant Dashboard Models

type RsAccountantOverview struct {
	KPIs             AccountantKPIs   `json:"kpis"`
	InvitesStatusPie InvitesStatusPie `json:"invites_status_pie"`
}

type AccountantKPIs struct {
	TotalPractitioners int `json:"total_practitioners"`
	TotalAccountants   int `json:"total_accountants"`
	TotalInvites       int `json:"total_invites"`
	TotalPermissions   int `json:"total_permissions"`
}

type InvitesStatusPie struct {
	Total    int           `json:"total"`
	ByStatus []StatusCount `json:"by_status"`
}

type StatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type RsResourceAccessTimeseries struct {
	Meta   TimeseriesMeta   `json:"meta"`
	Series []ResourceSeries `json:"series"`
}

type TimeseriesMeta struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Bucket string `json:"bucket"`
}

type ResourceSeries struct {
	ResourceType string      `json:"resource_type"`
	Points       []TimePoint `json:"points"`
}

type TimePoint struct {
	Timestamp string `json:"ts"`
	Count     int    `json:"count"`
}

// Billing Dashboard Models

type RsPlatformRevenue struct {
	Meta   RevenueMeta    `json:"meta"`
	Series []RevenuePoint `json:"series"`
}

type RevenueMeta struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Bucket   string `json:"bucket"`
	Currency string `json:"currency"`
}

type RevenuePoint struct {
	Timestamp string  `json:"ts"`
	Revenue   float64 `json:"revenue"`
}

type RsSubscriptionRecord struct {
	SubscriptionID    string    `json:"subscription_id"`
	PractitionerID    string    `json:"practitioner_id"`
	PractitionerName  string    `json:"practitioner_name"`
	PractitionerEmail string    `json:"practitioner_email"`
	PlanID            string    `json:"plan_id"`
	PlanName          string    `json:"plan_name"`
	Status            string    `json:"status"`
	Amount            float64   `json:"amount"`
	Currency          string    `json:"currency"`
	StartDate         time.Time `json:"start_date"`
	EndDate           time.Time `json:"end_date"`
	CreatedAt         time.Time `json:"created_at"`
}

type RsPlanDistribution struct {
	Meta  RevenueMeta        `json:"meta"`
	Plans []PlanDistribution `json:"plans"`
}

type PlanDistribution struct {
	PlanID   string                  `json:"plan_id"`
	PlanName string                  `json:"plan_name"`
	Counts   PlanCounts              `json:"counts"`
	Series   []PlanDistributionPoint `json:"series"`
}

type PlanCounts struct {
	TotalSubscriptions  int `json:"total_subscriptions"`
	ActiveSubscriptions int `json:"active_subscriptions"`
}

type PlanDistributionPoint struct {
	Timestamp           string  `json:"ts"`
	Revenue             float64 `json:"revenue"`
	NewSubscriptions    int     `json:"new_subscriptions"`
	ActiveSubscriptions int     `json:"active_subscriptions"`
}

// Dashboard Filter Models

type DateRangeFilter struct {
	From   *string `form:"from"`
	To     *string `form:"to"`
	Bucket *string `form:"bucket"` // day, week, month
}

type ResourceAnalyticsFilter struct {
	From    *string `form:"from"`
	To      *string `form:"to"`
	GroupBy *string `form:"group_by"` // entity_type, action
}

type SubscriptionRecordFilter struct {
	Search   *string `form:"search"`
	PlanName *string `form:"plan_name"`
	Status   *string `form:"status"`
	From     *string `form:"from"`
	To       *string `form:"to"`
	Limit    *int    `form:"limit"`
	Offset   *int    `form:"offset"`
	SortBy   *string `form:"sort_by"`
	OrderBy  *string `form:"order_by"`
}
