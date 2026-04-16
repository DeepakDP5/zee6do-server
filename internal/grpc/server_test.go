package grpc

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/DeepakDP5/zee6do-server/pkg/config"
	"github.com/DeepakDP5/zee6do-server/pkg/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// testValidator implements middleware.JWTValidator for integration tests.
type testValidator struct{}

func (t *testValidator) ValidateToken(_ context.Context, token string) (string, error) {
	if token == "valid-token" {
		return "test-user-id", nil
	}
	return "", fmt.Errorf("invalid token")
}

func newTestServer(t *testing.T, port int) (*Server, *observer.ObservedLogs) {
	t.Helper()

	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	cfg := &config.Config{
		Server: config.ServerConfig{
			GRPCPort:    port,
			Environment: "development",
		},
	}

	authCfg := middleware.AuthConfig{
		Validator: &testValidator{},
		SkipMethods: map[string]bool{
			// Health check is unauthenticated.
			"/grpc.health.v1.Health/Check": true,
		},
	}

	srv := NewServer(cfg, logger, authCfg)

	// Register the gRPC health service so we have an RPC to call.
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(srv.GRPCServer(), healthServer)

	return srv, logs
}

func getFreePort(t *testing.T) int {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0") //nolint:gosec // test-only, localhost binding is fine
	require.NoError(t, err)
	port := lis.Addr().(*net.TCPAddr).Port
	lis.Close()
	return port
}

func TestNewServer_creates_server(t *testing.T) {
	srv, _ := newTestServer(t, getFreePort(t))
	assert.NotNil(t, srv)
	assert.NotNil(t, srv.GRPCServer())
}

func TestServer_start_and_graceful_stop(t *testing.T) {
	port := getFreePort(t)
	srv, _ := newTestServer(t, port)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	// Wait for server to be ready.
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 500*time.Millisecond)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}, 3*time.Second, 50*time.Millisecond, "server should start listening")

	// Graceful stop.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := srv.Close(ctx)
	assert.NoError(t, err)

	// Server.Start should return without error after graceful stop.
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop in time")
	}
}

func TestServer_interceptor_chain_unauthenticated_rpc(t *testing.T) {
	port := getFreePort(t)
	srv, logs := newTestServer(t, port)

	go srv.Start() //nolint:errcheck
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Close(ctx) //nolint:errcheck
	})

	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 500*time.Millisecond)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}, 3*time.Second, 50*time.Millisecond)

	// Connect and call the health check (which is in the skip list).
	conn, err := grpc.NewClient(
		fmt.Sprintf("localhost:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	client := healthpb.NewHealthClient(conn)
	resp, err := client.Check(context.Background(), &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	assert.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.Status)

	// Verify logging interceptor fired — should see "request completed".
	completionLogs := logs.FilterMessage("request completed")
	assert.GreaterOrEqual(t, completionLogs.Len(), 1, "logging interceptor should have logged completion")
}

func TestServer_interceptor_chain_authenticated_rpc(t *testing.T) {
	port := getFreePort(t)

	// Create server with no skipped methods so health check requires auth.
	core, _ := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	cfg := &config.Config{
		Server: config.ServerConfig{
			GRPCPort:    port,
			Environment: "development",
		},
	}

	authCfg := middleware.AuthConfig{
		Validator:   &testValidator{},
		SkipMethods: map[string]bool{},
	}

	srv := NewServer(cfg, logger, authCfg)
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(srv.GRPCServer(), healthServer)

	go srv.Start() //nolint:errcheck
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Close(ctx) //nolint:errcheck
	})

	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 500*time.Millisecond)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}, 3*time.Second, 50*time.Millisecond)

	conn, err := grpc.NewClient(
		fmt.Sprintf("localhost:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	client := healthpb.NewHealthClient(conn)

	// Without auth token — should get Unauthenticated.
	_, err = client.Check(context.Background(), &healthpb.HealthCheckRequest{})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())

	// With valid auth token — should succeed.
	md := metadata.Pairs("authorization", "Bearer valid-token")
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	resp, err := client.Check(ctx, &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	assert.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.Status)

	// With invalid auth token — should get Unauthenticated.
	md = metadata.Pairs("authorization", "Bearer invalid-token")
	ctx = metadata.NewOutgoingContext(context.Background(), md)

	_, err = client.Check(ctx, &healthpb.HealthCheckRequest{})
	require.Error(t, err)
	st, _ = status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestServer_Close_timeout_forces_stop(t *testing.T) {
	port := getFreePort(t)
	srv, _ := newTestServer(t, port)

	go srv.Start() //nolint:errcheck

	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 500*time.Millisecond)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}, 3*time.Second, 50*time.Millisecond)

	// Close with already-cancelled context to force immediate stop.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Already expired.

	err := srv.Close(ctx)
	assert.NoError(t, err)
}
