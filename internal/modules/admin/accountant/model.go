package accountant

import (
	"database/sql"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

// dbAccountantWithPractitioners represents internal database model
type dbAccountantWithPractitioners struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	Email          string
	FirstName      string
	LastName       string
	Phone          *string
	PractitionerID sql.NullString
	PracEmail      sql.NullString
	PracFirstName  sql.NullString
	PracLastName   sql.NullString
	PracPhone      sql.NullString
}

// RsAccountantWithPractitioners represents the API response
type RsAccountantWithPractitioners struct {
	ID            uuid.UUID          `json:"id"`
	Name          string             `json:"name"`
	Email         string             `json:"email"`
	Phone         *string            `json:"phone"`
	Practitioners []PractitionerInfo `json:"practitioners"`
}

// PractitionerInfo represents practitioner details in response
type PractitionerInfo struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Email string    `json:"email"`
	Phone *string   `json:"phone"`
}

// Filter for listing accountants
type Filter struct {
	Email *string `form:"email" binding:"omitempty,email"`
	Name  *string `form:"name"`
	Phone *string `form:"phone"`
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

	return common.NewFilter(filter.Search, filters, operators, filter.Limit, filter.Offset, filter.SortBy, filter.OrderBy)
}
