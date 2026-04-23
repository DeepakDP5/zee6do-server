package auth

import (
	"context"

	zee6dov1 "github.com/DeepakDP5/zee6do-server/gen/zee6do/v1"
	apperrors "github.com/DeepakDP5/zee6do-server/pkg/errors"
	"github.com/DeepakDP5/zee6do-server/pkg/middleware"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Handler implements zee6dov1.AuthServiceServer by delegating to Service and
// translating domain types to proto messages.
type Handler struct {
	zee6dov1.UnimplementedAuthServiceServer
	svc *Service
}

// NewHandler creates a new gRPC handler for the auth service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// SendOTP handles the SendOTP RPC.
func (h *Handler) SendOTP(ctx context.Context, req *zee6dov1.SendOTPRequest) (*zee6dov1.SendOTPResponse, error) {
	res, err := h.svc.SendOTP(ctx, req.GetPhoneNumber())
	if err != nil {
		return nil, apperrors.ToGRPCError(err)
	}
	return &zee6dov1.SendOTPResponse{
		OtpId:     res.OTPID,
		ExpiresAt: timestamppb.New(res.ExpiresAt),
	}, nil
}

// VerifyOTP handles the VerifyOTP RPC.
func (h *Handler) VerifyOTP(ctx context.Context, req *zee6dov1.VerifyOTPRequest) (*zee6dov1.VerifyOTPResponse, error) {
	pair, err := h.svc.VerifyOTP(ctx, req.GetOtpId(), req.GetCode(), req.GetDeviceFingerprint())
	if err != nil {
		return nil, apperrors.ToGRPCError(err)
	}
	return &zee6dov1.VerifyOTPResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		User:         toProtoUser(pair),
	}, nil
}

// SocialLogin handles the SocialLogin RPC.
func (h *Handler) SocialLogin(ctx context.Context, req *zee6dov1.SocialLoginRequest) (*zee6dov1.SocialLoginResponse, error) {
	pair, err := h.svc.SocialLogin(ctx, req.GetProvider().String(), req.GetIdToken(), req.GetDeviceFingerprint())
	if err != nil {
		return nil, apperrors.ToGRPCError(err)
	}
	return &zee6dov1.SocialLoginResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		User:         toProtoUser(pair),
	}, nil
}

// RefreshToken handles the RefreshToken RPC.
func (h *Handler) RefreshToken(ctx context.Context, req *zee6dov1.RefreshTokenRequest) (*zee6dov1.RefreshTokenResponse, error) {
	pair, err := h.svc.RefreshToken(ctx, req.GetRefreshToken(), req.GetDeviceFingerprint())
	if err != nil {
		return nil, apperrors.ToGRPCError(err)
	}
	return &zee6dov1.RefreshTokenResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
	}, nil
}

// ListDevices handles the ListDevices RPC.
func (h *Handler) ListDevices(ctx context.Context, _ *zee6dov1.ListDevicesRequest) (*zee6dov1.ListDevicesResponse, error) {
	userID := middleware.UserIDFromContext(ctx)
	sessions, err := h.svc.ListDevices(ctx, userID)
	if err != nil {
		return nil, apperrors.ToGRPCError(err)
	}
	devices := make([]*zee6dov1.Device, 0, len(sessions))
	for _, s := range sessions {
		devices = append(devices, &zee6dov1.Device{
			Id:           s.ID.Hex(),
			Name:         s.DeviceID,
			CreatedAt:    timestamppb.New(s.CreatedAt),
			LastActiveAt: timestamppb.New(s.CreatedAt),
			IsCurrent:    false,
		})
	}
	return &zee6dov1.ListDevicesResponse{Devices: devices}, nil
}

// RevokeDevice handles the RevokeDevice RPC.
func (h *Handler) RevokeDevice(ctx context.Context, req *zee6dov1.RevokeDeviceRequest) (*zee6dov1.RevokeDeviceResponse, error) {
	userID := middleware.UserIDFromContext(ctx)
	if err := h.svc.RevokeDevice(ctx, userID, req.GetDeviceId()); err != nil {
		return nil, apperrors.ToGRPCError(err)
	}
	return &zee6dov1.RevokeDeviceResponse{}, nil
}

// Logout handles the Logout RPC. Beta: revokes all sessions for the caller
// since the request carries no session identifier.
func (h *Handler) Logout(ctx context.Context, _ *zee6dov1.LogoutRequest) (*zee6dov1.LogoutResponse, error) {
	userID := middleware.UserIDFromContext(ctx)
	if err := h.svc.Logout(ctx, userID, ""); err != nil {
		return nil, apperrors.ToGRPCError(err)
	}
	return &zee6dov1.LogoutResponse{}, nil
}

// toProtoUser maps a TokenPair's user to the proto User message.
func toProtoUser(pair *TokenPair) *zee6dov1.User {
	if pair == nil || pair.User == nil {
		return nil
	}
	u := pair.User
	return &zee6dov1.User{
		Id:        u.ID.Hex(),
		Phone:     u.Phone,
		Email:     u.Email,
		Name:      u.Name,
		CreatedAt: timestamppb.New(u.CreatedAt),
	}
}
