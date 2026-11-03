// Package mongo implements port.UserRepository on top of the official MongoDB Go driver.
package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/hozer006/backend-challenge/user-management-api/internal/domain"
	"github.com/hozer006/backend-challenge/user-management-api/internal/port"
)

const usersCollection = "users"

// userDocument is the MongoDB representation of a user.
// Keeping it separate from domain.User keeps bson tags out of the domain.
type userDocument struct {
	ID        bson.ObjectID `bson:"_id"`
	Name      string        `bson:"name"`
	Email     string        `bson:"email"`
	Password  string        `bson:"password"`
	CreatedAt time.Time     `bson:"created_at"`
}

func (d *userDocument) toDomain() *domain.User {
	return &domain.User{
		ID:           d.ID.Hex(),
		Name:         d.Name,
		Email:        d.Email,
		PasswordHash: d.Password,
		CreatedAt:    d.CreatedAt,
	}
}

// UserRepository stores users in a MongoDB collection.
type UserRepository struct {
	coll *mongo.Collection
}

var _ port.UserRepository = (*UserRepository)(nil)

// NewUserRepository creates a repository using the "users" collection of db.
func NewUserRepository(db *mongo.Database) *UserRepository {
	return &UserRepository{coll: db.Collection(usersCollection)}
}

// EnsureIndexes creates the unique index on email. Safe to call on every start.
func (r *UserRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("uniq_email"),
	})
	if err != nil {
		return fmt.Errorf("create email index: %w", err)
	}
	return nil
}

// Create inserts a user and sets its ID.
func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	doc := userDocument{
		ID:        bson.NewObjectID(),
		Name:      user.Name,
		Email:     user.Email,
		Password:  user.PasswordHash,
		CreatedAt: user.CreatedAt,
	}
	if _, err := r.coll.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return domain.ErrEmailAlreadyExists
		}
		return fmt.Errorf("insert user: %w", err)
	}
	user.ID = doc.ID.Hex()
	return nil
}

// GetByID returns the user with the given hex ObjectID.
func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, domain.ErrUserNotFound
	}
	return r.findOne(ctx, bson.D{{Key: "_id", Value: oid}})
}

// GetByEmail returns the user with the given (normalized) email.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return r.findOne(ctx, bson.D{{Key: "email", Value: email}})
}

func (r *UserRepository) findOne(ctx context.Context, filter bson.D) (*domain.User, error) {
	var doc userDocument
	if err := r.coll.FindOne(ctx, filter).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("find user: %w", err)
	}
	return doc.toDomain(), nil
}

// List returns all users ordered by creation time.
func (r *UserRepository) List(ctx context.Context) ([]*domain.User, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}, {Key: "_id", Value: 1}})
	cur, err := r.coll.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer cur.Close(ctx)

	var docs []userDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode users: %w", err)
	}
	users := make([]*domain.User, 0, len(docs))
	for i := range docs {
		users = append(users, docs[i].toDomain())
	}
	return users, nil
}

// Update applies the non-nil fields of update and returns the updated user.
func (r *UserRepository) Update(ctx context.Context, id string, update port.UserUpdate) (*domain.User, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, domain.ErrUserNotFound
	}

	set := bson.D{}
	if update.Name != nil {
		set = append(set, bson.E{Key: "name", Value: *update.Name})
	}
	if update.Email != nil {
		set = append(set, bson.E{Key: "email", Value: *update.Email})
	}
	if len(set) == 0 {
		return r.GetByID(ctx, id)
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var doc userDocument
	err = r.coll.FindOneAndUpdate(ctx, bson.D{{Key: "_id", Value: oid}}, bson.D{{Key: "$set", Value: set}}, opts).Decode(&doc)
	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		return nil, domain.ErrUserNotFound
	case mongo.IsDuplicateKeyError(err):
		return nil, domain.ErrEmailAlreadyExists
	case err != nil:
		return nil, fmt.Errorf("update user: %w", err)
	}
	return doc.toDomain(), nil
}

// Delete removes the user with the given ID.
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return domain.ErrUserNotFound
	}
	res, err := r.coll.DeleteOne(ctx, bson.D{{Key: "_id", Value: oid}})
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if res.DeletedCount == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

// Count returns the total number of users.
func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	n, err := r.coll.CountDocuments(ctx, bson.D{})
	if err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return n, nil
}
