package http

import (
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/hozer006/backend-challenge/user-management-api/internal/domain"
)

type ctxKey int

const userIDKey ctxKey = iota

// UserIDFromContext returns the authenticated user ID set by RequireAuth.
func UserIDFromContext(c fiber.Ctx) (string, bool) {
	id, ok := c.Locals(userIDKey).(string)
	return id, ok
}

// Logging logs the HTTP method, path, status and execution time of every request.
//
// It must be registered first. When a downstream handler returns an error, the
// app's ErrorHandler is invoked here so the logged status is the one actually
// sent to the client (Fiber would otherwise render the error after this returns).
func Logging(logger *slog.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		if err := c.Next(); err != nil {
			if herr := c.App().ErrorHandler(c, err); herr != nil {
				_ = c.SendStatus(fiber.StatusInternalServerError)
			}
		}
		logger.Info("http request",
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"duration_ms", float64(time.Since(start).Microseconds())/1000,
		)
		return nil
	}
}

// TokenValidator validates a bearer token and returns the user ID.
type TokenValidator interface {
	Authenticate(token string) (string, error)
}

// RequireAuth rejects requests without a valid "Authorization: Bearer <jwt>" header.
func RequireAuth(validator TokenValidator) fiber.Handler {
	return func(c fiber.Ctx) error {
		token, ok := bearerToken(c.Get(fiber.HeaderAuthorization))
		if !ok {
			c.Set(fiber.HeaderWWWAuthenticate, `Bearer`)
			return domain.ErrInvalidToken
		}
		userID, err := validator.Authenticate(token)
		if err != nil {
			c.Set(fiber.HeaderWWWAuthenticate, `Bearer error="invalid_token"`)
			return domain.ErrInvalidToken
		}
		c.Locals(userIDKey, userID)
		return c.Next()
	}
}

func bearerToken(header string) (string, bool) {
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}
