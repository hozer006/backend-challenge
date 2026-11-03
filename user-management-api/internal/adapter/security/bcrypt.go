// Package security provides password hashing and JWT token adapters.
package security

import (
	"errors"

	"golang.org/x/crypto/bcrypt"

	"github.com/hozer006/backend-challenge/user-management-api/internal/domain"
)

// BcryptHasher implements port.PasswordHasher using bcrypt.
type BcryptHasher struct {
	cost int
}

// NewBcryptHasher creates a hasher. A cost <= 0 uses bcrypt.DefaultCost.
func NewBcryptHasher(cost int) *BcryptHasher {
	if cost <= 0 {
		cost = bcrypt.DefaultCost
	}
	return &BcryptHasher{cost: cost}
}

// Hash returns the bcrypt hash of password.
func (h *BcryptHasher) Hash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Compare returns domain.ErrInvalidCredentials if password does not match hash.
func (h *BcryptHasher) Compare(hash, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return domain.ErrInvalidCredentials
	}
	return err
}
