package auth

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"github.com/DeepakDP5/zee6do-server/internal/users"
	"github.com/DeepakDP5/zee6do-server/pkg/config"
	"github.com/DeepakDP5/zee6do-server/pkg/crypto"
	apperrors "github.com/DeepakDP5/zee6do-server/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.uber.org/zap"
)

// ---- mocks ----

type mockRepo struct{ mock.Mock }

func (m *mockRepo) CreateOTP(ctx context.Context, r *OTPRecord) error {
	args := m.Called(ctx, r)
	return args.Error(0)
}
func (m *mockRepo) GetOTP(ctx context.Context, id string) (*OTPRecord, error) {
	args := m.Called(ctx, id)
	rec, _ := args.Get(0).(*OTPRecord)
	return rec, args.Error(1)
}
func (m *mockRepo) IncrementOTPAttempts(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockRepo) MarkOTPVerified(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockRepo) CreateSession(ctx context.Context, s *Session) error {
	args := m.Called(ctx, s)
	// Mimic repo by allocating an ID so handler/service see a non-zero ID.
	if s.ID.IsZero() {
		s.ID = bson.NewObjectID()
	}
	return args.Error(0)
}
func (m *mockRepo) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	args := m.Called(ctx, sessionID)
	s, _ := args.Get(0).(*Session)
	return s, args.Error(1)
}
func (m *mockRepo) GetSessionsByUser(ctx context.Context, userID string) ([]*Session, error) {
	args := m.Called(ctx, userID)
	s, _ := args.Get(0).([]*Session)
	return s, args.Error(1)
}
func (m *mockRepo) GetSessionByRefreshToken(ctx context.Context, tokenHash string) (*Session, error) {
	args := m.Called(ctx, tokenHash)
	s, _ := args.Get(0).(*Session)
	return s, args.Error(1)
}
func (m *mockRepo) RevokeSession(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockRepo) RevokeAllSessions(ctx context.Context, userID string) error {
	return m.Called(ctx, userID).Error(0)
}

type mockUserRepo struct{ mock.Mock }

func (m *mockUserRepo) CreateUser(ctx context.Context, u *users.User) error {
	args := m.Called(ctx, u)
	if u.ID.IsZero() {
		u.ID = bson.NewObjectID()
	}
	return args.Error(0)
}
func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*users.User, error) {
	args := m.Called(ctx, id)
	u, _ := args.Get(0).(*users.User)
	return u, args.Error(1)
}
func (m *mockUserRepo) GetByPhone(ctx context.Context, phone string) (*users.User, error) {
	args := m.Called(ctx, phone)
	u, _ := args.Get(0).(*users.User)
	return u, args.Error(1)
}
func (m *mockUserRepo) GetBySocialID(ctx context.Context, p, id string) (*users.User, error) {
	args := m.Called(ctx, p, id)
	u, _ := args.Get(0).(*users.User)
	return u, args.Error(1)
}

// ---- helpers ----

func newTestService(t *testing.T) (*Service, *mockRepo, *mockUserRepo) {
	t.Helper()
	cfg := &config.Config{JWT: config.JWTConfig{
		Secret:     "test-secret",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 30 * 24 * time.Hour,
	}}
	repo := &mockRepo{}
	userRepo := &mockUserRepo{}
	jwtSvc := crypto.NewJWTService(cfg)
	return NewService(repo, userRepo, jwtSvc, cfg, zap.NewNop()), repo, userRepo
}

// ---- tests ----

func TestSendOTP_StoresRecordAndReturnsExpiry(t *testing.T) {
	svc, repo, _ := newTestService(t)
	repo.On("CreateOTP", mock.Anything, mock.MatchedBy(func(r *OTPRecord) bool {
		return r.PhoneNumber == "+14155550123" &&
			r.MaxAttempts == otpMaxAttempts &&
			r.Attempts == 0 &&
			!r.Verified &&
			r.CodeHash != "" &&
			r.ExpiresAt.After(time.Now())
	})).Return(nil).Run(func(args mock.Arguments) {
		r := args.Get(1).(*OTPRecord)
		if r.ID.IsZero() {
			r.ID = bson.NewObjectID()
		}
	})

	res, err := svc.SendOTP(context.Background(), "+14155550123")
	require.NoError(t, err)
	require.NotEmpty(t, res.OTPID)
	assert.True(t, res.ExpiresAt.After(time.Now()))
	repo.AssertExpectations(t)
}

