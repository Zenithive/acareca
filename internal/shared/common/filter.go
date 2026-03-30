package common

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Operator string

const (
	OpEq    Operator = "eq"
	OpLike  Operator = "like"
	OpIn    Operator = "in"
	OpGt    Operator = "gt"
	OpLt    Operator = "lt"
	OpNotEq Operator = "neq"
)

type Condition struct {
	Field    string
	Operator Operator
	Value    interface{}
}

type Filter struct {
	Search  *string `form:"search"`
	Where   []Condition
	Limit   *int    `form:"limit"`
	Offset  *int    `form:"offset"`
	SortBy  *string `form:"sort_by"`
	OrderBy *string `form:"order_by"`
}

func BuildQuery(base string, f Filter, allowedColumns map[string]string, searchCols []string, count bool) (string, []interface{}) {

	var args []interface{}
	var conditions []string

	for _, c := range f.Where {
		col, ok := allowedColumns[c.Field]
		if !ok {
			continue
		}

		switch c.Operator {
		case OpEq:
			conditions = append(conditions, fmt.Sprintf("%s = ?", col))
			args = append(args, c.Value)

		case OpLike:
			conditions = append(conditions, fmt.Sprintf("%s LIKE ?", col))
			args = append(args, c.Value)

		case OpGt:
			conditions = append(conditions, fmt.Sprintf("%s > ?", col))
			args = append(args, c.Value)

		case OpLt:
			conditions = append(conditions, fmt.Sprintf("%s < ?", col))
			args = append(args, c.Value)

		case OpIn:
			query, inArgs, _ := sqlx.In(fmt.Sprintf("%s IN (?)", col), c.Value)
			conditions = append(conditions, query)
			args = append(args, inArgs...)

		case OpNotEq:
			conditions = append(conditions, fmt.Sprintf("%s != ?", col))
			args = append(args, c.Value)
		}
	}

	// 🔥 Search
	if f.Search != nil && *f.Search != "" && len(searchCols) > 0 {
		var searchParts []string

		for _, col := range searchCols {
			searchParts = append(searchParts, fmt.Sprintf("%s ILIKE ?", col))
			args = append(args, "%"+*f.Search+"%")
		}

		conditions = append(conditions, "("+strings.Join(searchParts, " OR ")+")")
	}

	// 🔥 Apply WHERE
	if len(conditions) > 0 {
		if strings.Contains(strings.ToUpper(base), "WHERE") {
			base += " AND " + strings.Join(conditions, " AND ")
		} else {
			base += " WHERE " + strings.Join(conditions, " AND ")
		}
	}

	// 🔥 COUNT MODE
	if count {
		query := "SELECT COUNT(*) " + base
		return query, args
	}

	// 🔥 NORMAL MODE
	query := base

	var sortBy string
	var OrderBy string

	if f.SortBy != nil {
		sortBy = *f.SortBy
	} else {
		sortBy = "created_at"
	}

	if f.OrderBy != nil {
		OrderBy = *f.OrderBy
	} else {
		OrderBy = "DESC"
	}

	// Sorting
	if col, ok := allowedColumns[sortBy]; ok {
		order := "ASC"
		if strings.ToUpper(OrderBy) == "DESC" {
			order = "DESC"
		}
		query += fmt.Sprintf(" ORDER BY %s %s", col, order)
	}

	// Pagination
	if f.Limit != nil {
		query += " LIMIT ?"
		args = append(args, f.Limit)
	}

	if f.Offset != nil {
		query += " OFFSET ?"
		args = append(args, f.Offset)
	}

	return query, args
}

func NewFilter(search *string, filters map[string]interface{}, operators map[string]Operator, limit, offset *int, sortBy, orderBy *string) Filter {

	var where []Condition

	for field, value := range filters {

		if value == nil {
			continue
		}

		switch v := value.(type) {
		case []string:
			if len(v) == 0 {
				continue
			}
		case []int:
			if len(v) == 0 {
				continue
			}
		case []uuid.UUID:
			if len(v) == 0 {
				continue
			}
		}

		op := OpEq

		if operators != nil {
			if customOp, ok := operators[field]; ok {
				op = customOp
			}
		}

		if op == OpEq {
			switch value.(type) {
			case []string, []int, []int64, []uuid.UUID:
				op = OpIn
			}
		}

		where = append(where, Condition{
			Field:    field,
			Operator: op,
			Value:    value,
		})
	}

	// 🔥 safe defaults
	l, o := 10, 0
	if limit != nil && *limit > 0 && *limit <= 100 {
		l = *limit
	}
	if offset != nil && *offset >= 0 {
		o = *offset
	}

	return Filter{
		Search:  search,
		Where:   where,
		Limit:   &l,
		Offset:  &o,
		SortBy:  sortBy,
		OrderBy: orderBy,
	}
}
