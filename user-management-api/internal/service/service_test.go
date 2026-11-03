package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/hozer006/backend-challenge/user-management-api/internal/adapter/security"
	"github.com/hozer006/backend-challenge/user-management-api/internal/domain"
	"github.com/hozer006/backend-challenge/user-management-api/internal/mock"
)

type fixture struct {
	repo  *mock.UserRepository
	users *UserService
	auth  *AuthService
}

func newFixture() fixture {
	repo := mock.NewUserRepository()
	hasher := security.NewBcryptHasher(bcrypt.MinCost)
	tokens := security.NewJWTService("test-secret", time.Hour)
	users := NewUserService(repo, hasher)
	return fixture{repo: repo, users: users, auth: NewAuthService(users, repo, hasher, tokens)}
}

func ptr(s string) *string { return &s }

var validInput = CreateUserInput{Name: " Alice ", Email: " Alice@Example.com ", Password: "password123"}

func TestCreate_NormalizesAndHashes(t *testing.T) {
	f := newFixture()
	u, err := f.users.Create(context.Background(), validInput)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID == "" {
		t.Error("ID should be set")
	}
	if u.Name != "Alice" || u.Email != "alice@example.com" {
		t.Errorf("not normalized: name=%q email=%q", u.Name, u.Email)
	}
	if u.PasswordHash == "" || u.PasswordHash == validInput.Password {
		t.Error("password must be hashed")
	}
	if u.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if !u.CreatedAt.Equal(u.CreatedAt.Truncate(time.Millisecond)) {
		t.Errorf("CreatedAt %v should be truncated to milliseconds (MongoDB precision)", u.CreatedAt)
	}
}

func TestCreate_ValidationErrors(t *testing.T) {
	cases := map[string]CreateUserInput{
		"missing name":   {Email: "a@b.com", Password: "password123"},
		"bad email":      {Name: "A", Email: "nope", Password: "password123"},
		"short password": {Name: "A", Email: "a@b.com", Password: "123"},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture()
			_, err := f.users.Create(context.Background(), in)
			if !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("err = %v, want validation error", err)
			}
			if f.repo.Calls["Create"] != 0 {
				t.Fatal("repository must not be called on invalid input")
			}
		})
	}
}

func TestCreate_DuplicateEmail(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	if _, err := f.users.Create(ctx, validInput); err != nil {
		t.Fatal(err)
	}
	dup := validInput
	dup.Email = "ALICE@example.com" // same after normalization
	if _, err := f.users.Create(ctx, dup); !errors.Is(err, domain.ErrEmailAlreadyExists) {
		t.Fatalf("err = %v, want ErrEmailAlreadyExists", err)
	}
}

func TestGetListDelete(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	u, _ := f.users.Create(ctx, validInput)

	got, err := f.users.GetByID(ctx, u.ID)
	if err != nil || got.Email != u.Email {
		t.Fatalf("GetByID = %v, %v", got, err)
	}

	list, err := f.users.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("List = %d users, err %v", len(list), err)
	}

	if err := f.users.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := f.users.GetByID(ctx, u.ID); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("after delete err = %v, want ErrUserNotFound", err)
	}
	if err := f.users.Delete(ctx, u.ID); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("second delete err = %v, want ErrUserNotFound", err)
	}
}

func TestUpdate(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	u, _ := f.users.Create(ctx, validInput)
	other, _ := f.users.Create(ctx, CreateUserInput{Name: "Bob", Email: "bob@example.com", Password: "password123"})

	t.Run("name only", func(t *testing.T) {
		got, err := f.users.Update(ctx, u.ID, UpdateUserInput{Name: ptr("  Alicia ")})
		if err != nil || got.Name != "Alicia" || got.Email != "alice@example.com" {
			t.Fatalf("got %+v, err %v", got, err)
		}
	})
	t.Run("email only", func(t *testing.T) {
		got, err := f.users.Update(ctx, u.ID, UpdateUserInput{Email: ptr("NEW@example.com")})
		if err != nil || got.Email != "new@example.com" {
			t.Fatalf("got %+v, err %v", got, err)
		}
	})
	t.Run("empty body", func(t *testing.T) {
		if _, err := f.users.Update(ctx, u.ID, UpdateUserInput{}); !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("err = %v, want validation error", err)
		}
	})
	t.Run("invalid email", func(t *testing.T) {
		if _, err := f.users.Update(ctx, u.ID, UpdateUserInput{Email: ptr("bad")}); !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("err = %v, want validation error", err)
		}
	})
	t.Run("email taken", func(t *testing.T) {
		if _, err := f.users.Update(ctx, u.ID, UpdateUserInput{Email: ptr(other.Email)}); !errors.Is(err, domain.ErrEmailAlreadyExists) {
			t.Fatalf("err = %v, want ErrEmailAlreadyExists", err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		if _, err := f.users.Update(ctx, "missing", UpdateUserInput{Name: ptr("X")}); !errors.Is(err, domain.ErrUserNotFound) {
			t.Fatalf("err = %v, want ErrUserNotFound", err)
		}
	})
}

func TestRepositoryErrorPropagates(t *testing.T) {
	f := newFixture()
	boom := errors.New("db down")
	f.repo.Err = boom
	if _, err := f.users.List(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
	if _, err := f.users.Count(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}

func TestLogin(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	u, err := f.auth.Register(ctx, validInput)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	token, err := f.auth.Login(ctx, "ALICE@example.com", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	sub, err := f.auth.Authenticate(token)
	if err != nil || sub != u.ID {
		t.Fatalf("Authenticate = %q, %v; want %q", sub, err, u.ID)
	}

	for name, creds := range map[string][2]string{
		"wrong password": {"alice@example.com", "wrongpass"},
		"unknown email":  {"nobody@example.com", "password123"},
		"empty":          {"", ""},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := f.auth.Login(ctx, creds[0], creds[1]); !errors.Is(err, domain.ErrInvalidCredentials) {
				t.Fatalf("err = %v, want ErrInvalidCredentials", err)
			}
		})
	}
}

func TestLogin_RepositoryError(t *testing.T) {
	f := newFixture()
	boom := errors.New("db down")
	f.repo.Err = boom
	if _, err := f.auth.Login(context.Background(), "a@b.com", "password123"); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v (must not be masked as invalid credentials)", err, boom)
	}
}
