package middleware

import (
	"context"

	"buf.build/go/protovalidate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// UnaryValidation returns a unary server interceptor that validates incoming
// request messages using buf protovalidate annotations defined in proto files.
// Invalid requests are rejected with codes.InvalidArgument before hitting the handler.
func UnaryValidation() grpc.UnaryServerInterceptor {
	validator := mustNewValidator()

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if msg, ok := req.(proto.Message); ok {
			if err := validator.Validate(msg); err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "validation failed: %s", err.Error())
			}
		}
		return handler(ctx, req)
	}
}

// StreamValidation returns a stream server interceptor. For server-streaming
// RPCs, the initial request is not intercepted at this level (it's the handler's
// responsibility to validate). This interceptor is provided for completeness
// with client-streaming or bidirectional RPCs where RecvMsg validation is needed.
func StreamValidation() grpc.StreamServerInterceptor {
	validator := mustNewValidator()

	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		return handler(srv, &validatingServerStream{ServerStream: ss, validator: validator})
	}
}

// validatingServerStream wraps grpc.ServerStream to validate received messages.
type validatingServerStream struct {
	grpc.ServerStream
	validator protovalidate.Validator
}

func (v *validatingServerStream) RecvMsg(m any) error {
	if err := v.ServerStream.RecvMsg(m); err != nil {
		return err
	}
	if msg, ok := m.(proto.Message); ok {
		if err := v.validator.Validate(msg); err != nil {
			return status.Errorf(codes.InvalidArgument, "validation failed: %s", err.Error())
		}
	}
	return nil
}

func mustNewValidator() protovalidate.Validator {
	v, err := protovalidate.New()
	if err != nil {
		panic("middleware: failed to create protovalidate validator: " + err.Error())
	}
	return v
}
