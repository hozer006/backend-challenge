package domain

import "errors"

// Sentinel errors returned by the domain and application layers.
// Adapters translate these into transport-specific responses (HTTP status, gRPC code).
var (
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrValidation         = errors.New("validation failed")
)

// ValidationError describes an invalid input field.
// errors.Is(err, ErrValidation) reports true for any ValidationError.
type ValidationError struct {
	Field   string
	Message string
}

// NewValidationError builds a ValidationError for a field.
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}

func (e *ValidationError) Error() string {
	return e.Field + " " + e.Message
}

// Is makes errors.Is(err, ErrValidation) match any ValidationError.
func (e *ValidationError) Is(target error) bool {
	return target == ErrValidation
}
