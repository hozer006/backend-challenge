package service

import (
	"context"
	"errors"

	"github.com/hozer006/backend-challenge/user-management-api/internal/domain"
	"github.com/hozer006/backend-challenge/user-management-api/internal/port"
)

// AuthService implements registration and login.
type AuthService struct {
	users  *UserService
	repo   port.UserRepository
	hasher port.PasswordHasher
	tokens port.TokenService
}

// NewAuthService creates an AuthService.
func NewAuthService(users *UserService, repo port.UserRepository, hasher port.PasswordHasher, tokens port.TokenService) *AuthService {
	return &AuthService{users: users, repo: repo, hasher: hasher, tokens: tokens}
}

// Register creates a new user account.
func (s *AuthService) Register(ctx context.Context, in CreateUserInput) (*domain.User, error) {
	return s.users.Create(ctx, in)
}

// Login verifies credentials and returns a signed JWT.
// It returns domain.ErrInvalidCredentials for both unknown email and wrong
// password so callers cannot probe which emails are registered.
func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	email = domain.NormalizeEmail(email)
	if email == "" || password == "" {
		return "", domain.ErrInvalidCredentials
	}

	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return "", domain.ErrInvalidCredentials
		}
		return "", err
	}
	if err := s.hasher.Compare(user.PasswordHash, password); err != nil {
		return "", domain.ErrInvalidCredentials
	}
	return s.tokens.Generate(user.ID)
}

// Authenticate validates a token and returns the user ID it was issued for.
func (s *AuthService) Authenticate(token string) (string, error) {
	return s.tokens.Validate(token)
}
