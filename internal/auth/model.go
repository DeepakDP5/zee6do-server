// Package auth implements the zee6do authentication module: OTP send/verify,
// social login, JWT issuance, and device session management. Business logic
// lives in Service; persistence is abstracted via Repository; the gRPC
// boundary is handled by Handler.
package auth

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Session represents an authenticated device/session row.
//
// The raw DeviceFingerprint is stored so we can re-hash and compare it on
// refresh (the hash in the JWT only tells us what it should be, not what the
// device actually presented on first login). In production this field should
// be encrypted at rest -- beta stores it plaintext.
type Session struct {
	ID                bson.ObjectID `bson:"_id,omitempty"`
	UserID            string        `bson:"user_id"`
	DeviceID          string        `bson:"device_id,omitempty"`
	DeviceFingerprint string        `bson:"device_fingerprint"`
	RefreshTokenHash  string        `bson:"refresh_token_hash"`
	CreatedAt         time.Time     `bson:"created_at"`
	ExpiresAt         time.Time     `bson:"expires_at"`
	Revoked           bool          `bson:"revoked"`
	RevokedAt         *time.Time    `bson:"revoked_at,omitempty"`
}

// OTPRecord is a single OTP challenge issued to a phone number.
type OTPRecord struct {
	ID          bson.ObjectID `bson:"_id,omitempty"`
	PhoneNumber string        `bson:"phone_number"`
	CodeHash    string        `bson:"code_hash"`
	Attempts    int           `bson:"attempts"`
	MaxAttempts int           `bson:"max_attempts"`
	ExpiresAt   time.Time     `bson:"expires_at"`
	Verified    bool          `bson:"verified"`
	CreatedAt   time.Time     `bson:"created_at"`
}
