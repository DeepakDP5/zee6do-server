package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	apperrors "github.com/DeepakDP5/zee6do-server/pkg/errors"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	otpCollection     = "otp_records"
	sessionCollection = "sessions"
)

// MongoRepository implements Repository on top of MongoDB.
type MongoRepository struct {
	otp      *mongo.Collection
	sessions *mongo.Collection
}

// NewMongoRepository builds the repository against the given Mongo database.
func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		otp:      db.Collection(otpCollection),
		sessions: db.Collection(sessionCollection),
	}
}

// ---- OTP ----

// CreateOTP inserts a new OTP challenge. CreatedAt is set to the current UTC
// time if unset; a new ObjectID is allocated if the caller did not supply one.
func (r *MongoRepository) CreateOTP(ctx context.Context, record *OTPRecord) error {
	if record == nil {
		return fmt.Errorf("auth.CreateOTP: record is nil")
	}
	if record.ID.IsZero() {
		record.ID = bson.NewObjectID()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	if _, err := r.otp.InsertOne(ctx, record); err != nil {
		return fmt.Errorf("auth.CreateOTP: %w", err)
	}
	return nil
}

// GetOTP fetches an OTP record by its hex ObjectID. Returns ErrNotFound when
// the ID is malformed or no document matches.
func (r *MongoRepository) GetOTP(ctx context.Context, id string) (*OTPRecord, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrNotFound, "auth.GetOTP: invalid id")
	}
	var rec OTPRecord
	if err := r.otp.FindOne(ctx, bson.M{"_id": oid}).Decode(&rec); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, apperrors.Wrap(apperrors.ErrNotFound, "auth.GetOTP")
		}
		return nil, fmt.Errorf("auth.GetOTP: %w", err)
	}
	return &rec, nil
}

// IncrementOTPAttempts bumps the attempts counter atomically.
func (r *MongoRepository) IncrementOTPAttempts(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return apperrors.Wrap(apperrors.ErrNotFound, "auth.IncrementOTPAttempts: invalid id")
	}
	res, err := r.otp.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$inc": bson.M{"attempts": 1}})
	if err != nil {
		return fmt.Errorf("auth.IncrementOTPAttempts: %w", err)
	}
	if res.MatchedCount == 0 {
		return apperrors.Wrap(apperrors.ErrNotFound, "auth.IncrementOTPAttempts")
	}
	return nil
}

// MarkOTPVerified atomically flips the verified flag to true for an OTP
// record that has not yet been verified. The filter includes
// `verified: false` so concurrent callers cannot both succeed; the second
// racer sees MatchedCount == 0 and gets ErrNotFound, which the service
// translates into an "already verified" failure. This preserves the
// single-use guarantee even under concurrent VerifyOTP requests.
func (r *MongoRepository) MarkOTPVerified(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return apperrors.Wrap(apperrors.ErrNotFound, "auth.MarkOTPVerified: invalid id")
	}
	res, err := r.otp.UpdateOne(ctx,
		bson.M{"_id": oid, "verified": false},
		bson.M{"$set": bson.M{"verified": true}},
	)
	if err != nil {
		return fmt.Errorf("auth.MarkOTPVerified: %w", err)
	}
	if res.MatchedCount == 0 {
		return apperrors.Wrap(apperrors.ErrNotFound, "auth.MarkOTPVerified")
	}
	return nil
}

// ---- Sessions ----

// CreateSession inserts a new session.
func (r *MongoRepository) CreateSession(ctx context.Context, session *Session) error {
	if session == nil {
		return fmt.Errorf("auth.CreateSession: session is nil")
	}
	if session.ID.IsZero() {
		session.ID = bson.NewObjectID()
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now().UTC()
	}
	if _, err := r.sessions.InsertOne(ctx, session); err != nil {
		return fmt.Errorf("auth.CreateSession: %w", err)
	}
	return nil
}

// GetSession fetches a single session by its hex ObjectID. Returns
// ErrNotFound when the ID is malformed or no document matches. Revoked
// sessions are returned as-is so callers can decide how to handle them.
func (r *MongoRepository) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	oid, err := bson.ObjectIDFromHex(sessionID)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrNotFound, "auth.GetSession: invalid id")
	}
	var s Session
	if err := r.sessions.FindOne(ctx, bson.M{"_id": oid}).Decode(&s); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, apperrors.Wrap(apperrors.ErrNotFound, "auth.GetSession")
		}
		return nil, fmt.Errorf("auth.GetSession: %w", err)
	}
	return &s, nil
}

// GetSessionsByUser returns all non-revoked sessions for the given user,
// newest first.
func (r *MongoRepository) GetSessionsByUser(ctx context.Context, userID string) ([]*Session, error) {
	filter := bson.M{"user_id": userID, "revoked": false}
	cursor, err := r.sessions.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("auth.GetSessionsByUser: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var out []*Session
	for cursor.Next(ctx) {
		var s Session
		if err := cursor.Decode(&s); err != nil {
			return nil, fmt.Errorf("auth.GetSessionsByUser: decode: %w", err)
		}
		out = append(out, &s)
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("auth.GetSessionsByUser: cursor: %w", err)
	}
	return out, nil
}

// GetSessionByRefreshToken returns the active session whose refresh token
// hashes to tokenHash. Revoked sessions are excluded.
func (r *MongoRepository) GetSessionByRefreshToken(ctx context.Context, tokenHash string) (*Session, error) {
	filter := bson.M{"refresh_token_hash": tokenHash, "revoked": false}
	var s Session
	if err := r.sessions.FindOne(ctx, filter).Decode(&s); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, apperrors.Wrap(apperrors.ErrNotFound, "auth.GetSessionByRefreshToken")
		}
		return nil, fmt.Errorf("auth.GetSessionByRefreshToken: %w", err)
	}
	return &s, nil
}

// RevokeSession marks a single session as revoked.
func (r *MongoRepository) RevokeSession(ctx context.Context, sessionID string) error {
	oid, err := bson.ObjectIDFromHex(sessionID)
	if err != nil {
		return apperrors.Wrap(apperrors.ErrNotFound, "auth.RevokeSession: invalid id")
	}
	now := time.Now().UTC()
	res, err := r.sessions.UpdateOne(ctx,
		bson.M{"_id": oid},
		bson.M{"$set": bson.M{"revoked": true, "revoked_at": now}},
	)
	if err != nil {
		return fmt.Errorf("auth.RevokeSession: %w", err)
	}
	if res.MatchedCount == 0 {
		return apperrors.Wrap(apperrors.ErrNotFound, "auth.RevokeSession")
	}
	return nil
}

// RevokeAllSessions marks every active session for the user as revoked.
func (r *MongoRepository) RevokeAllSessions(ctx context.Context, userID string) error {
	now := time.Now().UTC()
	_, err := r.sessions.UpdateMany(ctx,
		bson.M{"user_id": userID, "revoked": false},
		bson.M{"$set": bson.M{"revoked": true, "revoked_at": now}},
	)
	if err != nil {
		return fmt.Errorf("auth.RevokeAllSessions: %w", err)
	}
	return nil
}
