package auth

import "context"

// Repository is the persistence interface for OTP challenges and device
// sessions used by the auth module.
type Repository interface {
	CreateOTP(ctx context.Context, record *OTPRecord) error
	GetOTP(ctx context.Context, id string) (*OTPRecord, error)
	IncrementOTPAttempts(ctx context.Context, id string) error
	MarkOTPVerified(ctx context.Context, id string) error

	CreateSession(ctx context.Context, session *Session) error
	GetSessionsByUser(ctx context.Context, userID string) ([]*Session, error)
	GetSessionByRefreshToken(ctx context.Context, tokenHash string) (*Session, error)
	RevokeSession(ctx context.Context, sessionID string) error
	RevokeAllSessions(ctx context.Context, userID string) error
}
