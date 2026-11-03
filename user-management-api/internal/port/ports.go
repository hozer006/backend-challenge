// Package port defines the interfaces (ports) between the application core
// and the outside world. Adapters implement these; services depend only on them.
package port

import (
	"context"

	"github.com/hozer006/backend-challenge/user-management-api/internal/domain"
)

// UserRepository persists and retrieves users.
// Implementations must return domain.ErrUserNotFound when a user does not exist
// and domain.ErrEmailAlreadyExists when the unique email constraint is violated.
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, id string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	List(ctx context.Context) ([]*domain.User, error)
	Update(ctx context.Context, id string, update UserUpdate) (*domain.User, error)
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context) (int64, error)
}

// UserUpdate holds the optional fields that can be changed on a user.
// A nil field means "leave unchanged".
type UserUpdate struct {
	Name  *string
	Email *string
}

// PasswordHasher hashes and verifies passwords.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

// TokenService issues and validates authentication tokens.
type TokenService interface {
	Generate(userID string) (token string, err error)
	// Validate returns the user ID (subject) of a valid token,
	// or domain.ErrInvalidToken.
	Validate(token string) (userID string, err error)
}

// UserCounter is the minimal dependency of the background user-count job.
type UserCounter interface {
	Count(ctx context.Context) (int64, error)
}
