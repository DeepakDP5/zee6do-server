package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/DeepakDP5/zee6do-server/internal/users"
	"github.com/DeepakDP5/zee6do-server/pkg/config"
	"github.com/DeepakDP5/zee6do-server/pkg/crypto"
	apperrors "github.com/DeepakDP5/zee6do-server/pkg/errors"
	"go.uber.org/zap"
)

// Defaults for OTP challenges.
const (
	otpTTL         = 5 * time.Minute
	otpMaxAttempts = 5
)

// Service implements the auth business logic: OTP send/verify, social login,
// JWT issuance, and device session management.
type Service struct {
	repo     Repository
	userRepo users.Repository
	jwt      *crypto.JWTService
	cfg      *config.Config
	logger   *zap.Logger
}

// NewService wires the auth service with its dependencies.
func NewService(
	repo Repository,
	userRepo users.Repository,
	jwtSvc *crypto.JWTService,
	cfg *config.Config,
	logger *zap.Logger,
) *Service {
	return &Service{
		repo:     repo,
		userRepo: userRepo,
		jwt:      jwtSvc,
		cfg:      cfg,
		logger:   logger,
	}
}

// SendOTPResult is returned from SendOTP.
type SendOTPResult struct {
	OTPID     string
	ExpiresAt time.Time
}

// TokenPair bundles the freshly issued JWT tokens plus the authenticated user.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	User         *users.User
	SessionID    string
}

// SendOTP generates an OTP for the given phone number and stores its hash.
// In beta the code is logged instead of delivered via SMS.
func (s *Service) SendOTP(ctx context.Context, phone string) (*SendOTPResult, error) {
	if phone == "" {
		return nil, apperrors.Wrap(apperrors.ErrInvalidInput, "auth.SendOTP: phone required")
	}

	code, err := crypto.GenerateOTP()
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternal, fmt.Sprintf("auth.SendOTP: generate: %v", err))
	}
	hash, err := crypto.HashOTP(code)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternal, fmt.Sprintf("auth.SendOTP: hash: %v", err))
	}

	now := time.Now().UTC()
	record := &OTPRecord{
		PhoneNumber: phone,
		CodeHash:    hash,
		Attempts:    0,
		MaxAttempts: otpMaxAttempts,
		ExpiresAt:   now.Add(otpTTL),
		Verified:    false,
		CreatedAt:   now,
	}
	if err := s.repo.CreateOTP(ctx, record); err != nil {
		return nil, fmt.Errorf("auth.SendOTP: %w", err)
	}

	// Beta: no real SMS -- log the code at info level so QA / dev can use it.
	s.logger.Info("OTP issued (beta: no SMS)",
		zap.String("otp_id", record.ID.Hex()),
		zap.String("phone", phone),
		zap.String("code", code),
		zap.Time("expires_at", record.ExpiresAt),
	)

	return &SendOTPResult{
		OTPID:     record.ID.Hex(),
		ExpiresAt: record.ExpiresAt,
	}, nil
}

// VerifyOTP validates the OTP challenge and, on success, creates or fetches
// the user, issues a JWT pair, and persists a device session.
func (s *Service) VerifyOTP(ctx context.Context, otpID, code, deviceFingerprint string) (*TokenPair, error) {
	if otpID == "" || code == "" || deviceFingerprint == "" {
		return nil, apperrors.Wrap(apperrors.ErrInvalidInput, "auth.VerifyOTP: missing field")
	}

	rec, err := s.repo.GetOTP(ctx, otpID)
	if err != nil {
		return nil, fmt.Errorf("auth.VerifyOTP: %w", err)
	}

	if rec.Verified {
		return nil, apperrors.Wrap(apperrors.ErrInvalidInput, "auth.VerifyOTP: already verified")
	}
	if time.Now().UTC().After(rec.ExpiresAt) {
		return nil, apperrors.Wrap(apperrors.ErrInvalidInput, "auth.VerifyOTP: expired")
	}
	if rec.Attempts >= rec.MaxAttempts {
		return nil, apperrors.Wrap(apperrors.ErrRateLimited, "auth.VerifyOTP: max attempts")
	}

	if !crypto.VerifyOTP(code, rec.CodeHash) {
		if err := s.repo.IncrementOTPAttempts(ctx, otpID); err != nil {
			s.logger.Warn("failed to increment otp attempts", zap.Error(err))
		}
		// After incrementing, check whether we just crossed the threshold.
		if rec.Attempts+1 >= rec.MaxAttempts {
			return nil, apperrors.Wrap(apperrors.ErrRateLimited, "auth.VerifyOTP: max attempts")
		}
		return nil, apperrors.Wrap(apperrors.ErrInvalidInput, "auth.VerifyOTP: invalid code")
	}

	if err := s.repo.MarkOTPVerified(ctx, otpID); err != nil {
		return nil, fmt.Errorf("auth.VerifyOTP: %w", err)
	}

	user, err := s.upsertUserByPhone(ctx, rec.PhoneNumber)
	if err != nil {
		return nil, fmt.Errorf("auth.VerifyOTP: %w", err)
	}

	return s.issueTokenPair(ctx, user, deviceFingerprint)
}

