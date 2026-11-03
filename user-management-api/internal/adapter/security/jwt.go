package security

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/hozer006/backend-challenge/user-management-api/internal/domain"
)

const issuer = "user-management-api"

// JWTService implements port.TokenService with HMAC-SHA256 (HS256) signed tokens.
type JWTService struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

// NewJWTService creates a JWT service with the given signing secret and token lifetime.
func NewJWTService(secret string, ttl time.Duration) *JWTService {
	return &JWTService{secret: []byte(secret), ttl: ttl, now: time.Now}
}

// Generate issues a signed token whose subject is userID.
func (s *JWTService) Generate(userID string) (string, error) {
	now := s.now()
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		Issuer:    issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

// Validate verifies the signature, algorithm, issuer and expiry, and returns the subject.
func (s *JWTService) Validate(tokenString string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(*jwt.Token) (any, error) { return s.secret, nil },
		// Pin the algorithm so tokens signed with "none" or RS256 are rejected.
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(s.now),
	)
	if err != nil || !token.Valid {
		return "", domain.ErrInvalidToken
	}
	if claims.Subject == "" {
		return "", errors.Join(domain.ErrInvalidToken, errors.New("missing subject"))
	}
	return claims.Subject, nil
}
