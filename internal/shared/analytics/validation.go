package analytics

import (
	"strings"
	"time"
)

// ValidateDateRange validates that from date is before to date and neither is in the future
func ValidateDateRange(from, to string) error {
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

// ValidateBucket validates bucket value
func ValidateBucket(bucket string) error {
	if bucket == "" {
		return nil // Optional parameter
	}

	if !validBuckets[bucket] {
		return ErrInvalidBucket
	}

	return nil
}

// ValidatePagination validates pagination parameters
func ValidatePagination(limit, offset *int) error {
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

// SanitizeSearchTerm sanitizes and validates search term
func SanitizeSearchTerm(search *string) error {
	if search == nil || *search == "" {
		return nil
	}

	*search = strings.TrimSpace(*search)

	if len(*search) > 100 {
		return ErrSearchTooLong
	}

	return nil
}
