package migrations

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// migration002AuthIndexes creates the indexes required by the auth module.
//
// sessions collection:
//   - user_id: for per-user lookups and revocation sweeps.
//   - (user_id, revoked): compound index for active-session queries.
//   - expires_at: TTL index so expired sessions auto-purge.
//
// otp_records collection:
//   - expires_at: TTL index matching the 5-minute OTP lifetime.
//   - phone_number: for per-phone lookup / future rate limiting.
//
// users collection:
//   - phone: unique + sparse so at most one account per phone number but
//     multiple social-only (phone-less) users can coexist.
//   - email: unique + sparse so empty emails don't collide.
func migration002AuthIndexes(ctx context.Context, db *mongo.Database) error {
	sessions := db.Collection("sessions")
	sessionIdx := []mongo.IndexModel{
		{Keys: bson.D{{Key: "user_id", Value: 1}}},
		{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "revoked", Value: 1}}},
		{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
	}
	if _, err := sessions.Indexes().CreateMany(ctx, sessionIdx); err != nil {
		return fmt.Errorf("create sessions indexes: %w", err)
	}

	otp := db.Collection("otp_records")
	otpIdx := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
		{Keys: bson.D{{Key: "phone_number", Value: 1}}},
	}
	if _, err := otp.Indexes().CreateMany(ctx, otpIdx); err != nil {
		return fmt.Errorf("create otp_records indexes: %w", err)
	}

	users := db.Collection("users")
	userIdx := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "phone", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
		{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
	}
	if _, err := users.Indexes().CreateMany(ctx, userIdx); err != nil {
		return fmt.Errorf("create users indexes: %w", err)
	}

	return nil
}
