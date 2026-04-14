package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type Service interface {
	GetUserGrowth(ctx context.Context, filter *Filter) (*RsUserGrowth, error)
	GetSubscriptionMetrics(ctx context.Context) (*RsSubscriptionMetrics, error)
	GetActiveUsers(ctx context.Context, filter *Filter) (*RsActiveUsers, error)
	GetPractitionerDetails(ctx context.Context, practitionerID uuid.UUID) (*RsPractitionerDetail, error)
	ListPractitionersWithDetails(ctx context.Context, filter *PractitionerFilter) (*util.RsList, error)

	// Dashboard APIs
	GetPractitionerOverview(ctx context.Context) (*RsPractitionerOverview, error)
	GetResourceAnalytics(ctx context.Context, filter *ResourceAnalyticsFilter) (*RsResourceAnalytics, error)
	GetAccountantOverview(ctx context.Context) (*RsAccountantOverview, error)
	GetResourceAccessTimeseries(ctx context.Context, filter *DateRangeFilter) (*RsResourceAccessTimeseries, error)
	GetPlatformRevenue(ctx context.Context, filter *DateRangeFilter) (*RsPlatformRevenue, error)
	ListSubscriptionRecords(ctx context.Context, filter *SubscriptionRecordFilter) (*util.RsList, error)
	GetPlanDistribution(ctx context.Context, filter *DateRangeFilter) (*RsPlanDistribution, error)
	GetBillingDashboard(ctx context.Context, filter *DateRangeFilter, recordFilter *SubscriptionRecordFilter) (*RsBillingDashboard, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetUserGrowth(ctx context.Context, filter *Filter) (*RsUserGrowth, error) {
	startDate, endDate := s.getDateRange(filter)
	return s.repo.GetUserGrowth(ctx, startDate, endDate)
}

func (s *service) GetSubscriptionMetrics(ctx context.Context) (*RsSubscriptionMetrics, error) {
	return s.repo.GetSubscriptionMetrics(ctx)
}

func (s *service) GetActiveUsers(ctx context.Context, filter *Filter) (*RsActiveUsers, error) {
	startDate, endDate := s.getDateRange(filter)
	return s.repo.GetActiveUsers(ctx, startDate, endDate)
}

func (s *service) GetPractitionerDetails(ctx context.Context, practitionerID uuid.UUID) (*RsPractitionerDetail, error) {
	return s.repo.GetPractitionerDetails(ctx, practitionerID)
}

func (s *service) ListPractitionersWithDetails(ctx context.Context, filter *PractitionerFilter) (*util.RsList, error) {
	results, total, err := s.repo.ListPractitionersWithDetails(ctx, filter)
	if err != nil {
		return nil, err
	}

	offset := 0
	limit := 10
	if filter.Offset != nil {
		offset = *filter.Offset
	}
	if filter.Limit != nil {
		limit = *filter.Limit
	}

	var rsList util.RsList
	rsList.MapToList(results, total, offset, limit)
	return &rsList, nil
}

// getDateRange returns start and end dates based on filter or defaults to last 30 days
func (s *service) getDateRange(filter *Filter) (time.Time, time.Time) {
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30) // Default: last 30 days

	if filter != nil {
		if filter.StartDate != nil {
			startDate = *filter.StartDate
		}
		if filter.EndDate != nil {
			endDate = *filter.EndDate
		}
	}

	return startDate, endDate
}

// Dashboard Service Methods

func (s *service) GetPractitionerOverview(ctx context.Context) (*RsPractitionerOverview, error) {
	return s.repo.GetPractitionerOverview(ctx)
}

func (s *service) GetResourceAnalytics(ctx context.Context, filter *ResourceAnalyticsFilter) (*RsResourceAnalytics, error) {
	return s.repo.GetResourceAnalytics(ctx, filter)
}

func (s *service) GetAccountantOverview(ctx context.Context) (*RsAccountantOverview, error) {
	return s.repo.GetAccountantOverview(ctx)
}

func (s *service) GetResourceAccessTimeseries(ctx context.Context, filter *DateRangeFilter) (*RsResourceAccessTimeseries, error) {
	return s.repo.GetResourceAccessTimeseries(ctx, filter)
}

func (s *service) GetPlatformRevenue(ctx context.Context, filter *DateRangeFilter) (*RsPlatformRevenue, error) {
	return s.repo.GetPlatformRevenue(ctx, filter)
}

func (s *service) ListSubscriptionRecords(ctx context.Context, filter *SubscriptionRecordFilter) (*util.RsList, error) {
	results, total, err := s.repo.ListSubscriptionRecords(ctx, filter)
	if err != nil {
		return nil, err
	}

	offset := 0
	limit := 20
	if filter != nil {
		if filter.Offset != nil {
			offset = *filter.Offset
		}
		if filter.Limit != nil {
			limit = *filter.Limit
		}
	}

	var rsList util.RsList
	rsList.MapToList(results, total, offset, limit)
	return &rsList, nil
}

func (s *service) GetPlanDistribution(ctx context.Context, filter *DateRangeFilter) (*RsPlanDistribution, error) {
	return s.repo.GetPlanDistribution(ctx, filter)
}

func (s *service) GetBillingDashboard(ctx context.Context, filter *DateRangeFilter, recordFilter *SubscriptionRecordFilter) (*RsBillingDashboard, error) {
	type overviewResult struct {
		data *RsSubscriptionMetrics
		err  error
	}
	type recordsResult struct {
		data  []*RsSubscriptionRecord
		total int
		err   error
	}
	type distResult struct {
		data *RsPlanDistribution
		err  error
	}

	overviewCh := make(chan overviewResult, 1)
	recordsCh := make(chan recordsResult, 1)
	distCh := make(chan distResult, 1)

	go func() {
		d, err := s.repo.GetSubscriptionMetrics(ctx)
		overviewCh <- overviewResult{d, err}
	}()
	go func() {
		d, total, err := s.repo.ListSubscriptionRecords(ctx, recordFilter)
		recordsCh <- recordsResult{d, total, err}
	}()
	go func() {
		d, err := s.repo.GetPlanDistribution(ctx, filter)
		distCh <- distResult{d, err}
	}()

	o := <-overviewCh
	if o.err != nil {
		return nil, fmt.Errorf("billing overview: %w", o.err)
	}
	r := <-recordsCh
	if r.err != nil {
		return nil, fmt.Errorf("subscription records: %w", r.err)
	}
	d := <-distCh
	if d.err != nil {
		return nil, fmt.Errorf("plan distribution: %w", d.err)
	}

	return &RsBillingDashboard{
		Overview:         o.data,
		Records:          r.data,
		RecordsTotal:     r.total,
		PlanDistribution: d.data,
	}, nil
}
