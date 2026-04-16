package middleware

import (
	"context"
	"testing"

	zeev1 "github.com/DeepakDP5/zee6do-server/gen/zee6do/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestUnaryValidation_valid_proto_message(t *testing.T) {
	interceptor := UnaryValidation()

	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	// emptypb.Empty is a valid proto message with no validation constraints.
	resp, err := interceptor(context.Background(), &emptypb.Empty{},
		&grpc.UnaryServerInfo{FullMethod: "/test/Valid"}, handler)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestUnaryValidation_non_proto_message(t *testing.T) {
	interceptor := UnaryValidation()

	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	// A non-proto request should pass through without validation.
	resp, err := interceptor(context.Background(), "not-a-proto",
		&grpc.UnaryServerInfo{FullMethod: "/test/NonProto"}, handler)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestUnaryValidation_nil_request(t *testing.T) {
	interceptor := UnaryValidation()

	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	// nil request should pass through.
	resp, err := interceptor(context.Background(), nil,
		&grpc.UnaryServerInfo{FullMethod: "/test/Nil"}, handler)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestUnaryValidation_rejects_invalid_proto_message(t *testing.T) {
	interceptor := UnaryValidation()

	handler := func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called for invalid request")
		return nil, nil
	}

	// SendOTPRequest requires phone_number matching ^\+[1-9]\d{6,14}$
	invalidReq := &zeev1.SendOTPRequest{PhoneNumber: "not-a-phone"}

	_, err := interceptor(context.Background(), invalidReq,
		&grpc.UnaryServerInfo{FullMethod: "/zee6do.v1.AuthService/SendOTP"}, handler)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "validation failed")
}

func TestUnaryValidation_accepts_valid_proto_message_with_constraints(t *testing.T) {
	interceptor := UnaryValidation()

	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	validReq := &zeev1.SendOTPRequest{PhoneNumber: "+12345678901"}

	resp, err := interceptor(context.Background(), validReq,
		&grpc.UnaryServerInfo{FullMethod: "/zee6do.v1.AuthService/SendOTP"}, handler)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestStreamValidation_wraps_stream(t *testing.T) {
	interceptor := StreamValidation()

	var receivedStream grpc.ServerStream
	handler := func(srv any, stream grpc.ServerStream) error {
		receivedStream = stream
		return nil
	}

	ss := &fakeServerStream{ctx: context.Background()}
	err := interceptor(nil, ss, &grpc.StreamServerInfo{FullMethod: "/test/Stream"}, handler)
	require.NoError(t, err)

	// The handler should receive a validatingServerStream.
	_, ok := receivedStream.(*validatingServerStream)
	assert.True(t, ok, "stream should be wrapped with validatingServerStream")
}

func TestMustNewValidator_does_not_panic(t *testing.T) {
	assert.NotPanics(t, func() {
		v := mustNewValidator()
		assert.NotNil(t, v)
	})
}

// fakeRecvServerStream is a fakeServerStream that implements RecvMsg for validation tests.
type fakeRecvServerStream struct {
	grpc.ServerStream
	ctx  context.Context
	msg  proto.Message
	err  error
}

func (f *fakeRecvServerStream) Context() context.Context {
	return f.ctx
}

func (f *fakeRecvServerStream) RecvMsg(m any) error {
	if f.err != nil {
		return f.err
	}
	// Copy the stored message into m if both are proto.Message.
	if f.msg != nil {
		if dst, ok := m.(proto.Message); ok {
			proto.Merge(dst, f.msg)
		}
	}
	return nil
}

func TestValidatingServerStream_RecvMsg_valid(t *testing.T) {
	v := mustNewValidator()
	inner := &fakeRecvServerStream{
		ctx: context.Background(),
		msg: &emptypb.Empty{},
	}

	vs := &validatingServerStream{
		ServerStream: inner,
		validator:    v,
	}

	var received emptypb.Empty
	err := vs.RecvMsg(&received)
	require.NoError(t, err)
}

func TestValidatingServerStream_RecvMsg_inner_error(t *testing.T) {
	v := mustNewValidator()
	inner := &fakeRecvServerStream{
		ctx: context.Background(),
		err: status.Error(codes.Internal, "recv failed"),
	}

	vs := &validatingServerStream{
		ServerStream: inner,
		validator:    v,
	}

	var received emptypb.Empty
	err := vs.RecvMsg(&received)
	require.Error(t, err)

	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}
