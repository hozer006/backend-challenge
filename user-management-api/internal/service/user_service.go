// Package service holds the application use cases. It depends only on
// domain types and port interfaces, never on concrete adapters.
package service

import (
	"context"
	"time"

	"github.com/hozer006/backend-challenge/user-management-api/internal/domain"
	"github.com/hozer006/backend-challenge/user-management-api/internal/port"
)

// CreateUserInput is the input for creating or registering a user.
type CreateUserInput struct {
	Name     string
	Email    string
	Password string
}

// UpdateUserInput holds optional changes to a user. Nil means unchanged.
type UpdateUserInput struct {
	Name  *string
	Email *string
}

// UserService implements user management use cases.
type UserService struct {
	repo   port.UserRepository
	hasher port.PasswordHasher
	now    func() time.Time
}

// NewUserService creates a UserService.
func NewUserService(repo port.UserRepository, hasher port.PasswordHasher) *UserService {
	return &UserService{repo: repo, hasher: hasher, now: time.Now}
}

// Create validates input, hashes the password and stores a new user.
func (s *UserService) Create(ctx context.Context, in CreateUserInput) (*domain.User, error) {
	name := domain.NormalizeName(in.Name)
	email := domain.NormalizeEmail(in.Email)

	if err := domain.ValidateName(name); err != nil {
		return nil, err
	}
	if err := domain.ValidateEmail(email); err != nil {
		return nil, err
	}
	if err := domain.ValidatePassword(in.Password); err != nil {
		return nil, err
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		Name:         name,
		Email:        email,
		PasswordHash: hash,
		// MongoDB stores dates with millisecond precision; truncate so the value
		// returned on create matches what later reads return.
		CreatedAt: s.now().UTC().Truncate(time.Millisecond),
	}
	// Email uniqueness is enforced by the repository (unique index),
	// which avoids a check-then-insert race.
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// GetByID returns a single user.
func (s *UserService) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return s.repo.GetByID(ctx, id)
}

// List returns all users.
func (s *UserService) List(ctx context.Context) ([]*domain.User, error) {
	return s.repo.List(ctx)
}

// Update changes a user's name and/or email. At least one field is required.
func (s *UserService) Update(ctx context.Context, id string, in UpdateUserInput) (*domain.User, error) {
	if in.Name == nil && in.Email == nil {
		return nil, domain.NewValidationError("body", "must contain name or email")
	}

	var upd port.UserUpdate
	if in.Name != nil {
		name := domain.NormalizeName(*in.Name)
		if err := domain.ValidateName(name); err != nil {
			return nil, err
		}
		upd.Name = &name
	}
	if in.Email != nil {
		email := domain.NormalizeEmail(*in.Email)
		if err := domain.ValidateEmail(email); err != nil {
			return nil, err
		}
		upd.Email = &email
	}
	return s.repo.Update(ctx, id, upd)
}

// Delete removes a user.
func (s *UserService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// Count returns the total number of users.
func (s *UserService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}
