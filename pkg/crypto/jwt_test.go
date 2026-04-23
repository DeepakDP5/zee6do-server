package crypto

import (
	"context"
	"testing"
	"time"

	"github.com/DeepakDP5/zee6do-server/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestService(t *testing.T, secret string, accessTTL, refreshTTL time.Duration) *JWTService {
	t.Helper()
	return NewJWTService(&config.Config{
		JWT: config.JWTConfig{
			Secret:     secret,
			AccessTTL:  accessTTL,
			RefreshTTL: refreshTTL,
		},
	})
}

func TestJWTService_GenerateAndValidateAccessToken(t *testing.T) {
	svc := newTestService(t, "test-secret", 15*time.Minute, 30*24*time.Hour)
	token, err := svc.GenerateAccessToken("user-1", "devhash")
	require.NoError(t, err)
	require.NotEmpty(t, token)

	userID, err := svc.ValidateToken(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, "user-1", userID)
}

func TestJWTService_GenerateAndParseRefreshClaims(t *testing.T) {
	svc := newTestService(t, "test-secret", 15*time.Minute, 30*24*time.Hour)
	token, err := svc.GenerateRefreshToken("user-2", "devhash-2")
	require.NoError(t, err)

	claims, err := svc.ParseClaims(token)
	require.NoError(t, err)
	assert.Equal(t, "user-2", claims.UserID)
	assert.Equal(t, "devhash-2", claims.DeviceFingerprintHash)
	assert.Equal(t, "user-2", claims.Subject)
}

func TestJWTService_ValidateToken_Errors(t *testing.T) {
	svc := newTestService(t, "secret-a", 15*time.Minute, 30*24*time.Hour)
	other := newTestService(t, "secret-b", 15*time.Minute, 30*24*time.Hour)
	expired := newTestService(t, "secret-a", -1*time.Minute, -1*time.Minute)

	validToken, err := svc.GenerateAccessToken("user-x", "dfh")
	require.NoError(t, err)
	expiredToken, err := expired.GenerateAccessToken("user-x", "dfh")
	require.NoError(t, err)

	cases := []struct {
		name    string
		token   string
		svc     *JWTService
		wantErr bool
	}{
		{"wrong-secret", validToken, other, true},
		{"expired", expiredToken, svc, true},
		{"garbage", "not.a.jwt", svc, true},
		{"empty", "", svc, true},
		{"valid", validToken, svc, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.svc.ValidateToken(context.Background(), tc.token)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestJWTService_ParseClaims_DeviceFingerprintMismatch(t *testing.T) {
	// The JWT itself has no notion of mismatch -- callers compare the claim's
	// fingerprint hash against a freshly-computed hash of the raw fingerprint.
	svc := newTestService(t, "s", 15*time.Minute, 30*24*time.Hour)
	tok, err := svc.GenerateRefreshToken("u", HashFingerprint("device-A"))
	require.NoError(t, err)

	claims, err := svc.ParseClaims(tok)
	require.NoError(t, err)
	assert.False(t, CompareFingerprint("device-B", claims.DeviceFingerprintHash))
	assert.True(t, CompareFingerprint("device-A", claims.DeviceFingerprintHash))
}
