package auth

import (
	"context"
	"testing"
	"time"

	zee6dov1 "github.com/DeepakDP5/zee6do-server/gen/zee6do/v1"
	"github.com/DeepakDP5/zee6do-server/pkg/config"
	"github.com/DeepakDP5/zee6do-server/pkg/crypto"
	apperrors "github.com/DeepakDP5/zee6do-server/pkg/errors"
	"github.com/DeepakDP5/zee6do-server/pkg/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newTestHandler(t *testing.T) (*Handler, *mockRepo, *mockUserRepo) {
	t.Helper()
	cfg := &config.Config{JWT: config.JWTConfig{
		Secret:     "test-secret",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 30 * 24 * time.Hour,
	}}
	repo := &mockRepo{}
	userRepo := &mockUserRepo{}
	svc := NewService(repo, userRepo, crypto.NewJWTService(cfg), cfg, zap.NewNop())
	return NewHandler(svc), repo, userRepo
}

func TestHandler_SendOTP_Success(t *testing.T) {
	h, repo, _ := newTestHandler(t)
	repo.On("CreateOTP", mock.Anything, mock.AnythingOfType("*auth.OTPRecord")).Return(nil)

	resp, err := h.SendOTP(context.Background(), &zee6dov1.SendOTPRequest{PhoneNumber: "+1555"})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.OtpId)
	assert.NotNil(t, resp.ExpiresAt)
}

func TestHandler_SendOTP_InvalidInput_MapsToInvalidArgument(t *testing.T) {
	h, _, _ := newTestHandler(t)
	_, err := h.SendOTP(context.Background(), &zee6dov1.SendOTPRequest{PhoneNumber: ""})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestHandler_VerifyOTP_Unauthorized_TokenMismatch(t *testing.T) {
	// Refresh flow maps ErrUnauthorized -> Unauthenticated.
	h, repo, _ := newTestHandler(t)
	// Trigger a device mismatch by producing a valid refresh token but
	// mismatching the stored session fingerprint.
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "test-secret", AccessTTL: time.Minute, RefreshTTL: time.Hour}}
	jwtSvc := crypto.NewJWTService(cfg)
	refresh, err := jwtSvc.GenerateRefreshToken("user-1", crypto.HashFingerprint("legit"))
	require.NoError(t, err)

	session := &Session{
		ID:                bson.NewObjectID(),
		UserID:            "user-1",
		DeviceFingerprint: "legit",
		RefreshTokenHash:  hashRefreshToken(refresh),
		ExpiresAt:         time.Now().Add(time.Hour),
	}
	repo.On("GetSessionByRefreshToken", mock.Anything, mock.Anything).Return(session, nil)

	_, err = h.RefreshToken(context.Background(), &zee6dov1.RefreshTokenRequest{
		RefreshToken:      refresh,
		DeviceFingerprint: "attacker",
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestHandler_RevokeDevice_NotOwner_MapsToPermissionDenied(t *testing.T) {
	h, repo, _ := newTestHandler(t)
	// Simulate authenticated user in context.
	ctx := middleware.ContextWithUserID(context.Background(), "user-1")
	repo.On("GetSessionsByUser", mock.Anything, "user-1").Return([]*Session{}, nil)

	_, err := h.RevokeDevice(ctx, &zee6dov1.RevokeDeviceRequest{DeviceId: bson.NewObjectID().Hex()})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestHandler_ListDevices(t *testing.T) {
	h, repo, _ := newTestHandler(t)
	ctx := middleware.ContextWithUserID(context.Background(), "user-1")
	sessions := []*Session{{
		ID:        bson.NewObjectID(),
		UserID:    "user-1",
		DeviceID:  "iphone",
		CreatedAt: time.Now(),
	}}
	repo.On("GetSessionsByUser", mock.Anything, "user-1").Return(sessions, nil)

	resp, err := h.ListDevices(ctx, &zee6dov1.ListDevicesRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Devices, 1)
	assert.Equal(t, "iphone", resp.Devices[0].Name)
}

func TestHandler_Logout(t *testing.T) {
	h, repo, _ := newTestHandler(t)
	ctx := middleware.ContextWithUserID(context.Background(), "user-1")
	repo.On("RevokeAllSessions", mock.Anything, "user-1").Return(nil)

	_, err := h.Logout(ctx, &zee6dov1.LogoutRequest{})
	require.NoError(t, err)
}

func TestHandler_VerifyOTP_NotFound_MapsToNotFound(t *testing.T) {
	h, repo, _ := newTestHandler(t)
	repo.On("GetOTP", mock.Anything, mock.Anything).
		Return(nil, apperrors.Wrap(apperrors.ErrNotFound, "missing"))
	_, err := h.VerifyOTP(context.Background(), &zee6dov1.VerifyOTPRequest{
		OtpId: bson.NewObjectID().Hex(), Code: "123456", DeviceFingerprint: "dev",
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}
