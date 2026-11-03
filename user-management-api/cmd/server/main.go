// Command server runs the User Management REST and gRPC APIs.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	grpcadapter "github.com/hozer006/backend-challenge/user-management-api/internal/adapter/grpc"
	httpadapter "github.com/hozer006/backend-challenge/user-management-api/internal/adapter/http"
	mongoadapter "github.com/hozer006/backend-challenge/user-management-api/internal/adapter/mongo"
	"github.com/hozer006/backend-challenge/user-management-api/internal/adapter/security"
	"github.com/hozer006/backend-challenge/user-management-api/internal/config"
	"github.com/hozer006/backend-challenge/user-management-api/internal/service"
	"github.com/hozer006/backend-challenge/user-management-api/internal/worker"
)

const shutdownTimeout = 10 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// ctx is cancelled on SIGINT/SIGTERM, which starts graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --- MongoDB ---
	client, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		return fmt.Errorf("connect mongo: %w", err)
	}
	defer func() {
		dctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := client.Disconnect(dctx); err != nil {
			logger.Error("disconnect mongo", "error", err)
		}
	}()

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	err = client.Ping(pingCtx, readpref.Primary())
	cancel()
	if err != nil {
		return fmt.Errorf("ping mongo: %w", err)
	}

	repo := mongoadapter.NewUserRepository(client.Database(cfg.MongoDatabase))
	idxCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	err = repo.EnsureIndexes(idxCtx)
	cancel()
	if err != nil {
		return err
	}

	// --- Application core ---
	hasher := security.NewBcryptHasher(0)
	tokens := security.NewJWTService(cfg.JWTSecret, cfg.JWTTTL)
	userSvc := service.NewUserService(repo, hasher)
	authSvc := service.NewAuthService(userSvc, repo, hasher, tokens)

	// --- Servers ---
	app := httpadapter.NewApp(httpadapter.NewHandler(authSvc, userSvc), authSvc, logger)
	httpLis, err := net.Listen("tcp", ":"+cfg.HTTPPort)
	if err != nil {
		return fmt.Errorf("listen http: %w", err)
	}

	grpcSrv := grpcadapter.NewServer(userSvc, authSvc, logger)
	grpcLis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return fmt.Errorf("listen grpc: %w", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	wg.Go(func() { worker.RunUserCounter(ctx, userSvc, cfg.UserCountTick, logger) })

	wg.Go(func() {
		logger.Info("http server listening", "addr", httpLis.Addr().String())
		// Listener returns nil after a graceful shutdown.
		if err := app.Listener(httpLis, fiber.ListenConfig{DisableStartupMessage: true}); err != nil {
			errCh <- fmt.Errorf("http server: %w", err)
		}
	})

	wg.Go(func() {
		logger.Info("grpc server listening", "addr", grpcLis.Addr().String())
		if err := grpcSrv.Serve(grpcLis); err != nil {
			errCh <- fmt.Errorf("grpc server: %w", err)
		}
	})

	// Wait for a shutdown signal or a server failure.
	var runErr error
	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case runErr = <-errCh:
		logger.Error("server failed, shutting down", "error", runErr)
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Error("http shutdown", "error", err)
	}

	grpcDone := make(chan struct{})
	go func() { grpcSrv.GracefulStop(); close(grpcDone) }()
	select {
	case <-grpcDone:
	case <-shutdownCtx.Done():
		grpcSrv.Stop()
	}

	wg.Wait()
	logger.Info("shutdown complete")
	return runErr
}
