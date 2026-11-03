# User Management API

A RESTful (Fiber v3) and gRPC user management service in Go, backed by MongoDB, with JWT (HS256) authentication.
Built with a hexagonal (ports and adapters) architecture.

## Features

| Requirement | Where |
| --- | --- |
| User model (`ID`, `Name`, `Email` unique, hashed `Password`, `CreatedAt`) | `internal/domain/user.go`, unique index in `internal/adapter/mongo` |
| Register / Login returning JWT | `internal/service/auth_service.go`, `POST /auth/register`, `POST /auth/login` |
| JWT HS256 signing + validation middleware | `internal/adapter/security/jwt.go`, `internal/adapter/http/middleware.go` |
| CRUD: create, get by ID, list, update name/email, delete | `internal/service/user_service.go`, `internal/adapter/http/handler.go` |
| Official MongoDB Go driver (v2) | `internal/adapter/mongo/user_repository.go` |
| Logging middleware (method, path, status, duration) | `internal/adapter/http/middleware.go` |
| Goroutine logging user count every 10s | `internal/worker/user_counter.go` |
| Unit tests with `testing` + mocked repository | `*_test.go`, `internal/mock` |
| **Bonus:** Docker + docker-compose | `Dockerfile`, `docker-compose.yml` |
| **Bonus:** Interfaces abstracting MongoDB | `internal/port/ports.go` |
| **Bonus:** Input validation | `internal/domain/user.go` |
| **Bonus:** Graceful shutdown via `context.Context` | `cmd/server/main.go` |
| **Bonus:** gRPC `CreateUser` / `GetUser` with token metadata | `proto/user/v1/user.proto`, `internal/adapter/grpc` |
| **Bonus:** Hexagonal architecture | see below |

## Project Structure

```
user-management-api/
├── cmd/server/            # main: wiring, servers, graceful shutdown
├── internal/
│   ├── domain/            # entities, validation rules, domain errors (no external deps)
│   ├── port/              # interfaces: UserRepository, PasswordHasher, TokenService
│   ├── service/           # use cases: UserService, AuthService
│   ├── adapter/
│   │   ├── http/          # REST handlers, middleware, routing (Fiber v3)
│   │   ├── grpc/          # gRPC server + auth interceptor
│   │   ├── mongo/         # MongoDB repository
│   │   └── security/      # bcrypt hasher, JWT service
│   ├── worker/            # background user-count job
│   ├── mock/              # in-memory UserRepository for tests
│   └── config/            # env configuration
├── proto/user/v1/         # protobuf definition
├── gen/user/v1/           # generated gRPC code (buf generate)
└── test/                  # integration tests (real MongoDB, build tag `integration`)
```

Dependency direction: `adapter → service → port/domain`. The domain and service layers never import
MongoDB, HTTP, gRPC or JWT libraries; they only depend on the interfaces in `port/`.

## Getting Started

### Prerequisites

- Go 1.27+
- MongoDB 6+ (or Docker)

### Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `JWT_SECRET` | **required** | HMAC secret for signing tokens. The service refuses to start without it. |
| `JWT_TTL` | `24h` | Token lifetime (Go duration). |
| `HTTP_PORT` | `8080` | REST port. |
| `GRPC_PORT` | `9090` | gRPC port. |
| `MONGO_URI` | `mongodb://localhost:27017` | MongoDB connection string. |
| `MONGO_DATABASE` | `user_management` | Database name. |

Copy `.env.example` to `.env` and set a strong secret, e.g. `openssl rand -base64 32`.

### Run with Docker Compose

```bash
cp .env.example .env          # then edit JWT_SECRET
docker compose up --build
```

The API is at `http://localhost:8080`, gRPC at `localhost:9090`.

### Run locally

```bash
docker run -d --name mongo -p 27017:27017 mongo:8   # or use your own MongoDB
export JWT_SECRET="$(openssl rand -base64 32)"
go run ./cmd/server
```

PowerShell:

```powershell
$env:JWT_SECRET = "change-me-to-a-long-random-string"
go run ./cmd/server
```

### Tests

```bash
go test ./...                                 # unit tests (no MongoDB needed)
go test -cover ./...                          # with coverage
go test -tags=integration ./test/...          # integration tests against a real MongoDB
```

Unit tests replace MongoDB with `internal/mock.UserRepository`, an in-memory implementation of
`port.UserRepository` that mimics the unique-email and not-found behaviour. Handlers are tested end to end
through the real Fiber app with `app.Test` (in memory, no network), and the gRPC server through an in-memory `bufconn` listener.

### Regenerating gRPC code

```bash
go install github.com/bufbuild/buf/cmd/buf@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
buf generate
```

## Using JWT

1. **Register** a user (public endpoint).
2. **Login** with email and password. The response contains `access_token`.
3. Send the token on every protected request:

   ```
   Authorization: Bearer <access_token>
   ```

Token details:

- Algorithm: **HS256** (HMAC-SHA256), signed with `JWT_SECRET`.
- Claims: `sub` (user ID), `iss` (`user-management-api`), `iat`, `exp`.
- Validation rejects: bad signature, expired tokens, missing `exp`/`sub`, wrong issuer, and any algorithm
  other than HS256 (including `none`), which prevents algorithm-confusion attacks.
- Invalid or missing tokens get `401` with a `WWW-Authenticate: Bearer` header.

For gRPC, send the same value as metadata: `authorization: Bearer <access_token>`.

## API Reference

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `GET` | `/healthz` | – | Health check |
| `POST` | `/auth/register` | – | Register a user |
| `POST` | `/auth/login` | – | Login, returns JWT |
| `POST` | `/users` | JWT | Create a user |
| `GET` | `/users` | JWT | List all users |
| `GET` | `/users/:id` | JWT | Get a user by ID |
| `PATCH` | `/users/:id` | JWT | Update name and/or email |
| `DELETE` | `/users/:id` | JWT | Delete a user |

