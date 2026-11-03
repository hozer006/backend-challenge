// Package domain contains the core business entities and rules.
// It has no dependencies on frameworks, databases or transport layers.
package domain

import (
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

// User is the core user entity.
type User struct {
	ID           string
	Name         string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

const (
	MinPasswordLength = 8
	MaxPasswordLength = 72 // bcrypt only uses the first 72 bytes
	MaxNameLength     = 100
	MaxEmailLength    = 254
)

// NormalizeEmail lowercases and trims an email so uniqueness is case-insensitive.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// NormalizeName trims surrounding whitespace from a name.
func NormalizeName(name string) string {
	return strings.TrimSpace(name)
}

// ValidateName checks that a (normalized) name is present and not too long.
func ValidateName(name string) error {
	if name == "" {
		return NewValidationError("name", "is required")
	}
	if utf8.RuneCountInString(name) > MaxNameLength {
		return NewValidationError("name", "must be at most 100 characters")
	}
	return nil
}

// ValidateEmail checks that a (normalized) email is present and well-formed.
func ValidateEmail(email string) error {
	if email == "" {
		return NewValidationError("email", "is required")
	}
	if len(email) > MaxEmailLength {
		return NewValidationError("email", "is too long")
	}
	addr, err := mail.ParseAddress(email)
	// Reject display-name forms like "Bob <bob@x.com>": the parsed address must equal the input.
	if err != nil || addr.Address != email || !strings.Contains(email[strings.LastIndex(email, "@")+1:], ".") {
		return NewValidationError("email", "is not a valid email address")
	}
	return nil
}

// ValidatePassword checks password length rules.
func ValidatePassword(password string) error {
	if password == "" {
		return NewValidationError("password", "is required")
	}
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return NewValidationError("password", "must be at least 8 characters")
	}
	if len(password) > MaxPasswordLength {
		return NewValidationError("password", "must be at most 72 bytes")
	}
	return nil
}
