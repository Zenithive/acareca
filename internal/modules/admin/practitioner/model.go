package practitioner

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

// dbPractitionerWithSubscription represents internal database model
type dbPractitionerWithSubscription struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Verified  bool
	CreatedAt time.Time
	Email     string
	FirstName string
	LastName  string
	Phone     *string
	SubID     *int
	SubName   *string
	StartDate *time.Time
	EndDate   *time.Time
}

// RsPractitionerWithSubscription represents the API response
type RsPractitionerWithSubscription struct {
	ID                 uuid.UUID               `json:"id"`
	Name               string                  `json:"name"`
	Email              string                  `json:"email"`
	Phone              *string                 `json:"phone"`
	ActiveSubscription *ActiveSubscriptionInfo `json:"active_subscription,omitempty"`
}

// ActiveSubscriptionInfo represents subscription details in response
type ActiveSubscriptionInfo struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
}

// MapToResponse converts database model to API response
func (db *dbPractitionerWithSubscription) MapToResponse() *RsPractitionerWithSubscription {
	rs := &RsPractitionerWithSubscription{
		ID:    db.UserID,
		Name:  db.FirstName + " " + db.LastName,
		Email: db.Email,
		Phone: db.Phone,
	}

	if db.hasActiveSubscription() {
		rs.ActiveSubscription = db.mapSubscriptionInfo()
	}

	return rs
}

// hasActiveSubscription checks if practitioner has active subscription
func (db *dbPractitionerWithSubscription) hasActiveSubscription() bool {
	return db.SubID != nil && db.SubName != nil && db.StartDate != nil && db.EndDate != nil
}

// mapSubscriptionInfo maps subscription information
func (db *dbPractitionerWithSubscription) mapSubscriptionInfo() *ActiveSubscriptionInfo {
	return &ActiveSubscriptionInfo{
		ID:        *db.SubID,
		Name:      *db.SubName,
		StartDate: *db.StartDate,
		EndDate:   *db.EndDate,
	}
}

// Filter for listing practitioners with reusable and clean filters
type Filter struct {
	Email                 *string `form:"email" binding:"omitempty,email"`
	Name                  *string `form:"name"`
	Phone                 *string `form:"phone"`
	HasActiveSubscription *bool   `form:"has_active_subscription"`
	SubscriptionName      *string `form:"subscription_name"`
	common.Filter
}

func (filter *Filter) MapToFilter() common.Filter {
	filters := map[string]interface{}{}
	operators := map[string]common.Operator{}

	// Email filter - exact match
	if filter.Email != nil && *filter.Email != "" {
		filters["u.email"] = *filter.Email
	}

	// Name filter - partial match on full name
	if filter.Name != nil && *filter.Name != "" {
		filters["CONCAT(u.first_name, ' ', u.last_name)"] = "%" + *filter.Name + "%"
		operators["CONCAT(u.first_name, ' ', u.last_name)"] = common.OpLike
	}

	// Phone filter - partial match
	if filter.Phone != nil && *filter.Phone != "" {
		filters["u.phone"] = "%" + *filter.Phone + "%"
		operators["u.phone"] = common.OpLike
	}

	// Subscription name filter - partial match
	if filter.SubscriptionName != nil && *filter.SubscriptionName != "" {
		filters["s.name"] = "%" + *filter.SubscriptionName + "%"
		operators["s.name"] = common.OpLike
	}

	return common.NewFilter(filter.Search, filters, operators, filter.Limit, filter.Offset, filter.SortBy, filter.OrderBy)
}
