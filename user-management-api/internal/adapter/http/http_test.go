package http

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"

	"github.com/hozer006/backend-challenge/user-management-api/internal/adapter/security"
	"github.com/hozer006/backend-challenge/user-management-api/internal/mock"
	"github.com/hozer006/backend-challenge/user-management-api/internal/service"
)

// syncBuffer is a goroutine-safe buffer for capturing logs.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

type testAPI struct {
	app  *fiber.App
	repo *mock.UserRepository
	logs *syncBuffer
}

func newTestAPI(t *testing.T) *testAPI {
	t.Helper()
	repo := mock.NewUserRepository()
	hasher := security.NewBcryptHasher(bcrypt.MinCost)
	tokens := security.NewJWTService("test-secret", time.Hour)
	users := service.NewUserService(repo, hasher)
	auth := service.NewAuthService(users, repo, hasher, tokens)

	logs := &syncBuffer{}
	logger := slog.New(slog.NewTextHandler(logs, nil))
	return &testAPI{
		app:  NewApp(NewHandler(auth, users), auth, logger),
		repo: repo,
		logs: logs,
	}
}

type response struct {
	Code   int
	Header http.Header
	Body   []byte
}

func (r response) String() string { return string(r.Body) }

// send runs a request through the app in memory (no network).
func send(t *testing.T, app *fiber.App, method, path, token, body string) response {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response{Code: res.StatusCode, Header: res.Header, Body: b}
}

func (a *testAPI) do(t *testing.T, method, path, token, body string) response {
	t.Helper()
	return send(t, a.app, method, path, token, body)
}

func decode[T any](t *testing.T, res response) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(res.Body, &v); err != nil {
		t.Fatalf("decode %q: %v", res.Body, err)
	}
	return v
}

// registerAndLogin creates alice and returns her ID and token.
func (a *testAPI) registerAndLogin(t *testing.T) (string, string) {
	t.Helper()
	res := a.do(t, "POST", "/auth/register", "", `{"name":"Alice","email":"alice@example.com","password":"password123"}`)
	if res.Code != http.StatusCreated {
		t.Fatalf("register status = %d body=%s", res.Code, res)
	}
	id := decode[userResponse](t, res).ID

	res = a.do(t, "POST", "/auth/login", "", `{"email":"alice@example.com","password":"password123"}`)
	if res.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%s", res.Code, res)
	}
	return id, decode[loginResponse](t, res).AccessToken
}

func TestRegister(t *testing.T) {
	api := newTestAPI(t)

	res := api.do(t, "POST", "/auth/register", "", `{"name":"Alice","email":"Alice@Example.com","password":"password123"}`)
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s", res.Code, res)
	}
	if strings.Contains(res.String(), "password") {
		t.Fatalf("response must not include password: %s", res)
	}
	u := decode[userResponse](t, res)
	if u.ID == "" || u.Email != "alice@example.com" {
		t.Fatalf("unexpected user: %+v", u)
	}

	res = api.do(t, "POST", "/auth/register", "", `{"name":"Alice","email":"alice@example.com","password":"password123"}`)
	if res.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, want 409", res.Code)
	}
}

