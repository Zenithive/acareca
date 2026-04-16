package analytics

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	sharedAnalytics "github.com/iamarpitzala/acareca/internal/shared/analytics"
	"github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	GetUserGrowth(ctx context.Context, startDate, endDate time.Time) (*RsUserGrowth, error)
	GetSubscriptionMetrics(ctx context.Context) (*RsSubscriptionMetrics, error)
	GetActiveUsers(ctx context.Context, startDate, endDate time.Time) (*RsActiveUsers, error)
	GetPractitionerDetails(ctx context.Context, practitionerID uuid.UUID) (*RsPractitionerDetail, error)
	ListPractitionersWithDetails(ctx context.Context, filter *PractitionerFilter) ([]*RsPractitionerDetail, int, error)

	// Dashboard APIs
	GetPractitionerOverview(ctx context.Context) (*RsPractitionerOverview, error)
	GetResourceAnalytics(ctx context.Context, filter *ResourceAnalyticsFilter) (*RsResourceAnalytics, error)
	GetAccountantOverview(ctx context.Context) (*RsAccountantOverview, error)
	GetResourceAccessTimeseries(ctx context.Context, filter *DateRangeFilter) (*RsResourceAccessTimeseries, error)
	GetPlatformRevenue(ctx context.Context, filter *DateRangeFilter) (*RsPlatformRevenue, error)
	ListSubscriptionRecords(ctx context.Context, filter *SubscriptionRecordFilter) ([]*RsSubscriptionRecord, int, error)
	GetPlanDistribution(ctx context.Context, filter *DateRangeFilter) (*RsPlanDistribution, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

// GetUserGrowth retrieves user growth and retention metrics
func (r *repository) GetUserGrowth(ctx context.Context, startDate, endDate time.Time) (*RsUserGrowth, error) {
	var result RsUserGrowth

	// Total users
	query := `
		SELECT 
			COUNT(*) as total_users,
			COUNT(CASE WHEN role = 'PRACTITIONER' THEN 1 END) as total_practitioners,
			COUNT(CASE WHEN role = 'ACCOUNTANT' THEN 1 END) as total_accountants,
			COUNT(CASE WHEN created_at >= NOW() - INTERVAL '30 days' THEN 1 END) as new_users_30_days
		FROM tbl_user
		WHERE deleted_at IS NULL
	`
	err := r.db.GetContext(ctx, &result, query)
	if err != nil {
		return nil, fmt.Errorf("get user counts: %w", err)
	}

	// Active users (users with audit log entries in last 30 days)
	activeQuery := `
		SELECT COUNT(DISTINCT user_id) as active_users
		FROM tbl_audit_log
		WHERE created_at >= NOW() - INTERVAL '30 days'
	`
	err = r.db.GetContext(ctx, &result.ActiveUsers30Days, activeQuery)
	if err != nil {
		return nil, fmt.Errorf("get active users: %w", err)
	}

	denominator := float64(result.TotalUsers - result.NewUsers30Days)

	// Calculate growth rate
	if denominator > 0 {
		result.GrowthRate = roundToTwo((float64(result.NewUsers30Days) / denominator) * 100)
	} else if result.TotalUsers > 0 {
		// If all users are new (Total == New), growth is technically 100% or "New"
		result.GrowthRate = 100.0
	} else {
		// No users at all
		result.GrowthRate = 0.0
	}

	// Calculate retention rate
	if result.TotalUsers > 0 {
		result.RetentionRate = roundToTwo((float64(result.ActiveUsers30Days) / float64(result.TotalUsers)) * 100)
	} else {
		result.RetentionRate = 0.0
	}

	// Timeline data
	timelineQuery := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as new_users,
			SUM(COUNT(*)) OVER (ORDER BY DATE(created_at)) as total_users
		FROM tbl_user
		WHERE created_at BETWEEN $1 AND $2
			AND deleted_at IS NULL
		GROUP BY DATE(created_at)
		ORDER BY date
	`
	rows, err := r.db.QueryxContext(ctx, timelineQuery, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("get timeline: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var point UserGrowthPoint
		var date time.Time
		err := rows.Scan(&date, &point.NewUsers, &point.TotalUsers)
		if err != nil {
			return nil, fmt.Errorf("scan timeline: %w", err)
		}
		point.Date = date.Format("2006-01-02")
		result.Timeline = append(result.Timeline, point)
	}

	return &result, nil
}

// GetSubscriptionMetrics retrieves subscription distribution and MRR
func (r *repository) GetSubscriptionMetrics(ctx context.Context) (*RsSubscriptionMetrics, error) {
	// Single query: active totals + MRR + churn in one shot
	query := `
		WITH active AS (
			SELECT ps.id, s.price, s.duration_days
			FROM tbl_practitioner_subscription ps
			JOIN tbl_subscription s ON ps.subscription_id = s.id
			WHERE ps.status = 'ACTIVE' AND ps.deleted_at IS NULL AND s.deleted_at IS NULL
		),
		churn AS (
			SELECT
				COUNT(*) FILTER (WHERE status = 'ACTIVE' AND created_at <= NOW() - INTERVAL '30 days') AS active_30_ago,
				COUNT(*) FILTER (WHERE status IN ('CANCELLED','EXPIRED') AND updated_at >= NOW() - INTERVAL '30 days') AS churned_30
			FROM tbl_practitioner_subscription
			WHERE deleted_at IS NULL
		)
		SELECT
			(SELECT COUNT(*) FROM active) AS total_active,
			(SELECT COALESCE(SUM(price / NULLIF(duration_days,0) * 30), 0) FROM active) AS mrr,
			CASE WHEN c.active_30_ago > 0 THEN (c.churned_30::float / c.active_30_ago::float) * 100 ELSE 0 END AS churn_rate
		FROM churn c
	`
	var row struct {
		TotalActive int     `db:"total_active"`
		MRR         float64 `db:"mrr"`
		ChurnRate   float64 `db:"churn_rate"`
	}
	if err := r.db.QueryRowxContext(ctx, query).StructScan(&row); err != nil {
		return nil, fmt.Errorf("get subscription metrics: %w", err)
	}

	result := RsSubscriptionMetrics{
		TotalActiveSubscriptions: row.TotalActive,
		MRR:                      row.MRR,
		ARR:                      row.MRR * 12,
		ChurnRate:                row.ChurnRate,
	}
	if row.TotalActive > 0 {
		result.ARPU = row.MRR / float64(row.TotalActive)
	}

	// Distribution by plan — single query
	distQuery := `
		SELECT
			s.id AS plan_id,
			s.name AS plan_name,
			COUNT(*) AS count,
			SUM(s.price / NULLIF(s.duration_days,0) * 30) AS revenue
		FROM tbl_practitioner_subscription ps
		JOIN tbl_subscription s ON ps.subscription_id = s.id
		WHERE ps.status = 'ACTIVE' AND ps.deleted_at IS NULL AND s.deleted_at IS NULL
		GROUP BY s.id, s.name
		ORDER BY count DESC
	`
	rows, err := r.db.QueryxContext(ctx, distQuery)
	if err != nil {
		return nil, fmt.Errorf("get distribution: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var dist SubscriptionDistribution
		if err := rows.Scan(&dist.PlanID, &dist.PlanName, &dist.Count, &dist.Revenue); err != nil {
			return nil, fmt.Errorf("scan distribution: %w", err)
		}
		if row.TotalActive > 0 {
			dist.Percentage = (float64(dist.Count) / float64(row.TotalActive)) * 100
		}
		result.Distribution = append(result.Distribution, dist)
	}

	return &result, nil
}

// GetActiveUsers retrieves DAU/WAU/MAU metrics
func (r *repository) GetActiveUsers(ctx context.Context, startDate, endDate time.Time) (*RsActiveUsers, error) {
	var result RsActiveUsers

	// DAU (today)
	dauQuery := `
		SELECT COUNT(DISTINCT user_id)
		FROM tbl_audit_log
		WHERE DATE(created_at) = CURRENT_DATE
	`
	err := r.db.GetContext(ctx, &result.DAU, dauQuery)
	if err != nil {
		return nil, fmt.Errorf("get DAU: %w", err)
	}

	// WAU (last 7 days)
	wauQuery := `
		SELECT COUNT(DISTINCT user_id)
		FROM tbl_audit_log
		WHERE created_at >= NOW() - INTERVAL '7 days'
	`
	err = r.db.GetContext(ctx, &result.WAU, wauQuery)
	if err != nil {
		return nil, fmt.Errorf("get WAU: %w", err)
	}

	// MAU (last 30 days)
	mauQuery := `
		SELECT COUNT(DISTINCT user_id)
		FROM tbl_audit_log
		WHERE created_at >= NOW() - INTERVAL '30 days'
	`
	err = r.db.GetContext(ctx, &result.MAU, mauQuery)
	if err != nil {
		return nil, fmt.Errorf("get MAU: %w", err)
	}

	// DAU/MAU ratio
	if result.MAU > 0 {
		result.DAUToMAURatio = (float64(result.DAU) / float64(result.MAU)) * 100
	}

	// Timeline
	timelineQuery := `
		SELECT 
			DATE(created_at) as date,
			COUNT(DISTINCT user_id) as active_users
		FROM tbl_audit_log
		WHERE created_at BETWEEN $1 AND $2
		GROUP BY DATE(created_at)
		ORDER BY date
	`
	rows, err := r.db.QueryxContext(ctx, timelineQuery, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("get timeline: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var point ActiveUsersPoint
		var date time.Time
		err := rows.Scan(&date, &point.ActiveUsers)
		if err != nil {
			return nil, fmt.Errorf("scan timeline: %w", err)
		}
		point.Date = date.Format("2006-01-02")
		result.Timeline = append(result.Timeline, point)
	}

	return &result, nil
}

// GetPractitionerDetails retrieves detailed practitioner info with clinics and accountants
func (r *repository) GetPractitionerDetails(ctx context.Context, practitionerID uuid.UUID) (*RsPractitionerDetail, error) {
	// Get practitioner basic info
	practQuery := `
		SELECT 
			p.id,
			u.first_name || ' ' || u.last_name as name,
			u.email,
			u.phone,
			u.created_at
		FROM tbl_practitioner p
		JOIN tbl_user u ON p.user_id = u.id
		WHERE p.id = $1 AND p.deleted_at IS NULL
	`
	var result RsPractitionerDetail
	err := r.db.GetContext(ctx, &result, practQuery, practitionerID)
	if err != nil {
		return nil, fmt.Errorf("get practitioner: %w", err)
	}

	// Get subscription info
	subQuery := `
		SELECT 
			s.id,
			s.name,
			ps.status,
			ps.start_date,
			ps.end_date
		FROM tbl_practitioner_subscription ps
		JOIN tbl_subscription s ON ps.subscription_id = s.id
		WHERE ps.practitioner_id = $1
			AND ps.status = 'ACTIVE'
			AND ps.deleted_at IS NULL
		ORDER BY ps.created_at DESC
		LIMIT 1
	`
	var sub SubscriptionInfo
	err = r.db.GetContext(ctx, &sub, subQuery, practitionerID)
	if err == nil {
		result.Subscription = &sub
	}

	// Get clinics with accountants
	clinicQuery := `
		SELECT 
			c.id,
			c.name,
			c.abn
		FROM tbl_clinic c
		WHERE c.practitioner_id = $1 AND c.deleted_at IS NULL
		ORDER BY c.created_at DESC
	`
	rows, err := r.db.QueryxContext(ctx, clinicQuery, practitionerID)
	if err != nil {
		return nil, fmt.Errorf("get clinics: %w", err)
	}
	defer rows.Close()

	accountantMap := make(map[uuid.UUID]bool)
	for rows.Next() {
		var clinic ClinicInfo
		err := rows.Scan(&clinic.ID, &clinic.Name, &clinic.ABN)
		if err != nil {
			return nil, fmt.Errorf("scan clinic: %w", err)
		}

		// Get accountants for this clinic
		accQuery := `
			SELECT DISTINCT
				a.id,
				u.first_name || ' ' || u.last_name as name,
				u.email,
				u.phone,
				'read,update' as permissions
			FROM tbl_invite_permissions ip
			JOIN tbl_accountant a ON ip.accountant_id = a.id
			JOIN tbl_user u ON a.user_id = u.id
			WHERE ip.practitioner_id = $1
				AND ip.entity_id = $2
				AND ip.deleted_at IS NULL
				AND a.deleted_at IS NULL
		`
		accRows, err := r.db.QueryxContext(ctx, accQuery, practitionerID, clinic.ID)
		if err != nil {
			return nil, fmt.Errorf("get accountants: %w", err)
		}

		for accRows.Next() {
			var acc AccountantInfo
			err := accRows.Scan(&acc.ID, &acc.Name, &acc.Email, &acc.Phone, &acc.Permissions)
			if err != nil {
				accRows.Close()
				return nil, fmt.Errorf("scan accountant: %w", err)
			}
			clinic.Accountants = append(clinic.Accountants, acc)
			accountantMap[acc.ID] = true
		}
		accRows.Close()

		result.Clinics = append(result.Clinics, clinic)
	}

	result.TotalClinics = len(result.Clinics)
	result.TotalAccountants = len(accountantMap)

	return &result, nil
}

// ListPractitionersWithDetails retrieves filtered and paginated list of practitioners with details
// Fixed N+1 query problem by using batch queries
func (r *repository) ListPractitionersWithDetails(ctx context.Context, filter *PractitionerFilter) ([]*RsPractitionerDetail, int, error) {
	cf := filter.MapToFilter()

	// Base query for practitioners with subscription join
	baseQuery := `
		FROM tbl_practitioner p
		JOIN tbl_user u ON p.user_id = u.id
		LEFT JOIN tbl_practitioner_subscription ps ON ps.practitioner_id = p.id AND ps.status = 'ACTIVE' AND ps.deleted_at IS NULL
		LEFT JOIN tbl_subscription s ON ps.subscription_id = s.id
		WHERE p.deleted_at IS NULL
	`

	// Add subscription filter if specified
	if filter.HasActiveSubscription != nil {
		if *filter.HasActiveSubscription {
			baseQuery += " AND ps.id IS NOT NULL"
		} else {
			baseQuery += " AND ps.id IS NULL"
		}
	}

	// Get total count
	countQuery, countArgs := common.BuildQuery(baseQuery, cf, practitionerColumns, practitionerSearchCols, true)
	var total int
	err := r.db.GetContext(ctx, &total, r.db.Rebind(countQuery), countArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("count practitioners: %w", err)
	}

	// Get practitioner IDs with sorting and pagination
	selectQuery := "SELECT DISTINCT p.id, p.created_at, u.email, u.first_name, u.last_name " + baseQuery
	listQuery, listArgs := common.BuildQuery(selectQuery, cf, practitionerColumns, practitionerSearchCols, false)

	rows, err := r.db.QueryxContext(ctx, r.db.Rebind(listQuery), listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("get practitioner ids: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		var createdAt time.Time
		var email, firstName, lastName string
		if err := rows.Scan(&id, &createdAt, &email, &firstName, &lastName); err != nil {
			return nil, 0, fmt.Errorf("scan id: %w", err)
		}
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return []*RsPractitionerDetail{}, total, nil
	}

	// Batch fetch all data in 3 queries instead of N queries
	practitioners, err := r.batchGetPractitioners(ctx, ids)
	if err != nil {
		return nil, 0, err
	}

	subscriptions, err := r.batchGetSubscriptions(ctx, ids)
	if err != nil {
		return nil, 0, err
	}

	clinics, err := r.batchGetClinicsWithAccountants(ctx, ids)
	if err != nil {
		return nil, 0, err
	}

	// Assemble results
	results := make([]*RsPractitionerDetail, 0, len(ids))
	for _, id := range ids {
		detail := practitioners[id]
		if sub, ok := subscriptions[id]; ok {
			detail.Subscription = &sub
		}
		if clinicList, ok := clinics[id]; ok {
			detail.Clinics = clinicList
			detail.TotalClinics = len(clinicList)

			// Count unique accountants
			accountantMap := make(map[uuid.UUID]bool)
			for _, clinic := range clinicList {
				for _, acc := range clinic.Accountants {
					accountantMap[acc.ID] = true
				}
			}
			detail.TotalAccountants = len(accountantMap)
		}
		results = append(results, &detail)
	}

	return results, total, nil
}

// batchGetPractitioners fetches basic practitioner info for multiple IDs in one query
func (r *repository) batchGetPractitioners(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]RsPractitionerDetail, error) {
	query := `
		SELECT 
			p.id,
			u.first_name || ' ' || u.last_name as name,
			u.email,
			u.phone,
			u.created_at
		FROM tbl_practitioner p
		JOIN tbl_user u ON p.user_id = u.id
		WHERE p.id = ANY($1) AND p.deleted_at IS NULL
	`

	rows, err := r.db.QueryxContext(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("batch get practitioners: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]RsPractitionerDetail)
	for rows.Next() {
		var detail RsPractitionerDetail
		err := rows.Scan(&detail.ID, &detail.Name, &detail.Email, &detail.Phone, &detail.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan practitioner: %w", err)
		}
		detail.Clinics = []ClinicInfo{}
		result[detail.ID] = detail
	}

	return result, nil
}

// batchGetSubscriptions fetches active subscriptions for multiple practitioners in one query
func (r *repository) batchGetSubscriptions(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]SubscriptionInfo, error) {
	query := `
		SELECT DISTINCT ON (ps.practitioner_id)
			ps.practitioner_id,
			s.id,
			s.name,
			ps.status,
			ps.start_date,
			ps.end_date
		FROM tbl_practitioner_subscription ps
		JOIN tbl_subscription s ON ps.subscription_id = s.id
		WHERE ps.practitioner_id = ANY($1)
			AND ps.status = 'ACTIVE'
			AND ps.deleted_at IS NULL
		ORDER BY ps.practitioner_id, ps.created_at DESC
	`

	rows, err := r.db.QueryxContext(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("batch get subscriptions: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]SubscriptionInfo)
	for rows.Next() {
		var practitionerID uuid.UUID
		var sub SubscriptionInfo
		err := rows.Scan(&practitionerID, &sub.ID, &sub.Name, &sub.Status, &sub.StartDate, &sub.EndDate)
		if err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}
		result[practitionerID] = sub
	}

	return result, nil
}

// batchGetClinicsWithAccountants fetches clinics and accountants for multiple practitioners in two queries
func (r *repository) batchGetClinicsWithAccountants(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]ClinicInfo, error) {
	// Get all clinics for these practitioners
	clinicQuery := `
		SELECT 
			c.practitioner_id,
			c.id,
			c.name,
			c.abn
		FROM tbl_clinic c
		WHERE c.practitioner_id = ANY($1) AND c.deleted_at IS NULL
		ORDER BY c.practitioner_id, c.created_at DESC
	`

	rows, err := r.db.QueryxContext(ctx, clinicQuery, ids)
	if err != nil {
		return nil, fmt.Errorf("batch get clinics: %w", err)
	}
	defer rows.Close()

	clinicMap := make(map[uuid.UUID][]ClinicInfo)
	var allClinicIDs []uuid.UUID
	clinicToPractitioner := make(map[uuid.UUID]uuid.UUID)

	for rows.Next() {
		var practitionerID uuid.UUID
		var clinic ClinicInfo
		err := rows.Scan(&practitionerID, &clinic.ID, &clinic.Name, &clinic.ABN)
		if err != nil {
			return nil, fmt.Errorf("scan clinic: %w", err)
		}
		clinic.Accountants = []AccountantInfo{}
		clinicMap[practitionerID] = append(clinicMap[practitionerID], clinic)
		allClinicIDs = append(allClinicIDs, clinic.ID)
		clinicToPractitioner[clinic.ID] = practitionerID
	}

	if len(allClinicIDs) == 0 {
		return clinicMap, nil
	}

	// Get all accountants for these clinics in one query
	accQuery := `
		SELECT DISTINCT
			ip.entity_id as clinic_id,
			a.id,
			u.first_name || ' ' || u.last_name as name,
			u.email,
			u.phone,
			'read,update' as permissions
		FROM tbl_invite_permissions ip
		JOIN tbl_accountant a ON ip.accountant_id = a.id
		JOIN tbl_user u ON a.user_id = u.id
		WHERE ip.entity_id = ANY($1)
			AND ip.deleted_at IS NULL
			AND a.deleted_at IS NULL
	`

	accRows, err := r.db.QueryxContext(ctx, accQuery, allClinicIDs)
	if err != nil {
		return nil, fmt.Errorf("batch get accountants: %w", err)
	}
	defer accRows.Close()

	// Map accountants to clinics
	clinicAccountants := make(map[uuid.UUID][]AccountantInfo)
	for accRows.Next() {
		var clinicID uuid.UUID
		var acc AccountantInfo
		err := accRows.Scan(&clinicID, &acc.ID, &acc.Name, &acc.Email, &acc.Phone, &acc.Permissions)
		if err != nil {
			return nil, fmt.Errorf("scan accountant: %w", err)
		}
		clinicAccountants[clinicID] = append(clinicAccountants[clinicID], acc)
	}

	// Attach accountants to clinics
	for practitionerID, clinics := range clinicMap {
		for i := range clinics {
			if accountants, ok := clinicAccountants[clinics[i].ID]; ok {
				clinicMap[practitionerID][i].Accountants = accountants
			}
		}
	}

	return clinicMap, nil
}

// Dashboard Repository Methods

// GetPractitionerOverview retrieves practitioner dashboard overview
func (r *repository) GetPractitionerOverview(ctx context.Context) (*RsPractitionerOverview, error) {
	var result RsPractitionerOverview

	// Get KPIs
	kpiQuery := `
		SELECT 
			COUNT(DISTINCT p.id) as total_practitioners,
			COUNT(DISTINCT CASE WHEN ps.status = 'ACTIVE' THEN ps.id END) as active_subscriptions,
			COUNT(DISTINCT CASE WHEN ps.id IS NULL THEN p.id END) as no_active_plan,
			(SELECT COUNT(*) FROM tbl_invitation) as total_invites
		FROM tbl_practitioner p
		LEFT JOIN tbl_practitioner_subscription ps ON ps.practitioner_id = p.id AND ps.status = 'ACTIVE' AND ps.deleted_at IS NULL
		WHERE p.deleted_at IS NULL
	`
	err := r.db.QueryRowContext(ctx, kpiQuery).Scan(
		&result.KPIs.TotalPractitioners,
		&result.KPIs.ActiveSubscriptions,
		&result.KPIs.NoActivePlan,
		&result.KPIs.TotalInvites,
	)
	if err != nil {
		return nil, fmt.Errorf("get practitioner KPIs: %w", err)
	}

	// Get user bifurcation
	bifurcationQuery := `
		SELECT role, COUNT(*) as count
		FROM tbl_user
		WHERE deleted_at IS NULL
		GROUP BY role
	`
	rows, err := r.db.QueryxContext(ctx, bifurcationQuery)
	if err != nil {
		return nil, fmt.Errorf("get user bifurcation: %w", err)
	}
	defer rows.Close()

	var total int
	for rows.Next() {
		var rc RoleCount
		if err := rows.Scan(&rc.Role, &rc.Count); err != nil {
			return nil, fmt.Errorf("scan role count: %w", err)
		}
		result.UserBifurcation.ByRole = append(result.UserBifurcation.ByRole, rc)
		total += rc.Count
	}
	result.UserBifurcation.Total = total

	return &result, nil
}

// GetResourceAnalytics retrieves resource analytics grouped by entity type
func (r *repository) GetResourceAnalytics(ctx context.Context, filter *ResourceAnalyticsFilter) (*RsResourceAnalytics, error) {
	from, to := sharedAnalytics.ParseDateRange(filter.From, filter.To, sharedAnalytics.DefaultDaysShort)
	groupBy := "entity_type"
	if filter.GroupBy != nil {
		groupBy = *filter.GroupBy
	}

	result := RsResourceAnalytics{
		Meta: ResourceAnalyticsMeta{From: from, To: to, GroupBy: groupBy},
	}

	query := `
		SELECT 
			entity_type,
        COUNT(CASE WHEN action LIKE '%.created' THEN 1 END) as create_count,
        COUNT(CASE WHEN action LIKE '%.updated' THEN 1 END) as update_count,
        
   
        COUNT(CASE WHEN action LIKE '%.deleted' THEN 1 END) as delete_count,
			COUNT(*) as total
		FROM tbl_audit_log
		WHERE created_at BETWEEN $1 AND $2
			AND entity_type IN ('tbl_clinic', 'tbl_form', 'tbl_form_field_entry')
		GROUP BY entity_type
		ORDER BY total DESC
	`
	rows, err := r.db.QueryxContext(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("get resource analytics: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var row ResourceRow
		err := rows.Scan(&row.EntityType, &row.Actions.Create, &row.Actions.Update, &row.Actions.Delete, &row.Total)
		if err != nil {
			return nil, fmt.Errorf("scan resource row: %w", err)
		}
		result.Rows = append(result.Rows, row)
	}

	return &result, nil
}

// GetAccountantOverview retrieves accountant dashboard overview
func (r *repository) GetAccountantOverview(ctx context.Context) (*RsAccountantOverview, error) {
	var result RsAccountantOverview

	// Get KPIs
	kpiQuery := `
		SELECT 
			(SELECT COUNT(*) FROM tbl_practitioner WHERE deleted_at IS NULL) as total_practitioners,
			(SELECT COUNT(*) FROM tbl_accountant WHERE deleted_at IS NULL) as total_accountants,
			(SELECT COUNT(*) FROM tbl_invitation) as total_invites,
			COUNT(*) as total_permissions
		FROM tbl_invite_permissions
		WHERE deleted_at IS NULL
	`
	err := r.db.QueryRowContext(ctx, kpiQuery).Scan(
		&result.KPIs.TotalPractitioners,
		&result.KPIs.TotalAccountants,
		&result.KPIs.TotalInvites,
		&result.KPIs.TotalPermissions,
	)
	if err != nil {
		return nil, fmt.Errorf("get accountant KPIs: %w", err)
	}

	// Get invites status distribution from tbl_invitation
	statusQuery := `
		SELECT status, COUNT(*) as count
		FROM tbl_invitation
		GROUP BY status
	`
	rows, err := r.db.QueryxContext(ctx, statusQuery)
	if err != nil {
		return nil, fmt.Errorf("get invite status: %w", err)
	}
	defer rows.Close()

	var total int
	for rows.Next() {
		var sc StatusCount
		if err := rows.Scan(&sc.Status, &sc.Count); err != nil {
			return nil, fmt.Errorf("scan status count: %w", err)
		}
		result.InvitesStatusPie.ByStatus = append(result.InvitesStatusPie.ByStatus, sc)
		total += sc.Count
	}
	result.InvitesStatusPie.Total = total

	return &result, nil
}

// GetResourceAccessTimeseries retrieves accountant resource access over time
func (r *repository) GetResourceAccessTimeseries(ctx context.Context, filter *DateRangeFilter) (*RsResourceAccessTimeseries, error) {
	from, to := sharedAnalytics.ParseDateRange(filter.From, filter.To, sharedAnalytics.DefaultDaysShort)
	bucket := sharedAnalytics.ParseBucket(filter.Bucket, sharedAnalytics.BucketDay)
	dateTrunc, dateFormat := sharedAnalytics.GetBucketConfig(bucket)

	//query := buildTimeseriesQuery("tbl_audit_log", "entity_type", "created_at", "AND entity_type IN ('CLINIC', 'INVOICE', 'PATIENT', 'FORM')")

	entityList := []string{
		audit.EntityClinic,
		audit.EntityForm,
		audit.EntityFormFieldEntry,
		audit.EntityUser,
		audit.EntityInvitation,
	}

	inClause := fmt.Sprintf("AND entity_type IN ('%s')", strings.Join(entityList, "','"))

	query := buildTimeseriesQuery("tbl_audit_log", "entity_type", "created_at", inClause)
	rows, err := r.db.QueryxContext(ctx, query, dateTrunc, from, to)
	if err != nil {
		return nil, fmt.Errorf("get resource access timeseries: %w", err)
	}
	defer rows.Close()

	seriesMap, err := scanTimeseries(rows, dateFormat)
	if err != nil {
		return nil, fmt.Errorf("scan timeseries: %w", err)
	}

	result := RsResourceAccessTimeseries{
		Meta:   TimeseriesMeta{From: from, To: to, Bucket: bucket},
		Series: make([]ResourceSeries, 0),
	}

	for resourceType, points := range seriesMap {
		result.Series = append(result.Series, ResourceSeries{
			ResourceType: resourceType,
			Points:       points,
		})
	}

	return &result, nil
}

// GetPlatformRevenue retrieves platform revenue over time
func (r *repository) GetPlatformRevenue(ctx context.Context, filter *DateRangeFilter) (*RsPlatformRevenue, error) {
	from, to := sharedAnalytics.ParseDateRange(filter.From, filter.To, sharedAnalytics.DefaultDaysLong)
	bucket := sharedAnalytics.ParseBucket(filter.Bucket, sharedAnalytics.BucketMonth)
	dateTrunc, dateFormat := sharedAnalytics.GetBucketConfig(bucket)

	rows, err := r.db.QueryxContext(ctx, buildRevenueQuery(), dateTrunc, from, to)
	if err != nil {
		return nil, fmt.Errorf("get platform revenue: %w", err)
	}
	defer rows.Close()

	series, err := scanRevenue(rows, dateFormat)
	if err != nil {
		return nil, fmt.Errorf("scan revenue: %w", err)
	}

	return &RsPlatformRevenue{
		Meta:   RevenueMeta{From: from, To: to, Bucket: bucket, Currency: sharedAnalytics.DefaultCurrency},
		Series: series,
	}, nil
}

// ListSubscriptionRecords retrieves filtered subscription records
func (r *repository) ListSubscriptionRecords(ctx context.Context, filter *SubscriptionRecordFilter) ([]*RsSubscriptionRecord, int, error) {
	// Defensive validation at repository level
	if err := validateSubscriptionRecordFilterAtRepo(filter); err != nil {
		return nil, 0, fmt.Errorf("invalid filter: %w", err)
	}

	limit, offset := sharedAnalytics.ParsePaginationParams(filter.Limit, filter.Offset)
	sortBy, orderBy := sharedAnalytics.ParseSortParams(filter.SortBy, filter.OrderBy, "created_at", "DESC")

	// Validate sort parameters to prevent SQL injection using shared function
	validSortFields := []string{"created_at", "start_date", "end_date", "status"}
	validatedSortBy, err := sharedAnalytics.ValidateSortField(sortBy, validSortFields)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid sort field: %w", err)
	}

	validatedOrderBy, err := sharedAnalytics.ValidateOrderBy(orderBy)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid order: %w", err)
	}

	baseQuery := `
		FROM tbl_practitioner_subscription ps
		JOIN tbl_subscription s ON ps.subscription_id = s.id
		JOIN tbl_practitioner p ON ps.practitioner_id = p.id
		JOIN tbl_user u ON p.user_id = u.id
		WHERE ps.deleted_at IS NULL
	`

	conditions, args, argCount, err := buildFilterConditions(filter)
	if err != nil {
		return nil, 0, fmt.Errorf("build filter conditions: %w", err)
	}

	if len(conditions) > 0 {
		baseQuery += " AND " + strings.Join(conditions, " AND ")
	}

	// Run count and list queries in parallel
	type countResult struct {
		total int
		err   error
	}
	type listResult struct {
		rows []*RsSubscriptionRecord
		err  error
	}

	countCh := make(chan countResult, 1)
	listCh := make(chan listResult, 1)

	go func() {
		var total int
		err := r.db.GetContext(ctx, &total, "SELECT COUNT(*) "+baseQuery, args...)
		countCh <- countResult{total, err}
	}()

	go func() {
		selectQuery := `
			SELECT 
				ps.id as subscription_id,
				p.id as practitioner_id,
				CONCAT(u.first_name, ' ', u.last_name) as practitioner_name,
				u.email as practitioner_email,
				s.id as plan_id,
				s.name as plan_name,
				ps.status,
				s.price as amount,
				'AUD' as currency,
				ps.start_date,
				ps.end_date,
				ps.created_at
		` + baseQuery + fmt.Sprintf(" ORDER BY ps.%s %s LIMIT $%d OFFSET $%d", validatedSortBy, validatedOrderBy, argCount, argCount+1)

		listArgs := append(args, limit, offset)
		rows, err := r.db.QueryxContext(ctx, selectQuery, listArgs...)
		if err != nil {
			listCh <- listResult{nil, fmt.Errorf("list subscriptions: %w", err)}
			return
		}
		defer rows.Close()

		var results []*RsSubscriptionRecord
		for rows.Next() {
			var record RsSubscriptionRecord
			err := rows.Scan(
				&record.SubscriptionID,
				&record.PractitionerID,
				&record.PractitionerName,
				&record.PractitionerEmail,
				&record.PlanID,
				&record.PlanName,
				&record.Status,
				&record.Amount,
				&record.Currency,
				&record.StartDate,
				&record.EndDate,
				&record.CreatedAt,
			)
			if err != nil {
				listCh <- listResult{nil, fmt.Errorf("scan subscription: %w", err)}
				return
			}
			results = append(results, &record)
		}

		// Check for iteration errors
		if err := rows.Err(); err != nil {
			listCh <- listResult{nil, fmt.Errorf("iterate rows: %w", err)}
			return
		}

		listCh <- listResult{results, nil}
	}()

	c := <-countCh
	if c.err != nil {
		return nil, 0, fmt.Errorf("count subscriptions: %w", c.err)
	}
	l := <-listCh
	if l.err != nil {
		return nil, 0, l.err
	}

	return l.rows, c.total, nil
}

// validateSubscriptionRecordFilterAtRepo performs defensive validation at repository level
func validateSubscriptionRecordFilterAtRepo(filter *SubscriptionRecordFilter) error {
	if filter == nil {
		return nil
	}

	// Validate date format and range
	var fromDate, toDate time.Time
	var err error

	if filter.From != nil && *filter.From != "" {
		fromDate, err = time.Parse("2006-01-02", *filter.From)
		if err != nil {
			return fmt.Errorf("invalid from date format: %w", err)
		}
	}

	if filter.To != nil && *filter.To != "" {
		toDate, err = time.Parse("2006-01-02", *filter.To)
		if err != nil {
			return fmt.Errorf("invalid to date format: %w", err)
		}
	}

	// Validate date range
	if filter.From != nil && *filter.From != "" && filter.To != nil && *filter.To != "" {
		if fromDate.After(toDate) {
			return fmt.Errorf("from date must be before or equal to to date")
		}

		// Check for unreasonably large ranges (> 2 years)
		if toDate.Sub(fromDate) > 730*24*time.Hour {
			return fmt.Errorf("date range cannot exceed 2 years")
		}
	}

	// Validate dates are not too far in the future
	// Allow dates up to end of current month to handle ongoing month queries
	now := time.Now().UTC()
	endOfMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)

	if filter.From != nil && *filter.From != "" && fromDate.After(endOfMonth) {
		return fmt.Errorf("from date cannot be in the future")
	}

	if filter.To != nil && *filter.To != "" && toDate.After(endOfMonth) {
		return fmt.Errorf("to date cannot be in the future")
	}

	// Validate status if provided using shared function
	if filter.Status != nil && *filter.Status != "" {
		if err := sharedAnalytics.ValidateSubscriptionStatus(*filter.Status); err != nil {
			return err
		}
	}

	// Validate search term length
	if filter.Search != nil && *filter.Search != "" {
		if len(*filter.Search) > 100 {
			return fmt.Errorf("search term too long (max 100 characters)")
		}
	}

	// Validate plan name length
	if filter.PlanName != nil && *filter.PlanName != "" {
		if len(*filter.PlanName) > 100 {
			return fmt.Errorf("plan_name filter too long (max 100 characters)")
		}
	}

	return nil
}

// GetPlanDistribution retrieves plan distribution with historical data
func (r *repository) GetPlanDistribution(ctx context.Context, filter *DateRangeFilter) (*RsPlanDistribution, error) {
	// Handle nil filter
	if filter == nil {
		filter = &DateRangeFilter{}
	}

	from, to := sharedAnalytics.ParseDateRange(filter.From, filter.To, sharedAnalytics.DefaultDaysLong)
	bucket := sharedAnalytics.ParseBucket(filter.Bucket, sharedAnalytics.BucketMonth)
	dateTrunc, dateFormat := sharedAnalytics.GetBucketConfig(bucket)

	result := RsPlanDistribution{
		Meta:  RevenueMeta{From: from, To: to, Bucket: bucket, Currency: sharedAnalytics.DefaultCurrency},
		Plans: []PlanDistribution{}, // Initialize empty slice to avoid null in JSON
	}

	// Single query: plan counts + timeseries in one pass
	query := `
		WITH plan_totals AS (
			SELECT 
				s.id AS plan_id,
				s.name AS plan_name,
				COUNT(*) AS total_subscriptions,
				COUNT(*) FILTER (WHERE ps.status = 'ACTIVE') AS active_subscriptions
			FROM tbl_subscription s
			LEFT JOIN tbl_practitioner_subscription ps 
				ON ps.subscription_id = s.id 
				AND ps.deleted_at IS NULL
			WHERE s.deleted_at IS NULL
			GROUP BY s.id, s.name
		),
		time_series AS (
			SELECT
				s.id AS plan_id,
				DATE_TRUNC($1, ps.created_at) AS ts,
				SUM(s.price / NULLIF(s.duration_days, 0) * 30) AS revenue,
				COUNT(*) AS new_subscriptions,
				COUNT(*) FILTER (WHERE ps.status = 'ACTIVE') AS active_in_bucket
			FROM tbl_practitioner_subscription ps
			JOIN tbl_subscription s ON ps.subscription_id = s.id
			WHERE ps.created_at BETWEEN $2 AND $3
			  AND ps.created_at IS NOT NULL
			  AND ps.deleted_at IS NULL
			  AND s.deleted_at IS NULL
			GROUP BY s.id, DATE_TRUNC($1, ps.created_at)
		)
		SELECT 
			pt.plan_id,
			pt.plan_name,
			pt.total_subscriptions,
			pt.active_subscriptions,
			COALESCE(ts.ts, NULL) AS ts,
			COALESCE(ts.revenue, 0) AS revenue,
			COALESCE(ts.new_subscriptions, 0) AS new_subscriptions,
			COALESCE(ts.active_in_bucket, 0) AS active_in_bucket
		FROM plan_totals pt
		LEFT JOIN time_series ts ON ts.plan_id = pt.plan_id
		WHERE ts.ts IS NOT NULL OR pt.total_subscriptions > 0
		ORDER BY pt.plan_id, ts.ts
	`

	rows, err := r.db.QueryxContext(ctx, query, dateTrunc, from, to)
	if err != nil {
		return nil, fmt.Errorf("get plan distribution: %w", err)
	}
	defer rows.Close()

	// Track plans and their data
	planMap := make(map[int]*PlanDistribution)
	planOrder := []int{}

	for rows.Next() {
		var planID int
		var planName string
		var total, active int
		var ts *time.Time
		var revenue float64
		var newSubs, activeBucket int

		if err := rows.Scan(&planID, &planName, &total, &active, &ts, &revenue, &newSubs, &activeBucket); err != nil {
			return nil, fmt.Errorf("scan plan distribution: %w", err)
		}

		// Initialize plan if not seen before
		if _, exists := planMap[planID]; !exists {
			planMap[planID] = &PlanDistribution{
				PlanID:   fmt.Sprintf("%d", planID),
				PlanName: planName,
				Counts: PlanCounts{
					TotalSubscriptions:  total,
					ActiveSubscriptions: active,
				},
				Series: []PlanDistributionPoint{},
			}
			planOrder = append(planOrder, planID)
		}

		// Add time-series point if timestamp exists
		if ts != nil {
			planMap[planID].Series = append(planMap[planID].Series, PlanDistributionPoint{
				Timestamp:           ts.Format(dateFormat),
				Revenue:             revenue,
				NewSubscriptions:    newSubs,
				ActiveSubscriptions: activeBucket,
			})
		}
	}

	// Check for iteration errors
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plan distribution rows: %w", err)
	}

	// Build final result in order
	for _, planID := range planOrder {
		result.Plans = append(result.Plans, *planMap[planID])
	}

	return &result, nil
}

// Query builder helper functions

// buildTimeseriesQuery creates a reusable timeseries query
func buildTimeseriesQuery(table, groupField, dateField string, entityFilter string) string {
	return fmt.Sprintf(`
		SELECT 
			%s,
			DATE_TRUNC($1, %s) as ts,
			COUNT(*) as count
		FROM %s
		WHERE %s BETWEEN $2 AND $3
			%s
		GROUP BY %s, DATE_TRUNC($1, %s)
		ORDER BY %s, ts
	`, groupField, dateField, table, dateField, entityFilter, groupField, dateField, groupField)
}

// scanTimeseries scans timeseries data into a map
func scanTimeseries(rows *sqlx.Rows, dateFormat string) (map[string][]TimePoint, error) {
	seriesMap := make(map[string][]TimePoint)

	for rows.Next() {
		var key string
		var ts time.Time
		var count int
		if err := rows.Scan(&key, &ts, &count); err != nil {
			return nil, err
		}

		seriesMap[key] = append(seriesMap[key], TimePoint{
			Timestamp: ts.Format(dateFormat),
			Count:     count,
		})
	}

	return seriesMap, nil
}

// buildRevenueQuery creates a reusable revenue query
func buildRevenueQuery() string {
	return `
		SELECT 
			DATE_TRUNC($1, ps.created_at) as ts,
			SUM(s.price) as revenue
		FROM tbl_practitioner_subscription ps
		JOIN tbl_subscription s ON ps.subscription_id = s.id
		WHERE ps.created_at BETWEEN $2 AND $3
			AND ps.deleted_at IS NULL
		GROUP BY DATE_TRUNC($1, ps.created_at)
		ORDER BY ts
	`
}

// scanRevenue scans revenue data
func scanRevenue(rows *sqlx.Rows, dateFormat string) ([]RevenuePoint, error) {
	var points []RevenuePoint

	for rows.Next() {
		var ts time.Time
		var revenue float64
		if err := rows.Scan(&ts, &revenue); err != nil {
			return nil, err
		}
		points = append(points, RevenuePoint{
			Timestamp: ts.Format(dateFormat),
			Revenue:   revenue,
		})
	}

	return points, nil
}

// buildFilterConditions builds WHERE conditions from filter
func buildFilterConditions(filter *SubscriptionRecordFilter) ([]string, []interface{}, int, error) {
	var conditions []string
	var args []interface{}
	argCount := 1

	if filter == nil {
		return conditions, args, argCount, nil
	}

	// Search filter with LIKE pattern escaping
	if filter.Search != nil && *filter.Search != "" {
		searchTerm := strings.TrimSpace(*filter.Search)
		if len(searchTerm) > 100 {
			return nil, nil, 0, fmt.Errorf("search term too long (max 100 characters)")
		}
		// Escape LIKE wildcards using shared function
		searchTerm = sharedAnalytics.EscapeLikePattern(searchTerm)
		// Use separate placeholders for better query optimization
		conditions = append(conditions, fmt.Sprintf("(u.email ILIKE $%d OR u.first_name ILIKE $%d OR u.last_name ILIKE $%d)", argCount, argCount+1, argCount+2))
		pattern := "%" + searchTerm + "%"
		args = append(args, pattern, pattern, pattern)
		argCount += 3
	}

	// Plan name filter with validation and escaping
	if filter.PlanName != nil && *filter.PlanName != "" {
		planName := strings.TrimSpace(*filter.PlanName)
		if len(planName) > 100 {
			return nil, nil, 0, fmt.Errorf("plan_name filter too long (max 100 characters)")
		}
		// Escape LIKE wildcards using shared function
		planName = sharedAnalytics.EscapeLikePattern(planName)
		conditions = append(conditions, fmt.Sprintf("s.name ILIKE $%d", argCount))
		args = append(args, "%"+planName+"%")
		argCount++
	}

	// Status filter with enum validation using shared function
	if filter.Status != nil && *filter.Status != "" {
		if err := sharedAnalytics.ValidateSubscriptionStatus(*filter.Status); err != nil {
			return nil, nil, 0, err
		}
		conditions = append(conditions, fmt.Sprintf("ps.status = $%d", argCount))
		args = append(args, strings.ToUpper(*filter.Status))
		argCount++
	}

	// Date filters with proper timestamp handling
	if filter.From != nil && *filter.From != "" {
		fromDate, err := time.Parse("2006-01-02", *filter.From)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("invalid from date format: %w", err)
		}
		// Start of day in UTC
		fromTimestamp := time.Date(fromDate.Year(), fromDate.Month(), fromDate.Day(), 0, 0, 0, 0, time.UTC)
		conditions = append(conditions, fmt.Sprintf("ps.created_at >= $%d", argCount))
		args = append(args, fromTimestamp)
		argCount++
	}

	if filter.To != nil && *filter.To != "" {
		toDate, err := time.Parse("2006-01-02", *filter.To)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("invalid to date format: %w", err)
		}
		// End of day in UTC (start of next day)
		toTimestamp := time.Date(toDate.Year(), toDate.Month(), toDate.Day(), 23, 59, 59, 999999999, time.UTC)
		conditions = append(conditions, fmt.Sprintf("ps.created_at <= $%d", argCount))
		args = append(args, toTimestamp)
		argCount++
	}

	return conditions, args, argCount, nil
}

// Helper to round to 2 decimal places
func roundToTwo(n float64) float64 {
	return math.Round(n*100) / 100
}