// SocialLogin stubs out social provider verification for beta. The id_token
// is logged and treated as a trusted provider subject so the surrounding
// flow (user upsert, session creation, JWT issuance) can be exercised.
func (s *Service) SocialLogin(ctx context.Context, provider, token, deviceFingerprint string) (*TokenPair, error) {
	if provider == "" || token == "" || deviceFingerprint == "" {
		return nil, apperrors.Wrap(apperrors.ErrInvalidInput, "auth.SocialLogin: missing field")
	}

	s.logger.Info("social login placeholder (beta: no real verification)",
		zap.String("provider", provider),
	)

	// Derive a stable provider-subject from the token so repeated calls with
	// the same token resolve to the same user without real verification.
	socialID := hashTokenForStub(token)

	user, err := s.userRepo.GetBySocialID(ctx, provider, socialID)
	if err != nil {
		if !errors.Is(err, apperrors.ErrNotFound) {
			return nil, fmt.Errorf("auth.SocialLogin: lookup: %w", err)
		}
		user = &users.User{
			SocialIDs: map[string]string{provider: socialID},
		}
		if err := s.userRepo.CreateUser(ctx, user); err != nil {
			return nil, fmt.Errorf("auth.SocialLogin: create: %w", err)
		}
	}

	return s.issueTokenPair(ctx, user, deviceFingerprint)
}

// RefreshToken validates the refresh token, confirms the device fingerprint
// matches the stored session, revokes the old session, and issues a new pair.
func (s *Service) RefreshToken(ctx context.Context, refreshToken, deviceFingerprint string) (*TokenPair, error) {
	if refreshToken == "" || deviceFingerprint == "" {
		return nil, apperrors.Wrap(apperrors.ErrInvalidInput, "auth.RefreshToken: missing field")
	}

	claims, err := s.jwt.ParseClaims(refreshToken)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrUnauthorized, fmt.Sprintf("auth.RefreshToken: %v", err))
	}

	tokenHash := hashRefreshToken(refreshToken)
	session, err := s.repo.GetSessionByRefreshToken(ctx, tokenHash)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrUnauthorized, fmt.Sprintf("auth.RefreshToken: %v", err))
	}

	// Device fingerprint binding:
	// - The raw fingerprint presented now must equal the one stored on the
	//   session (constant-time compare via a hash-based check).
	// - And the hash carried in the refresh token claims must match the
	//   hash of the session's fingerprint.
	storedHash := crypto.HashFingerprint(session.DeviceFingerprint)
	if !crypto.CompareFingerprint(deviceFingerprint, storedHash) {
		return nil, apperrors.Wrap(apperrors.ErrUnauthorized, "auth.RefreshToken: device mismatch")
	}
	if claims.DeviceFingerprintHash != storedHash {
		return nil, apperrors.Wrap(apperrors.ErrUnauthorized, "auth.RefreshToken: token/device mismatch")
	}
	if session.UserID != claims.UserID {
		return nil, apperrors.Wrap(apperrors.ErrUnauthorized, "auth.RefreshToken: token/session user mismatch")
	}

	// Revoke the old session so the refresh token can only be used once.
	if err := s.repo.RevokeSession(ctx, session.ID.Hex()); err != nil {
		return nil, fmt.Errorf("auth.RefreshToken: revoke: %w", err)
	}

	user, err := s.userRepo.GetByID(ctx, session.UserID)
	if err != nil {
		return nil, fmt.Errorf("auth.RefreshToken: load user: %w", err)
	}

	return s.issueTokenPair(ctx, user, deviceFingerprint)
}

