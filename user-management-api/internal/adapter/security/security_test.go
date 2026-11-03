package security

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/hozer006/backend-challenge/user-management-api/internal/domain"
)

func TestBcryptHasher(t *testing.T) {
	h := NewBcryptHasher(bcrypt.MinCost)

	hash, err := h.Hash("secret123")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if hash == "secret123" {
		t.Fatal("hash must not equal the plain password")
	}
	if err := h.Compare(hash, "secret123"); err != nil {
		t.Fatalf("Compare correct password: %v", err)
	}
	if err := h.Compare(hash, "wrong"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("Compare wrong password = %v, want ErrInvalidCredentials", err)
	}
}

func TestJWT_GenerateAndValidate(t *testing.T) {
	svc := NewJWTService("test-secret", time.Hour)

	token, err := svc.Generate("user-1")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	sub, err := svc.Validate(token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if sub != "user-1" {
		t.Fatalf("subject = %q, want user-1", sub)
	}
}

func TestJWT_UsesHS256(t *testing.T) {
	svc := NewJWTService("test-secret", time.Hour)
	token, _ := svc.Generate("user-1")

	parsed, _, err := jwt.NewParser().ParseUnverified(token, &jwt.RegisteredClaims{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Method.Alg() != "HS256" {
		t.Fatalf("alg = %s, want HS256", parsed.Method.Alg())
	}
}

func TestJWT_Rejects(t *testing.T) {
	svc := NewJWTService("test-secret", time.Hour)
	valid, _ := svc.Generate("user-1")

	expiredSvc := NewJWTService("test-secret", time.Hour)
	expiredSvc.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	expired, _ := expiredSvc.Generate("user-1")

	otherSecret, _ := NewJWTService("other-secret", time.Hour).Generate("user-1")

	noneToken, _ := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{
		Subject:   "user-1",
		Issuer:    issuer,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)

	hs512, _ := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.RegisteredClaims{
		Subject:   "user-1",
		Issuer:    issuer,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}).SignedString([]byte("test-secret"))

	noSubject, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    issuer,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}).SignedString([]byte("test-secret"))

	cases := map[string]string{
		"empty":        "",
		"garbage":      "not.a.jwt",
		"tampered":     valid + "x",
		"expired":      expired,
		"wrong secret": otherSecret,
		"alg none":     noneToken,
		"alg HS512":    hs512,
		"no subject":   noSubject,
	}
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.Validate(tok); !errors.Is(err, domain.ErrInvalidToken) {
				t.Fatalf("Validate = %v, want ErrInvalidToken", err)
			}
		})
	}
}
