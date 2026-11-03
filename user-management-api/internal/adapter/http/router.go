package http

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
)

// NewApp builds the Fiber app with routes and middleware.
//
//	Public:    GET /healthz, POST /auth/register, POST /auth/login
//	Protected: POST /users, GET /users, GET/PATCH/DELETE /users/:id
func NewApp(h *Handler, validator TokenValidator, logger *slog.Logger) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "user-management-api",
		ErrorHandler: NewErrorHandler(logger),
		BodyLimit:    maxBodyBytes,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	})

	app.Use(Logging(logger))

	app.Get("/healthz", h.Health)

	auth := app.Group("/auth")
	auth.Post("/register", h.Register)
	auth.Post("/login", h.Login)

	users := app.Group("/users", RequireAuth(validator))
	users.Post("/", h.CreateUser)
	users.Get("/", h.ListUsers)
	users.Get("/:id", h.GetUser)
	users.Patch("/:id", h.UpdateUser)
	users.Delete("/:id", h.DeleteUser)

	return app
}
