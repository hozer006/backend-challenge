// Package mock provides in-memory test doubles for the ports.
package mock

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/hozer006/backend-challenge/user-management-api/internal/domain"
	"github.com/hozer006/backend-challenge/user-management-api/internal/port"
)

// UserRepository is an in-memory, concurrency-safe port.UserRepository that
// mimics MongoDB semantics (unique email, not-found errors).
// Set Err to make every call fail with that error.
type UserRepository struct {
	mu     sync.Mutex
	users  map[string]*domain.User
	nextID int
	Err    error
	Calls  map[string]int
}

var _ port.UserRepository = (*UserRepository)(nil)

// NewUserRepository creates an empty repository.
func NewUserRepository() *UserRepository {
	return &UserRepository{users: map[string]*domain.User{}, Calls: map[string]int{}}
}

func (r *UserRepository) begin(method string) error {
	r.Calls[method]++
	return r.Err
}

func clone(u *domain.User) *domain.User {
	c := *u
	return &c
}

func (r *UserRepository) emailTaken(email, exceptID string) bool {
	for id, u := range r.users {
		if u.Email == email && id != exceptID {
			return true
		}
	}
	return false
}

func (r *UserRepository) Create(_ context.Context, user *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.begin("Create"); err != nil {
		return err
	}
	if r.emailTaken(user.Email, "") {
		return domain.ErrEmailAlreadyExists
	}
	r.nextID++
	user.ID = fmt.Sprintf("%024x", r.nextID)
	r.users[user.ID] = clone(user)
	return nil
}

func (r *UserRepository) GetByID(_ context.Context, id string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.begin("GetByID"); err != nil {
		return nil, err
	}
	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return clone(u), nil
}

func (r *UserRepository) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.begin("GetByEmail"); err != nil {
		return nil, err
	}
	for _, u := range r.users {
		if u.Email == email {
			return clone(u), nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (r *UserRepository) List(_ context.Context) ([]*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.begin("List"); err != nil {
		return nil, err
	}
	out := make([]*domain.User, 0, len(r.users))
	for _, u := range r.users {
		out = append(out, clone(u))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *UserRepository) Update(_ context.Context, id string, upd port.UserUpdate) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.begin("Update"); err != nil {
		return nil, err
	}
	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	if upd.Email != nil && r.emailTaken(*upd.Email, id) {
		return nil, domain.ErrEmailAlreadyExists
	}
	if upd.Name != nil {
		u.Name = *upd.Name
	}
	if upd.Email != nil {
		u.Email = *upd.Email
	}
	return clone(u), nil
}

func (r *UserRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.begin("Delete"); err != nil {
		return err
	}
	if _, ok := r.users[id]; !ok {
		return domain.ErrUserNotFound
	}
	delete(r.users, id)
	return nil
}

func (r *UserRepository) Count(_ context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.begin("Count"); err != nil {
		return 0, err
	}
	return int64(len(r.users)), nil
}