### Errors

All errors are JSON: `{"error": "<message>", "field": "<field, for validation errors>"}`.

| Status | When |
| --- | --- |
| `400` | Invalid JSON, unknown fields, validation failure |
| `401` | Missing/invalid token, wrong credentials |
| `404` | User not found (including malformed IDs) |
| `409` | Email already exists |
| `500` | Unexpected error (details are logged, not returned) |

### Sample Requests

**Register**

```bash
curl -s -X POST http://localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"alice@example.com","password":"password123"}'
```

```json
HTTP/1.1 201 Created
{"id":"6720f1c2a1b2c3d4e5f60718","name":"Alice","email":"alice@example.com","created_at":"2026-11-03T09:00:00Z"}
```

**Login**

```bash
curl -s -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"password123"}'
```

```json
HTTP/1.1 200 OK
{"access_token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...","token_type":"Bearer"}
```

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"password123"}' | jq -r .access_token)
```

**Create user**

```bash
curl -s -X POST http://localhost:8080/users \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"Bob","email":"bob@example.com","password":"password123"}'
```

```json
HTTP/1.1 201 Created
{"id":"6720f1d0a1b2c3d4e5f60719","name":"Bob","email":"bob@example.com","created_at":"2026-11-03T09:01:00Z"}
```

**List users**

```bash
curl -s http://localhost:8080/users -H "Authorization: Bearer $TOKEN"
```

```json
HTTP/1.1 200 OK
[
  {"id":"6720f1c2a1b2c3d4e5f60718","name":"Alice","email":"alice@example.com","created_at":"2026-11-03T09:00:00Z"},
  {"id":"6720f1d0a1b2c3d4e5f60719","name":"Bob","email":"bob@example.com","created_at":"2026-11-03T09:01:00Z"}
]
```

**Get user**

```bash
curl -s http://localhost:8080/users/6720f1d0a1b2c3d4e5f60719 -H "Authorization: Bearer $TOKEN"
```

**Update user**

```bash
curl -s -X PATCH http://localhost:8080/users/6720f1d0a1b2c3d4e5f60719 \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"Robert"}'
```

```json
HTTP/1.1 200 OK
{"id":"6720f1d0a1b2c3d4e5f60719","name":"Robert","email":"bob@example.com","created_at":"2026-11-03T09:01:00Z"}
```

**Delete user**

```bash
curl -s -i -X DELETE http://localhost:8080/users/6720f1d0a1b2c3d4e5f60719 -H "Authorization: Bearer $TOKEN"
```

```
HTTP/1.1 204 No Content
```

**Error example**

```json
HTTP/1.1 400 Bad Request
{"error":"is not a valid email address","field":"email"}
```

**gRPC** (with [grpcurl](https://github.com/fullstorydev/grpcurl))

```bash
grpcurl -plaintext -import-path proto -proto user/v1/user.proto \
  -H "authorization: Bearer $TOKEN" \
  -d '{"name":"Carol","email":"carol@example.com","password":"password123"}' \
  localhost:9090 user.v1.UserService/CreateUser
```

### Logs

Logs are structured JSON (`log/slog`):

```json
{"time":"...","level":"INFO","msg":"http request","method":"GET","path":"/users","status":200,"duration_ms":1.843}
{"time":"...","level":"INFO","msg":"total users","count":2}
```

## Design Decisions and Assumptions

- **Hexagonal architecture.** Business rules live in `domain` and `service` and depend only on `port`
  interfaces. MongoDB, HTTP, gRPC, bcrypt and JWT are adapters, so each can be swapped or mocked.
- **Email uniqueness is enforced by a MongoDB unique index**, not a check-then-insert, which avoids a race
  between two concurrent registrations. Duplicate-key errors are mapped to `409 Conflict`.
- **Emails are normalized** (trimmed and lowercased) so `Alice@X.com` and `alice@x.com` are the same account.
- **Passwords** are hashed with bcrypt (default cost). Length is limited to 8–72 bytes because bcrypt ignores
  bytes after 72. Password hashes are never returned by the API.
- **Login returns the same error** for an unknown email and a wrong password so the endpoint cannot be used
  to discover which emails are registered.
- **`POST /users` requires a password** because every user must be able to log in. It shares validation
  with `/auth/register`; the difference is that it is a protected (admin-style) operation.
- **Authorization scope:** any authenticated user may manage any user. The requirements do not define roles
  or ownership; adding them would be a change in the service layer (for example, only allow users to
  update themselves).
- **Updating** uses `PATCH` with optional fields; at least one of `name` or `email` is required. Passwords are
  not updatable through this endpoint, matching the requirements.
- **Malformed IDs return `404`**, not `400`, because from the client's point of view no such user exists.
- **The request body is limited to 1 MiB** and unknown JSON fields are rejected.
- **No JWT secret default.** The service refuses to start without `JWT_SECRET` to avoid shipping a guessable secret.
- **Graceful shutdown:** on `SIGINT`/`SIGTERM` the root context is cancelled, the background worker stops, the
  HTTP server drains in-flight requests, gRPC uses `GracefulStop`, and the MongoDB client disconnects
  (10 s timeout overall).
- **HTTP framework: [Fiber v3](https://gofiber.io)** (built on fasthttp). Handlers return errors and a single
  custom `ErrorHandler` maps domain errors to status codes, so error handling lives in one place. The logging
  middleware calls that error handler itself so it logs the status the client actually received. Because of
  hexagonal architecture, Fiber is confined to `internal/adapter/http`; domain and services do not import it.
- **Listing is unpaginated**, as specified. For large datasets, cursor-based pagination would be added.
