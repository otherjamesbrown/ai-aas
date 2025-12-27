package httputil

// ValidationError represents a field-level validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// NewValidationError creates a new validation error for a field.
func NewValidationError(field, message string) ValidationError {
	return ValidationError{
		Field:   field,
		Message: message,
	}
}

// ValidationErrors is a slice of ValidationError with helper methods.
type ValidationErrors []ValidationError

// Add adds a validation error for a field.
func (v *ValidationErrors) Add(field, message string) {
	*v = append(*v, NewValidationError(field, message))
}

// HasErrors returns true if there are any validation errors.
func (v ValidationErrors) HasErrors() bool {
	return len(v) > 0
}

// ForField returns all validation errors for a specific field.
func (v ValidationErrors) ForField(field string) []ValidationError {
	var result []ValidationError
	for _, err := range v {
		if err.Field == field {
			result = append(result, err)
		}
	}
	return result
}
