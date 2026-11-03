package http

import (
	"github.com/gofiber/fiber/v3"

	"github.com/hozer006/backend-challenge/user-management-api/internal/service"
)

// Handler serves the REST API. Handlers return errors; NewErrorHandler renders them.
type Handler struct {
	auth  *service.AuthService
	users *service.UserService
}

// NewHandler creates a Handler.
func NewHandler(auth *service.AuthService, users *service.UserService) *Handler {
	return &Handler{auth: auth, users: users}
}

type createUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type updateUserRequest struct {
	Name  *string `json:"name"`
	Email *string `json:"email"`
}

// Register handles POST /auth/register.
func (h *Handler) Register(c fiber.Ctx) error {
	var req createUserRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	user, err := h.auth.Register(c.Context(), service.CreateUserInput(req))
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(toUserResponse(user))
}

// Login handles POST /auth/login.
func (h *Handler) Login(c fiber.Ctx) error {
	var req loginRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	token, err := h.auth.Login(c.Context(), req.Email, req.Password)
	if err != nil {
		return err
	}
	return c.JSON(loginResponse{AccessToken: token, TokenType: "Bearer"})
}

// CreateUser handles POST /users.
func (h *Handler) CreateUser(c fiber.Ctx) error {
	var req createUserRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	user, err := h.users.Create(c.Context(), service.CreateUserInput(req))
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(toUserResponse(user))
}

// ListUsers handles GET /users.
func (h *Handler) ListUsers(c fiber.Ctx) error {
	users, err := h.users.List(c.Context())
	if err != nil {
		return err
	}
	resp := make([]userResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, toUserResponse(u))
	}
	return c.JSON(resp)
}

// GetUser handles GET /users/:id.
func (h *Handler) GetUser(c fiber.Ctx) error {
	user, err := h.users.GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(toUserResponse(user))
}

// UpdateUser handles PATCH /users/:id.
func (h *Handler) UpdateUser(c fiber.Ctx) error {
	var req updateUserRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	user, err := h.users.Update(c.Context(), c.Params("id"), service.UpdateUserInput(req))
	if err != nil {
		return err
	}
	return c.JSON(toUserResponse(user))
}

// DeleteUser handles DELETE /users/:id.
func (h *Handler) DeleteUser(c fiber.Ctx) error {
	if err := h.users.Delete(c.Context(), c.Params("id")); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// Health handles GET /healthz.
func (h *Handler) Health(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}
