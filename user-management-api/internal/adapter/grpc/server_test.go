package grpc

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	userv1 "github.com/hozer006/backend-challenge/user-management-api/gen/user/v1"
	"github.com/hozer006/backend-challenge/user-management-api/internal/adapter/security"
	"github.com/hozer006/backend-challenge/user-management-api/internal/mock"
	"github.com/hozer006/backend-challenge/user-management-api/internal/service"
)

// newClient starts an in-memory gRPC server and returns a client plus a valid token.
func newClient(t *testing.T) (userv1.UserServiceClient, string) {
	t.Helper()
	repo := mock.NewUserRepository()
	hasher := security.NewBcryptHasher(bcrypt.MinCost)
	tokens := security.NewJWTService("test-secret", time.Hour)
	users := service.NewUserService(repo, hasher)
	auth := service.NewAuthService(users, repo, hasher, tokens)

	lis := bufconn.Listen(1 << 20)
	srv := NewServer(users, auth, slog.New(slog.DiscardHandler))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	token, err := tokens.Generate("caller-id")
	if err != nil {
		t.Fatal(err)
	}
	return userv1.NewUserServiceClient(conn), token
}

func withToken(token string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
}

func TestGRPC_CreateAndGet(t *testing.T) {
	client, token := newClient(t)
	ctx := withToken(token)

	created, err := client.CreateUser(ctx, &userv1.CreateUserRequest{Name: "Alice", Email: "alice@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if created.GetUser().GetId() == "" || created.GetUser().GetCreatedAt() == nil {
		t.Fatalf("unexpected user: %v", created.GetUser())
	}

	got, err := client.GetUser(ctx, &userv1.GetUserRequest{Id: created.GetUser().GetId()})
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.GetUser().GetEmail() != "alice@example.com" {
		t.Fatalf("email = %q", got.GetUser().GetEmail())
	}
}

func TestGRPC_ErrorCodes(t *testing.T) {
	client, token := newClient(t)
	ctx := withToken(token)

	_, err := client.GetUser(ctx, &userv1.GetUserRequest{Id: "missing"})
	assertCode(t, err, codes.NotFound)

	_, err = client.CreateUser(ctx, &userv1.CreateUserRequest{Name: "A", Email: "bad", Password: "password123"})
	assertCode(t, err, codes.InvalidArgument)

	req := &userv1.CreateUserRequest{Name: "A", Email: "a@example.com", Password: "password123"}
	if _, err := client.CreateUser(ctx, req); err != nil {
		t.Fatal(err)
	}
	_, err = client.CreateUser(ctx, req)
	assertCode(t, err, codes.AlreadyExists)
}

func TestGRPC_RequiresToken(t *testing.T) {
	client, _ := newClient(t)

	_, err := client.GetUser(context.Background(), &userv1.GetUserRequest{Id: "x"})
	assertCode(t, err, codes.Unauthenticated)

	_, err = client.GetUser(withToken("garbage"), &userv1.GetUserRequest{Id: "x"})
	assertCode(t, err, codes.Unauthenticated)

	basic := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Basic abc")
	_, err = client.GetUser(basic, &userv1.GetUserRequest{Id: "x"})
	assertCode(t, err, codes.Unauthenticated)
}

func assertCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("code = %v (err %v), want %v", got, err, want)
	}
}
