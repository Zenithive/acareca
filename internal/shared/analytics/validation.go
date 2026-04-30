package analytics

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// EscapeLikePattern escapes special LIKE/ILIKE wildcard characters
func EscapeLikePattern(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\") // Escape backslash first
	s = strings.ReplaceAll(s, "%", "\\%")   // Escape %
	s = strings.ReplaceAll(s, "_", "\\_")   // Escape _
	return s
}

// ValidateSubscriptionStatus validates status against enum values
func ValidateSubscriptionStatus(status string) error {
	validStatuses := map[string]bool{
		"ACTIVE":    true,
		"PAST_DUE":  true,
		"CANCELLED": true,
		"PAUSED":    true,
		"EXPIRED":   true,
	}

	upperStatus := strings.ToUpper(status)
	if !validStatuses[upperStatus] {
		return fmt.Errorf("invalid status: must be one of ACTIVE, PAST_DUE, CANCELLED, PAUSED, EXPIRED")
	}

	return nil
}

// ValidateSortField validates sort field against whitelist
func ValidateSortField(field string, validFields []string) (string, error) {
	fieldMap := make(map[string]bool)
	for _, f := range validFields {
		fieldMap[f] = true
	}

	if fieldMap[field] {
		return field, nil
	}

	// Return first valid field as default
	if len(validFields) > 0 {
		return validFields[0], fmt.Errorf("invalid sort field: must be one of %s", strings.Join(validFields, ", "))
	}

	return "", fmt.Errorf("invalid sort field: %s", field)
}

// ValidateOrderBy validates order direction
func ValidateOrderBy(order string) (string, error) {
	upperOrder := strings.ToUpper(order)
	if upperOrder == "ASC" || upperOrder == "DESC" {
		return upperOrder, nil
	}

	return "DESC", fmt.Errorf("invalid order: must be ASC or DESC")
}

// ValidateDateRange validates that from date is before to date and neither is in the future
func ValidateDateRange(from, to string) error {
	if from == "" && to == "" {
		return nil // Both optional
	}

	var fromDate, toDate time.Time
	var err error

	if from != "" {
		fromDate, err = time.Parse("2006-01-02", from)
		if err != nil {
			return fmt.Errorf("invalid from date format: %w", err)
		}
	}

	if to != "" {
		toDate, err = time.Parse("2006-01-02", to)
		if err != nil {
			return fmt.Errorf("invalid to date format: %w", err)
		}
	}

	// Validate range if both provided
	if from != "" && to != "" {
		if fromDate.After(toDate) {
			return ErrInvalidDateRange
		}

		// Check for unreasonably large ranges (> 2 years)
		if toDate.Sub(fromDate) > 730*24*time.Hour {
			return errors.New("date range cannot exceed 2 years")
		}
	}

	// Check for future dates (use UTC for consistency)
	// Allow dates up to end of current month to handle ongoing month queries
	now := time.Now().UTC()
	endOfMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)

	if from != "" && fromDate.After(endOfMonth) {
		return ErrFutureDateNotValid
	}

	if to != "" && toDate.After(endOfMonth) {
		return ErrFutureDateNotValid
	}

	// Check for unreasonably old dates (before 2020)
	minDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if from != "" && fromDate.Before(minDate) {
		return errors.New("from date cannot be before 2020-01-01")
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
// Returns the sanitized value instead of modifying the input
func SanitizeSearchTerm(search *string) error {
	if search == nil || *search == "" {
		return nil
	}

	trimmed := strings.TrimSpace(*search)

	if len(trimmed) > 100 {
		return ErrSearchTooLong
	}

	// Update the pointer value with trimmed version
	*search = trimmed

	return nil
}