func TestSendOTP_EmptyPhone(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.SendOTP(context.Background(), "")
	assert.ErrorIs(t, err, apperrors.ErrInvalidInput)
}

func TestVerifyOTP_ValidCode(t *testing.T) {
	svc, repo, userRepo := newTestService(t)

	code := "123456"
	hash, err := crypto.HashOTP(code)
	require.NoError(t, err)

	otpID := bson.NewObjectID()
	rec := &OTPRecord{
		ID:          otpID,
		PhoneNumber: "+1555",
		CodeHash:    hash,
		Attempts:    0,
		MaxAttempts: otpMaxAttempts,
		ExpiresAt:   time.Now().Add(1 * time.Minute),
	}
	repo.On("GetOTP", mock.Anything, otpID.Hex()).Return(rec, nil)
	repo.On("MarkOTPVerified", mock.Anything, otpID.Hex()).Return(nil)

	userRepo.On("GetByPhone", mock.Anything, "+1555").
		Return(nil, apperrors.Wrap(apperrors.ErrNotFound, "missing"))
	userRepo.On("CreateUser", mock.Anything, mock.MatchedBy(func(u *users.User) bool {
		return u.Phone == "+1555"
	})).Return(nil)

	repo.On("CreateSession", mock.Anything, mock.AnythingOfType("*auth.Session")).Return(nil)

	pair, err := svc.VerifyOTP(context.Background(), otpID.Hex(), code, "device-xyz")
	require.NoError(t, err)
	require.NotEmpty(t, pair.AccessToken)
	require.NotEmpty(t, pair.RefreshToken)
	require.NotNil(t, pair.User)
	repo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestVerifyOTP_WrongCode_Increments(t *testing.T) {
	svc, repo, _ := newTestService(t)
	hash, _ := crypto.HashOTP("123456")
	otpID := bson.NewObjectID()
	rec := &OTPRecord{
		ID: otpID, CodeHash: hash, Attempts: 1, MaxAttempts: otpMaxAttempts,
		ExpiresAt: time.Now().Add(time.Minute),
	}
	repo.On("GetOTP", mock.Anything, otpID.Hex()).Return(rec, nil)
	repo.On("IncrementOTPAttempts", mock.Anything, otpID.Hex()).Return(nil)

	_, err := svc.VerifyOTP(context.Background(), otpID.Hex(), "000000", "dev")
	assert.ErrorIs(t, err, apperrors.ErrInvalidInput)
	repo.AssertExpectations(t)
}

func TestVerifyOTP_Expired(t *testing.T) {
	svc, repo, _ := newTestService(t)
	otpID := bson.NewObjectID()
	rec := &OTPRecord{
		ID: otpID, CodeHash: "x", MaxAttempts: otpMaxAttempts,
		ExpiresAt: time.Now().Add(-time.Minute),
	}
	repo.On("GetOTP", mock.Anything, otpID.Hex()).Return(rec, nil)

	_, err := svc.VerifyOTP(context.Background(), otpID.Hex(), "123456", "dev")
	assert.ErrorIs(t, err, apperrors.ErrInvalidInput)
}

func TestVerifyOTP_AlreadyVerified(t *testing.T) {
	svc, repo, _ := newTestService(t)
	otpID := bson.NewObjectID()
	rec := &OTPRecord{
		ID: otpID, CodeHash: "x", MaxAttempts: otpMaxAttempts, Verified: true,
		ExpiresAt: time.Now().Add(time.Minute),
	}
	repo.On("GetOTP", mock.Anything, otpID.Hex()).Return(rec, nil)
	_, err := svc.VerifyOTP(context.Background(), otpID.Hex(), "123456", "dev")
	assert.ErrorIs(t, err, apperrors.ErrInvalidInput)
}

// When two concurrent VerifyOTP calls race, the atomic MarkOTPVerified in
// the repo rejects the loser (MatchedCount == 0 -> ErrNotFound). The
// service must surface that error instead of minting tokens.
func TestVerifyOTP_Race_LoserGetsErrorFromMarkVerified(t *testing.T) {
	svc, repo, _ := newTestService(t)
	code := "123456"
	hash, err := crypto.HashOTP(code)
	require.NoError(t, err)

	otpID := bson.NewObjectID()
	rec := &OTPRecord{
		ID:          otpID,
		PhoneNumber: "+1555",
		CodeHash:    hash,
		MaxAttempts: otpMaxAttempts,
		ExpiresAt:   time.Now().Add(time.Minute),
	}
	repo.On("GetOTP", mock.Anything, otpID.Hex()).Return(rec, nil)
	// Simulate the atomic update finding no document because another
	// request already flipped verified=true.
	repo.On("MarkOTPVerified", mock.Anything, otpID.Hex()).
		Return(apperrors.Wrap(apperrors.ErrNotFound, "race lost"))

	_, err = svc.VerifyOTP(context.Background(), otpID.Hex(), code, "dev")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
}

func TestVerifyOTP_MaxAttempts_RateLimited(t *testing.T) {
	svc, repo, _ := newTestService(t)
	otpID := bson.NewObjectID()
	rec := &OTPRecord{
		ID: otpID, CodeHash: "x", Attempts: 5, MaxAttempts: otpMaxAttempts,
		ExpiresAt: time.Now().Add(time.Minute),
	}
	repo.On("GetOTP", mock.Anything, otpID.Hex()).Return(rec, nil)
	_, err := svc.VerifyOTP(context.Background(), otpID.Hex(), "123456", "dev")
	assert.ErrorIs(t, err, apperrors.ErrRateLimited)
}

func TestVerifyOTP_WrongCode_JustCrossesMax_RateLimited(t *testing.T) {
	svc, repo, _ := newTestService(t)
	hash, _ := crypto.HashOTP("123456")
	otpID := bson.NewObjectID()
	// Attempts == max - 1: wrong code should bump us over and return RateLimited.
	rec := &OTPRecord{
		ID: otpID, CodeHash: hash, Attempts: otpMaxAttempts - 1, MaxAttempts: otpMaxAttempts,
		ExpiresAt: time.Now().Add(time.Minute),
	}
	repo.On("GetOTP", mock.Anything, otpID.Hex()).Return(rec, nil)
	repo.On("IncrementOTPAttempts", mock.Anything, otpID.Hex()).Return(nil)

	_, err := svc.VerifyOTP(context.Background(), otpID.Hex(), "000000", "dev")
	assert.ErrorIs(t, err, apperrors.ErrRateLimited)
}

func TestRefreshToken_DeviceMismatch_Unauthorized(t *testing.T) {
	svc, repo, _ := newTestService(t)
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "test-secret", AccessTTL: time.Minute, RefreshTTL: time.Hour}}
	jwtSvc := crypto.NewJWTService(cfg)

	rawDevice := "device-legit"
	dfh := crypto.HashFingerprint(rawDevice)
	userID := bson.NewObjectID().Hex()
	refresh, err := jwtSvc.GenerateRefreshToken(userID, dfh)
	require.NoError(t, err)

	session := &Session{
		ID:                bson.NewObjectID(),
		UserID:            userID,
		DeviceFingerprint: rawDevice,
		RefreshTokenHash:  hashRefreshToken(refresh),
		ExpiresAt:         time.Now().Add(time.Hour),
	}
	repo.On("GetSessionByRefreshToken", mock.Anything, mock.Anything).Return(session, nil)

	_, err = svc.RefreshToken(context.Background(), refresh, "device-attacker")
	assert.ErrorIs(t, err, apperrors.ErrUnauthorized)
}

