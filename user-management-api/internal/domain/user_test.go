package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		email string
		ok    bool
	}{
		{"alice@example.com", true},
		{"a.b+tag@sub.example.co.th", true},
		{"", false},
		{"not-an-email", false},
		{"alice@", false},
		{"alice@localhost", false},
		{"Alice <alice@example.com>", false},
		{strings.Repeat("a", 250) + "@x.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if (err == nil) != tt.ok {
				t.Fatalf("ValidateEmail(%q) err = %v, want ok=%v", tt.email, err, tt.ok)
			}
			if err != nil && !errors.Is(err, ErrValidation) {
				t.Fatalf("error should match ErrValidation, got %v", err)
			}
		})
	}
}

func TestValidateName(t *testing.T) {
	if err := ValidateName("Alice"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ValidateName(""); err == nil {
		t.Fatal("expected error for empty name")
	}
	if err := ValidateName(strings.Repeat("ก", MaxNameLength)); err != nil {
		t.Fatalf("100 Thai characters should be valid: %v", err)
	}
	if err := ValidateName(strings.Repeat("a", MaxNameLength+1)); err == nil {
		t.Fatal("expected error for too-long name")
	}
}

func TestValidatePassword(t *testing.T) {
	cases := map[string]bool{
		"":                      false,
		"short":                 false,
		"longenough":            true,
		strings.Repeat("x", 72): true,
		strings.Repeat("x", 73): false,
	}
	for pw, ok := range cases {
		if err := ValidatePassword(pw); (err == nil) != ok {
			t.Errorf("ValidatePassword(len=%d) err = %v, want ok=%v", len(pw), err, ok)
		}
	}
}

func TestNormalizeEmail(t *testing.T) {
	if got := NormalizeEmail("  Alice@Example.COM "); got != "alice@example.com" {
		t.Fatalf("got %q", got)
	}
}

func TestValidationErrorIs(t *testing.T) {
	err := NewValidationError("name", "is required")
	if !errors.Is(err, ErrValidation) {
		t.Fatal("ValidationError should match ErrValidation")
	}
	if err.Error() != "name is required" {
		t.Fatalf("Error() = %q", err.Error())
	}
}
