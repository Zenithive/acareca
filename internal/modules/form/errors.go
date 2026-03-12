package form

import "errors"

// Shared errors for form operations, used by detail handler to return 4xx.
// Field and other subpackages return these so the HTTP layer can map them correctly.
var (
	ErrCoaNotFound             = errors.New("chart of account not found or does not belong to this practice")
	ErrFormNotDraftForFields   = errors.New("only forms in DRAFT status can be edited; publish or archive prevents field changes")
	ErrTooManyFields           = errors.New("max fields per form version exceeded")
	ErrFieldWrongVersion       = errors.New("field does not belong to this form version")
	ErrFieldHasSubmittedEntries = errors.New("cannot delete field: it has submitted entry values")
)