func TestRefreshToken_HappyPath(t *testing.T) {
	svc, repo, userRepo := newTestService(t)
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "test-secret", AccessTTL: time.Minute, RefreshTTL: time.Hour}}
	jwtSvc := crypto.NewJWTService(cfg)

	rawDevice := "device-legit"
	dfh := crypto.HashFingerprint(rawDevice)
	userOID := bson.NewObjectID()
	userID := userOID.Hex()
	refresh, err := jwtSvc.GenerateRefreshToken(userID, dfh)
	require.NoError(t, err)

	sessionID := bson.NewObjectID()
	session := &Session{
		ID:                sessionID,
		UserID:            userID,
		DeviceFingerprint: rawDevice,
		RefreshTokenHash:  hashRefreshToken(refresh),
		ExpiresAt:         time.Now().Add(time.Hour),
	}
	repo.On("GetSessionByRefreshToken", mock.Anything, mock.Anything).Return(session, nil)
	repo.On("RevokeSession", mock.Anything, sessionID.Hex()).Return(nil)
	userRepo.On("GetByID", mock.Anything, userID).Return(&users.User{ID: userOID, Phone: "+1"}, nil)
	repo.On("CreateSession", mock.Anything, mock.AnythingOfType("*auth.Session")).Return(nil)

	pair, err := svc.RefreshToken(context.Background(), refresh, rawDevice)
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
}

