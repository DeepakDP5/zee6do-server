// Package crypto provides cryptographic primitives for the auth module:
// JWT generation/validation, device fingerprint hashing, and OTP generation.
package crypto

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/DeepakDP5/zee6do-server/pkg/config"
	"github.com/golang-jwt/jwt/v5"
)

// Claims are the custom JWT claims used by the zee6do auth module.
// UserID identifies the authenticated user; DeviceFingerprintHash binds the
// token to a specific device (SHA-256 hex of the raw fingerprint).
type Claims struct {
	UserID                string `json:"user_id"`
	DeviceFingerprintHash string `json:"dfh,omitempty"`
	jwt.RegisteredClaims
}

// JWTService signs and validates JWTs using the HS256 algorithm.
// It implements middleware.JWTValidator so it can be plugged directly into
// the gRPC auth interceptor.
type JWTService struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewJWTService builds a JWTService from application config.
func NewJWTService(cfg *config.Config) *JWTService {
	return &JWTService{
		secret:     []byte(cfg.JWT.Secret),
		accessTTL:  cfg.JWT.AccessTTL,
		refreshTTL: cfg.JWT.RefreshTTL,
	}
}

// GenerateAccessToken issues a short-lived access token bound to the given
// user and device fingerprint hash.
func (s *JWTService) GenerateAccessToken(userID, deviceFingerprintHash string) (string, error) {
	return s.sign(userID, deviceFingerprintHash, s.accessTTL)
}

// GenerateRefreshToken issues a long-lived refresh token bound to the given
// user and device fingerprint hash.
func (s *JWTService) GenerateRefreshToken(userID, deviceFingerprintHash string) (string, error) {
	return s.sign(userID, deviceFingerprintHash, s.refreshTTL)
}

func (s *JWTService) sign(userID, deviceFingerprintHash string, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID:                userID,
		DeviceFingerprintHash: deviceFingerprintHash,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Subject:   userID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("crypto.JWTService.sign: %w", err)
	}
	return signed, nil
}

// ValidateToken implements middleware.JWTValidator. It verifies the token
// signature and expiry using the configured secret and returns the embedded
// user ID on success.
func (s *JWTService) ValidateToken(_ context.Context, token string) (string, error) {
	claims, err := s.ParseClaims(token)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}

// ParseClaims parses and validates the given JWT and returns the full claims
// struct, including the device fingerprint hash. Used by the refresh flow.
func (s *JWTService) ParseClaims(token string) (*Claims, error) {
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("crypto.JWTService.ParseClaims: %w", err)
	}
	if !parsed.Valid {
		return nil, errors.New("crypto.JWTService.ParseClaims: token invalid")
	}
	return claims, nil
}
