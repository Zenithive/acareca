package analytics

import "errors"

// Time range defaults
const (
	DefaultDaysShort = 30
	DefaultDaysLong  = 365
	DefaultPageSize  = 20
	MaxPageSize      = 100
)

// Bucket types
const (
	BucketDay   = "day"
	BucketWeek  = "week"
	BucketMonth = "month"
)

// Currency
const (
	DefaultCurrency = "AUD"
)

// Errors
var (
	ErrInvalidDateRange   = errors.New("from date must be before or equal to to date")
	ErrInvalidBucket      = errors.New("bucket must be one of: day, week, month")
	ErrInvalidPageSize    = errors.New("page size must be between 1 and 100")
	ErrSearchTooLong      = errors.New("search term must be less than 100 characters")
	ErrFutureDateNotValid = errors.New("date cannot be in the future")
)

// Valid bucket values
var validBuckets = map[string]bool{
	BucketDay:   true,
	BucketWeek:  true,
	BucketMonth: true,
}
