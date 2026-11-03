// Package http is the REST adapter built on Fiber: handlers, middleware and routing.
package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/hozer006/backend-challenge/user-management-api/internal/domain"
)

const maxBodyBytes = 1 << 20 // 1 MiB

type userResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func toUserResponse(u *domain.User) userResponse {
	return userResponse{ID: u.ID, Name: u.Name, Email: u.Email, CreatedAt: u.CreatedAt}
}

type errorResponse struct {
	Error string `json:"error"`
	Field string `json:"field,omitempty"`
}

// decodeJSON reads a single JSON object from the body, rejecting unknown fields.
func decodeJSON(c fiber.Ctx, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(c.Body()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return domain.NewValidationError("body", "is not valid JSON: "+err.Error())
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return domain.NewValidationError("body", "must contain a single JSON object")
	}
	return nil
}

// NewErrorHandler returns a Fiber ErrorHandler that maps domain errors to HTTP
// status codes. Handlers just return errors; this is the single place that
// turns them into responses. Unknown errors become 500 and are logged without
// leaking details to the client.
func NewErrorHandler(logger *slog.Logger) fiber.ErrorHandler {
	return func(c fiber.Ctx, err error) error {
		var (
			verr *domain.ValidationError
			ferr *fiber.Error
			resp errorResponse
			code int
		)
		switch {
		case errors.As(err, &verr):
			code, resp = fiber.StatusBadRequest, errorResponse{Error: verr.Message, Field: verr.Field}
		case errors.Is(err, domain.ErrUserNotFound):
			code, resp = fiber.StatusNotFound, errorResponse{Error: err.Error()}
		case errors.Is(err, domain.ErrEmailAlreadyExists):
			code, resp = fiber.StatusConflict, errorResponse{Error: err.Error()}
		case errors.Is(err, domain.ErrInvalidCredentials), errors.Is(err, domain.ErrInvalidToken):
			code, resp = fiber.StatusUnauthorized, errorResponse{Error: err.Error()}
		case errors.As(err, &ferr):
			// Framework errors: unknown route (404), wrong method (405), body too large (413), ...
			code, resp = ferr.Code, errorResponse{Error: ferr.Message}
		default:
			logger.Error("internal error", "method", c.Method(), "path", c.Path(), "error", err)
			code, resp = fiber.StatusInternalServerError, errorResponse{Error: "internal server error"}
		}
		return c.Status(code).JSON(resp)
	}
}