// ListDevices returns the user's active sessions.
func (s *Service) ListDevices(ctx context.Context, userID string) ([]*Session, error) {
	if userID == "" {
		return nil, apperrors.Wrap(apperrors.ErrUnauthorized, "auth.ListDevices: unauthenticated")
	}
	return s.repo.GetSessionsByUser(ctx, userID)
}

// RevokeDevice revokes a single session belonging to the calling user.
func (s *Service) RevokeDevice(ctx context.Context, userID, sessionID string) error {
	if userID == "" {
		return apperrors.Wrap(apperrors.ErrUnauthorized, "auth.RevokeDevice: unauthenticated")
	}
	if sessionID == "" {
		return apperrors.Wrap(apperrors.ErrInvalidInput, "auth.RevokeDevice: session id required")
	}

	// Ownership check: load all user sessions and verify the target belongs
	// to the caller. Keeps the repo interface simple; real paging/volume
	// concerns can be addressed when a GetSession(id) accessor is needed.
	sessions, err := s.repo.GetSessionsByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("auth.RevokeDevice: %w", err)
	}
	found := false
	for _, sess := range sessions {
		if sess.ID.Hex() == sessionID {
			found = true
			break
		}
	}
	if !found {
		return apperrors.Wrap(apperrors.ErrForbidden, "auth.RevokeDevice: not owner")
	}

	return s.repo.RevokeSession(ctx, sessionID)
}

// Logout revokes the caller's current session.
func (s *Service) Logout(ctx context.Context, userID, sessionID string) error {
	if userID == "" {
		return apperrors.Wrap(apperrors.ErrUnauthorized, "auth.Logout: unauthenticated")
	}
	if sessionID == "" {
		// No specific session -- revoke all (e.g. logout from all devices).
		return s.repo.RevokeAllSessions(ctx, userID)
	}
	return s.RevokeDevice(ctx, userID, sessionID)
}

// ---- helpers ----

func (s *Service) upsertUserByPhone(ctx context.Context, phone string) (*users.User, error) {
	user, err := s.userRepo.GetByPhone(ctx, phone)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, apperrors.ErrNotFound) {
		return nil, err
	}
	user = &users.User{Phone: phone}
	if err := s.userRepo.CreateUser(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Service) issueTokenPair(ctx context.Context, user *users.User, deviceFingerprint string) (*TokenPair, error) {
	userID := user.ID.Hex()
	dfh := crypto.HashFingerprint(deviceFingerprint)

	access, err := s.jwt.GenerateAccessToken(userID, dfh)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternal, fmt.Sprintf("auth.issueTokenPair: access: %v", err))
	}
	refresh, err := s.jwt.GenerateRefreshToken(userID, dfh)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternal, fmt.Sprintf("auth.issueTokenPair: refresh: %v", err))
	}

	now := time.Now().UTC()
	session := &Session{
		UserID:            userID,
		DeviceFingerprint: deviceFingerprint,
		RefreshTokenHash:  hashRefreshToken(refresh),
		CreatedAt:         now,
		ExpiresAt:         now.Add(s.cfg.JWT.RefreshTTL),
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("auth.issueTokenPair: create session: %w", err)
	}

	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		User:         user,
		SessionID:    session.ID.Hex(),
	}, nil
}

// hashRefreshToken returns a deterministic hash of the refresh token for
// storage/lookup. We never store the raw token; SHA-256 is sufficient because
// the token itself has high entropy.
func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// hashTokenForStub produces a stable fake social-provider subject for beta.
func hashTokenForStub(token string) string {
	sum := sha256.Sum256([]byte("social-stub:" + token))
	return hex.EncodeToString(sum[:])
}
