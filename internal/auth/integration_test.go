//go:build integration

package auth

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	zee6dov1 "github.com/DeepakDP5/zee6do-server/gen/zee6do/v1"
	"github.com/DeepakDP5/zee6do-server/internal/users"
	"github.com/DeepakDP5/zee6do-server/pkg/config"
	"github.com/DeepakDP5/zee6do-server/pkg/crypto"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestAuthIntegration exercises the full gRPC round-trip for the auth
// service: SendOTP -> VerifyOTP -> RefreshToken -> Logout. It requires a
// running MongoDB instance pointed to by ZEE6DO_TEST_MONGODB_URI. Without
// that env var the test is skipped.
func TestAuthIntegration(t *testing.T) {
	uri := os.Getenv("ZEE6DO_TEST_MONGODB_URI")
	if uri == "" {
		t.Skip("requires MongoDB: set ZEE6DO_TEST_MONGODB_URI")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mongoClient, err := mongo.Connect(options.Client().ApplyURI(uri))
	require.NoError(t, err)
	defer func() { _ = mongoClient.Disconnect(context.Background()) }()

	dbName := "zee6do_test_auth_integration"
	db := mongoClient.Database(dbName)
	defer func() { _ = db.Drop(context.Background()) }()

	cfg := &config.Config{JWT: config.JWTConfig{
		Secret:     "integration-secret",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 30 * 24 * time.Hour,
	}}
	logger := zap.NewNop()
	jwtSvc := crypto.NewJWTService(cfg)

	// Wire real repositories.
	authRepo := NewMongoRepository(db)
	userRepo := users.NewMongoRepository(db)

	svc := NewService(authRepo, userRepo, jwtSvc, cfg, logger)
	handler := NewHandler(svc)

	// Spin up an in-process gRPC server.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer()
	zee6dov1.RegisterAuthServiceServer(srv, handler)
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := zee6dov1.NewAuthServiceClient(conn)

	// SendOTP
	sendResp, err := client.SendOTP(ctx, &zee6dov1.SendOTPRequest{PhoneNumber: "+15555550100"})
	require.NoError(t, err)
	require.NotEmpty(t, sendResp.OtpId)

	// Look up the plaintext code from the stored record (beta: no SMS).
	rec, err := authRepo.GetOTP(ctx, sendResp.OtpId)
	require.NoError(t, err)
	// In production we wouldn't know the code — here we bypass by seeding
	// a known hash. Re-issue via direct repo write so we know the code.
	knownCode := "424242"
	knownHash, err := crypto.HashOTP(knownCode)
	require.NoError(t, err)
	rec.CodeHash = knownHash
	_, err = db.Collection("otp_records").ReplaceOne(ctx, map[string]any{"_id": rec.ID}, rec)
	require.NoError(t, err)

	// VerifyOTP
	verifyResp, err := client.VerifyOTP(ctx, &zee6dov1.VerifyOTPRequest{
		OtpId:             sendResp.OtpId,
		Code:              knownCode,
		DeviceFingerprint: "integration-device",
	})
	require.NoError(t, err)
	require.NotEmpty(t, verifyResp.AccessToken)
	require.NotEmpty(t, verifyResp.RefreshToken)

	// RefreshToken
	refreshResp, err := client.RefreshToken(ctx, &zee6dov1.RefreshTokenRequest{
		RefreshToken:      verifyResp.RefreshToken,
		DeviceFingerprint: "integration-device",
	})
	require.NoError(t, err)
	require.NotEmpty(t, refreshResp.AccessToken)
	require.NotEmpty(t, refreshResp.RefreshToken)

	// Logout — revokes all sessions for the authenticated user. The
	// integration test skips the middleware so we call service directly.
	require.NoError(t, svc.Logout(ctx, verifyResp.User.Id, ""))
}
