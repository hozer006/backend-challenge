//go:build integration

// Integration tests against a real MongoDB. Run with:
//
//	MONGO_URI=mongodb://localhost:27017 go test -tags=integration ./test/...
package test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	mongoadapter "github.com/hozer006/backend-challenge/user-management-api/internal/adapter/mongo"
	"github.com/hozer006/backend-challenge/user-management-api/internal/domain"
	"github.com/hozer006/backend-challenge/user-management-api/internal/port"
)

func newRepo(t *testing.T) *mongoadapter.UserRepository {
	t.Helper()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		t.Skipf("MongoDB not reachable at %s: %v", uri, err)
	}

	db := client.Database("user_mgmt_test_" + bson.NewObjectID().Hex())
	t.Cleanup(func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	})

	repo := mongoadapter.NewUserRepository(db)
	if err := repo.EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestMongoUserRepository(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()

	u := &domain.User{Name: "Alice", Email: "alice@example.com", PasswordHash: "hash", CreatedAt: time.Now().UTC()}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID == "" {
		t.Fatal("ID not set")
	}

	dup := &domain.User{Name: "Other", Email: "alice@example.com", PasswordHash: "h", CreatedAt: time.Now()}
	if err := repo.Create(ctx, dup); !errors.Is(err, domain.ErrEmailAlreadyExists) {
		t.Fatalf("duplicate Create = %v, want ErrEmailAlreadyExists", err)
	}

	got, err := repo.GetByEmail(ctx, "alice@example.com")
	if err != nil || got.ID != u.ID || got.PasswordHash != "hash" {
		t.Fatalf("GetByEmail = %+v, %v", got, err)
	}

	name := "Alicia"
	updated, err := repo.Update(ctx, u.ID, port.UserUpdate{Name: &name})
	if err != nil || updated.Name != "Alicia" {
		t.Fatalf("Update = %+v, %v", updated, err)
	}

	if n, err := repo.Count(ctx); err != nil || n != 1 {
		t.Fatalf("Count = %d, %v", n, err)
	}
	if list, err := repo.List(ctx); err != nil || len(list) != 1 {
		t.Fatalf("List = %d, %v", len(list), err)
	}

	if err := repo.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, u.ID); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("GetByID after delete = %v", err)
	}
	if _, err := repo.GetByID(ctx, "not-an-object-id"); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("GetByID invalid id = %v", err)
	}
}