func TestRegister_BadRequests(t *testing.T) {
	api := newTestAPI(t)
	cases := map[string]string{
		"invalid json":   `{`,
		"unknown field":  `{"name":"A","email":"a@b.com","password":"password123","admin":true}`,
		"two objects":    `{"name":"A","email":"a@b.com","password":"password123"}{}`,
		"missing name":   `{"email":"a@b.com","password":"password123"}`,
		"bad email":      `{"name":"A","email":"bad","password":"password123"}`,
		"short password": `{"name":"A","email":"a@b.com","password":"x"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			res := api.do(t, "POST", "/auth/register", "", body)
			if res.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", res.Code, res)
			}
			if decode[errorResponse](t, res).Error == "" {
				t.Fatal("error message should be set")
			}
		})
	}
}

// TestBodyTooLarge uses a real listener: fasthttp rejects oversized bodies while
// reading the connection, which app.Test cannot observe (it returns
// fasthttp.ErrBodyTooLarge instead of a response).
//
// Only the headers are sent. The server rejects based on Content-Length and
// closes the connection, so a client still uploading the body could get a
// connection reset before reading the 413 (seen on Windows).
func TestBodyTooLarge(t *testing.T) {
	api := newTestAPI(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = api.app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}) }()
	t.Cleanup(func() { _ = api.app.Shutdown() })

	conn, err := net.DialTimeout("tcp", ln.Addr().String(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	_, err = fmt.Fprintf(conn, "POST /auth/register HTTP/1.1\r\nHost: test\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n", maxBodyBytes+1)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	if res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413; body=%s", res.StatusCode, body)
	}
	if !strings.Contains(string(body), `"error"`) {
		t.Fatalf("413 should use the JSON error format, got %s", body)
	}
}

func TestUnknownRoute(t *testing.T) {
	api := newTestAPI(t)
	res := api.do(t, "GET", "/nope", "", "")
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.Code)
	}
	if decode[errorResponse](t, res).Error == "" {
		t.Fatal("404 should use the JSON error format")
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	api := newTestAPI(t)
	api.registerAndLogin(t)

	res := api.do(t, "POST", "/auth/login", "", `{"email":"alice@example.com","password":"wrongpass"}`)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", res.Code)
	}
}

func TestProtectedRoutes_RequireToken(t *testing.T) {
	api := newTestAPI(t)
	routes := [][2]string{
		{"GET", "/users"},
		{"POST", "/users"},
		{"GET", "/users/abc"},
		{"PATCH", "/users/abc"},
		{"DELETE", "/users/abc"},
	}
	for _, rt := range routes {
		for name, token := range map[string]string{"no token": "", "bad token": "garbage"} {
			t.Run(rt[0]+" "+rt[1]+" "+name, func(t *testing.T) {
				res := api.do(t, rt[0], rt[1], token, "")
				if res.Code != http.StatusUnauthorized {
					t.Fatalf("status = %d, want 401", res.Code)
				}
				if res.Header.Get("WWW-Authenticate") == "" {
					t.Fatal("missing WWW-Authenticate header")
				}
			})
		}
	}
}

func TestUserCRUD(t *testing.T) {
	api := newTestAPI(t)
	aliceID, token := api.registerAndLogin(t)

	// Create
	res := api.do(t, "POST", "/users", token, `{"name":"Bob","email":"bob@example.com","password":"password123"}`)
	if res.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", res.Code, res)
	}
	bobID := decode[userResponse](t, res).ID

	// List
	res = api.do(t, "GET", "/users", token, "")
	if res.Code != http.StatusOK {
		t.Fatalf("list status = %d", res.Code)
	}
	if n := len(decode[[]userResponse](t, res)); n != 2 {
		t.Fatalf("list returned %d users, want 2", n)
	}

	// Get
	res = api.do(t, "GET", "/users/"+aliceID, token, "")
	if res.Code != http.StatusOK || decode[userResponse](t, res).Name != "Alice" {
		t.Fatalf("get status = %d body=%s", res.Code, res)
	}

	// Update
	res = api.do(t, "PATCH", "/users/"+bobID, token, `{"name":"Robert"}`)
	if res.Code != http.StatusOK || decode[userResponse](t, res).Name != "Robert" {
		t.Fatalf("update status = %d body=%s", res.Code, res)
	}
	res = api.do(t, "PATCH", "/users/"+bobID, token, `{"email":"alice@example.com"}`)
	if res.Code != http.StatusConflict {
		t.Fatalf("update to taken email status = %d, want 409", res.Code)
	}
	res = api.do(t, "PATCH", "/users/"+bobID, token, `{}`)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("empty update status = %d, want 400", res.Code)
	}

	// Delete
	res = api.do(t, "DELETE", "/users/"+bobID, token, "")
	if res.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", res.Code)
	}
	res = api.do(t, "GET", "/users/"+bobID, token, "")
	if res.Code != http.StatusNotFound {
		t.Fatalf("get deleted status = %d, want 404", res.Code)
	}
}

func TestInternalErrorIsHidden(t *testing.T) {
	api := newTestAPI(t)
	_, token := api.registerAndLogin(t)
	api.repo.Err = errors.New("secret db details")

	res := api.do(t, "GET", "/users", token, "")
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", res.Code)
	}
	if strings.Contains(res.String(), "secret db details") {
		t.Fatal("internal error details leaked to client")
	}
	if !strings.Contains(api.logs.String(), "secret db details") {
		t.Fatal("internal error should be logged")
	}
}

func TestLoggingMiddleware(t *testing.T) {
	var buf syncBuffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	app := fiber.New(fiber.Config{ErrorHandler: NewErrorHandler(logger)})
	app.Use(Logging(logger))
	app.Get("/teapot", func(c fiber.Ctx) error { return c.SendStatus(http.StatusTeapot) })
	app.Get("/fail", func(c fiber.Ctx) error { return errors.New("boom") })

	send(t, app, "GET", "/teapot", "", "")
	out := buf.String()
	for _, want := range []string{"method=GET", "path=/teapot", "status=418", "duration_ms="} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %q:\n%s", want, out)
		}
	}

	// A handler error must be logged with the status the client actually received.
	res := send(t, app, "GET", "/fail", "", "")
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", res.Code)
	}
	if !strings.Contains(buf.String(), "path=/fail status=500") {
		t.Errorf("error request should be logged with status 500:\n%s", buf.String())
	}
}

type stubValidator struct{}

func (stubValidator) Authenticate(token string) (string, error) {
	if token == "good" {
		return "user-1", nil
	}
	return "", errors.New("bad")
}

func TestRequireAuth_SetsUserID(t *testing.T) {
	var gotID string
	app := fiber.New(fiber.Config{ErrorHandler: NewErrorHandler(slog.New(slog.DiscardHandler))})
	app.Get("/", RequireAuth(stubValidator{}), func(c fiber.Ctx) error {
		gotID, _ = UserIDFromContext(c)
		return c.SendStatus(http.StatusOK)
	})

	cases := map[string]int{
		"Bearer good":  http.StatusOK,
		"bearer good":  http.StatusOK,
		"Bearer  good": http.StatusOK,
		"Bearer bad":   http.StatusUnauthorized,
		"Basic good":   http.StatusUnauthorized,
		"Bearer":       http.StatusUnauthorized,
		"":             http.StatusUnauthorized,
	}
	for header, want := range cases {
		t.Run(header, func(t *testing.T) {
			gotID = ""
			req := httptest.NewRequest("GET", "/", nil)
			if header != "" {
				req.Header.Set("Authorization", header)
			}
			res, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			if res.StatusCode != want {
				t.Fatalf("status = %d, want %d", res.StatusCode, want)
			}
			if want == http.StatusOK && gotID != "user-1" {
				t.Fatalf("user ID in context = %q", gotID)
			}
		})
	}
}
