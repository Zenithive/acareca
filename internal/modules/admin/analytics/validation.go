package analytics

import (
	"strings"
	"time"
)

// validateDateRange validates that from date is before to date and neither is in the future
func validateDateRange(from, to string) error {
	if from == "" || to == "" {
		return nil // Optional parameters
	}

	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		return err
	}

	toDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		return err
	}

	if fromDate.After(toDate) {
		return ErrInvalidDateRange
	}

	now := time.Now()
	if fromDate.After(now) || toDate.After(now) {
		return ErrFutureDateNotValid
	}

	return nil
}

// validateBucket validates bucket value
func validateBucket(bucket string) error {
	if bucket == "" {
		return nil // Optional parameter
	}

	if !validBuckets[bucket] {
		return ErrInvalidBucket
	}

	return nil
}

// validatePagination validates pagination parameters
func validatePagination(limit, offset *int) error {
	if limit != nil {
		if *limit < 1 || *limit > MaxPageSize {
			return ErrInvalidPageSize
		}
	}

	if offset != nil && *offset < 0 {
		return ErrInvalidPageSize
	}

	return nil
}

// sanitizeSearchTerm sanitizes and validates search term
func sanitizeSearchTerm(search *string) error {
	if search == nil || *search == "" {
		return nil
	}

	*search = strings.TrimSpace(*search)

	if len(*search) > 100 {
		return ErrSearchTooLong
	}

	return nil
}

// validateDateRangeFilter validates DateRangeFilter
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

	if err := validateDateRange(from, to); err != nil {
		return err
	}

	var bucket string
	if filter.Bucket != nil {
		bucket = *filter.Bucket
	}

	return validateBucket(bucket)
}

// validateResourceAnalyticsFilter validates ResourceAnalyticsFilter
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

	return validateDateRange(from, to)
}

// validateSubscriptionRecordFilter validates SubscriptionRecordFilter
func validateSubscriptionRecordFilter(filter *SubscriptionRecordFilter) error {
	if filter == nil {
		return nil
	}

	if err := sanitizeSearchTerm(filter.Search); err != nil {
		return err
	}

	if err := validatePagination(filter.Limit, filter.Offset); err != nil {
		return err
	}

	var from, to string
	if filter.From != nil {
		from = *filter.From
	}
	if filter.To != nil {
		to = *filter.To
	}

	return validateDateRange(from, to)
}