func TestRefreshToken_InvalidToken(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.RefreshToken(context.Background(), "not.a.jwt", "dev")
	assert.ErrorIs(t, err, apperrors.ErrUnauthorized)
}

func TestListDevices(t *testing.T) {
	svc, repo, _ := newTestService(t)
	sessions := []*Session{{ID: bson.NewObjectID(), UserID: "u1"}}
	repo.On("GetSessionsByUser", mock.Anything, "u1").Return(sessions, nil)
	got, err := svc.ListDevices(context.Background(), "u1")
	require.NoError(t, err)
	assert.Equal(t, sessions, got)
}

func TestListDevices_Unauthenticated(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.ListDevices(context.Background(), "")
	assert.ErrorIs(t, err, apperrors.ErrUnauthorized)
}

func TestRevokeDevice_HappyPath(t *testing.T) {
	svc, repo, _ := newTestService(t)
	sid := bson.NewObjectID()
	repo.On("GetSession", mock.Anything, sid.Hex()).Return(&Session{ID: sid, UserID: "u1"}, nil)
	repo.On("RevokeSession", mock.Anything, sid.Hex()).Return(nil)
	require.NoError(t, svc.RevokeDevice(context.Background(), "u1", sid.Hex()))
}

func TestRevokeDevice_NotOwner(t *testing.T) {
	svc, repo, _ := newTestService(t)
	sid := bson.NewObjectID()
	// Session exists but belongs to a different user.
	repo.On("GetSession", mock.Anything, sid.Hex()).Return(&Session{ID: sid, UserID: "other"}, nil)
	err := svc.RevokeDevice(context.Background(), "u1", sid.Hex())
	assert.ErrorIs(t, err, apperrors.ErrForbidden)
}

func TestRevokeDevice_MissingSession_MapsToForbidden(t *testing.T) {
	// Unknown session IDs must not leak existence; we return Forbidden
	// just like the "wrong owner" path.
	svc, repo, _ := newTestService(t)
	sid := bson.NewObjectID()
	repo.On("GetSession", mock.Anything, sid.Hex()).
		Return(nil, apperrors.Wrap(apperrors.ErrNotFound, "missing"))
	err := svc.RevokeDevice(context.Background(), "u1", sid.Hex())
	assert.ErrorIs(t, err, apperrors.ErrForbidden)
}

func TestLogout_AllSessionsWhenNoID(t *testing.T) {
	svc, repo, _ := newTestService(t)
	repo.On("RevokeAllSessions", mock.Anything, "u1").Return(nil)
	require.NoError(t, svc.Logout(context.Background(), "u1", ""))
	repo.AssertExpectations(t)
}

func TestLogout_SpecificSession(t *testing.T) {
	svc, repo, _ := newTestService(t)
	sid := bson.NewObjectID()
	repo.On("GetSession", mock.Anything, sid.Hex()).Return(&Session{ID: sid, UserID: "u1"}, nil)
	repo.On("RevokeSession", mock.Anything, sid.Hex()).Return(nil)
	require.NoError(t, svc.Logout(context.Background(), "u1", sid.Hex()))
}

func TestSocialLogin_NewUser(t *testing.T) {
	svc, repo, userRepo := newTestService(t)
	userRepo.On("GetBySocialID", mock.Anything, "SOCIAL_PROVIDER_GOOGLE", mock.Anything).
		Return(nil, apperrors.Wrap(apperrors.ErrNotFound, "missing"))
	userRepo.On("CreateUser", mock.Anything, mock.AnythingOfType("*users.User")).Return(nil)
	repo.On("CreateSession", mock.Anything, mock.AnythingOfType("*auth.Session")).Return(nil)

	pair, err := svc.SocialLogin(context.Background(), "SOCIAL_PROVIDER_GOOGLE", "id-token", "dev-1")
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
}

func TestSocialLogin_LookupError(t *testing.T) {
	svc, _, userRepo := newTestService(t)
	userRepo.On("GetBySocialID", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, stderrors.New("boom"))
	_, err := svc.SocialLogin(context.Background(), "SOCIAL_PROVIDER_GOOGLE", "id-token", "dev-1")
	assert.Error(t, err)
}
