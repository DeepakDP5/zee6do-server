// Package users is a minimal stub of the users module. It provides the
// data model and repository interface required by the auth flow (phone
// lookup, social-id lookup, create) without a full service layer yet.
// A complete users module will land in a later milestone.
package users

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// User is the minimal user record stored in MongoDB.
type User struct {
	ID        bson.ObjectID     `bson:"_id,omitempty"`
	Phone     string            `bson:"phone"`
	Email     string            `bson:"email,omitempty"`
	Name      string            `bson:"name,omitempty"`
	SocialIDs map[string]string `bson:"social_ids,omitempty"` // provider -> id
	CreatedAt time.Time         `bson:"created_at"`
	UpdatedAt time.Time         `bson:"updated_at"`
}
