package analytics

import "time"

// parseDateRange extracts and validates date range from filter
func ParseDateRange(from, to *string, defaultDays int) (string, string) {
	endDate := time.Now().Format("2006-01-02")
	startDate := time.Now().AddDate(0, 0, -defaultDays).Format("2006-01-02")

	if from != nil && *from != "" {
		startDate = *from
	}
	if to != nil && *to != "" {
		endDate = *to
	}

	return startDate, endDate
}

// ParseBucket extracts bucket value with default
func ParseBucket(bucket *string, defaultBucket string) string {
	if bucket != nil && *bucket != "" {
		return *bucket
	}
	return defaultBucket
}

// GetBucketConfig returns date truncation and format based on bucket type
func GetBucketConfig(bucket string) (dateTrunc, dateFormat string) {
	switch bucket {
	case BucketDay:
		return "day", "2006-01-02"
	case BucketWeek:
		return "week", "2006-01-02"
	case BucketMonth:
		return "month", "2006-01"
	default:
		return "month", "2006-01"
	}
}

// ParsePaginationParams extracts pagination parameters with defaults
func ParsePaginationParams(limit, offset *int) (int, int) {
	l, o := DefaultPageSize, 0
	if limit != nil && *limit > 0 && *limit <= MaxPageSize {
		l = *limit
	}
	if offset != nil && *offset >= 0 {
		o = *offset
	}
	return l, o
}

// ParseSortParams extracts sort parameters with defaults
func ParseSortParams(sortBy, orderBy *string, defaultSort, defaultOrder string) (string, string) {
	sort := defaultSort
	order := defaultOrder

	if sortBy != nil && *sortBy != "" {
		sort = *sortBy
	}
	if orderBy != nil && *orderBy != "" {
		order = *orderBy
	}

	return sort, order
}
