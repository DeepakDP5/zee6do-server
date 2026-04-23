package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	apperrors "github.com/DeepakDP5/zee6do-server/pkg/errors"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const collectionName = "users"

// MongoRepository is the MongoDB-backed implementation of Repository.
type MongoRepository struct {
	coll *mongo.Collection
}

// NewMongoRepository wires the repository against the given Mongo database.
func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{coll: db.Collection(collectionName)}
}

// CreateUser inserts a new user. If ID is the zero value a new ObjectID is
// allocated. CreatedAt / UpdatedAt are set to the current UTC time.
func (r *MongoRepository) CreateUser(ctx context.Context, user *User) error {
	if user == nil {
		return fmt.Errorf("users.CreateUser: user is nil")
	}
	now := time.Now().UTC()
	if user.ID.IsZero() {
		user.ID = bson.NewObjectID()
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now

	if _, err := r.coll.InsertOne(ctx, user); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return apperrors.Wrap(apperrors.ErrAlreadyExists, "users.CreateUser")
		}
		return fmt.Errorf("users.CreateUser: %w", err)
	}
	return nil
}

// GetByID looks up a user by hex ObjectID. Returns ErrNotFound when no user
// with the given ID exists or when the ID is not a valid ObjectID.
func (r *MongoRepository) GetByID(ctx context.Context, id string) (*User, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrNotFound, "users.GetByID: invalid id")
	}
	return r.findOne(ctx, bson.M{"_id": oid})
}

// GetByPhone returns the user matching the given phone number, or ErrNotFound.
func (r *MongoRepository) GetByPhone(ctx context.Context, phone string) (*User, error) {
	return r.findOne(ctx, bson.M{"phone": phone})
}

// GetBySocialID returns the user linked to the given provider/social-id pair.
func (r *MongoRepository) GetBySocialID(ctx context.Context, provider, socialID string) (*User, error) {
	key := "social_ids." + provider
	return r.findOne(ctx, bson.M{key: socialID})
}

func (r *MongoRepository) findOne(ctx context.Context, filter bson.M) (*User, error) {
	var u User
	if err := r.coll.FindOne(ctx, filter).Decode(&u); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, apperrors.Wrap(apperrors.ErrNotFound, "users.findOne")
		}
		return nil, fmt.Errorf("users.findOne: %w", err)
	}
	return &u, nil
}
