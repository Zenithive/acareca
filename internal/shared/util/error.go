package util

import "errors"

var (
	ErrAccessDenied       = errors.New("access denied")
	ErrInvalidExpenseType = errors.New("form is not an expense entry")
)
