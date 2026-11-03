// Package grpc is the gRPC adapter for the user service.
package grpc

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	userv1 "github.com/hozer006/backend-challenge/user-management-api/gen/user/v1"
	"github.com/hozer006/backend-challenge/user-management-api/internal/domain"
	"github.com/hozer006/backend-challenge/user-management-api/internal/service"
)

// TokenValidator validates a bearer token and returns the user ID.
type TokenValidator interface {
	Authenticate(token string) (string, error)
}

// UserServer implements userv1.UserServiceServer.
type UserServer struct {
	userv1.UnimplementedUserServiceServer
	users  *service.UserService
	logger *slog.Logger
}

// NewUserServer creates a gRPC user server.
func NewUserServer(users *service.UserService, logger *slog.Logger) *UserServer {
	return &UserServer{users: users, logger: logger}
}

// NewServer builds a *grpc.Server with JWT auth and registers the user service.
func NewServer(users *service.UserService, validator TokenValidator, logger *slog.Logger) *grpc.Server {
	srv := grpc.NewServer(grpc.UnaryInterceptor(AuthInterceptor(validator)))
	userv1.RegisterUserServiceServer(srv, NewUserServer(users, logger))
	return srv
}

// CreateUser creates a user.
func (s *UserServer) CreateUser(ctx context.Context, req *userv1.CreateUserRequest) (*userv1.CreateUserResponse, error) {
	user, err := s.users.Create(ctx, service.CreateUserInput{
		Name:     req.GetName(),
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
	})
	if err != nil {
		return nil, s.toStatus(err)
	}
	return &userv1.CreateUserResponse{User: toProto(user)}, nil
}

// GetUser fetches a user by ID.
func (s *UserServer) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
	user, err := s.users.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, s.toStatus(err)
	}
	return &userv1.GetUserResponse{User: toProto(user)}, nil
}

func toProto(u *domain.User) *userv1.User {
	return &userv1.User{
		Id:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: timestamppb.New(u.CreatedAt),
	}
}

func (s *UserServer) toStatus(err error) error {
	var verr *domain.ValidationError
	switch {
	case errors.As(err, &verr):
		return status.Error(codes.InvalidArgument, verr.Error())
	case errors.Is(err, domain.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrEmailAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		s.logger.Error("grpc internal error", "error", err)
		return status.Error(codes.Internal, "internal error")
	}
}

// AuthInterceptor requires "authorization: Bearer <jwt>" metadata on every unary call.
func AuthInterceptor(validator TokenValidator) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}
		values := md.Get("authorization")
		if len(values) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization token")
		}
		scheme, token, found := strings.Cut(values[0], " ")
		if !found || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
			return nil, status.Error(codes.Unauthenticated, "invalid authorization header")
		}
		if _, err := validator.Authenticate(strings.TrimSpace(token)); err != nil {
			return nil, status.Error(codes.Unauthenticated, domain.ErrInvalidToken.Error())
		}
		return handler(ctx, req)
	}
}
