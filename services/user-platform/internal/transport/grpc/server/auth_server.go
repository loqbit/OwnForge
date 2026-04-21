package server

import (
	"context"
	"strings"

	pb "github.com/luckysxx/common/proto/auth"
	authservice "github.com/ownforge/ownforge/services/user-platform/internal/service/auth"
	grpcerrs "github.com/ownforge/ownforge/services/user-platform/internal/transport/grpc/codec/errs"
	"github.com/ownforge/ownforge/services/user-platform/internal/transport/grpc/interceptor"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthServer struct {
	pb.UnimplementedAuthServiceServer
	avc    authservice.AuthService
	logger *zap.Logger
}

// AuthServerDependencies 描述认证 gRPC Server 所需的依赖集合。
type AuthServerDependencies struct {
	AuthService authservice.AuthService
	Logger      *zap.Logger
}

func NewAuthServer(deps AuthServerDependencies) *AuthServer {
	return &AuthServer{avc: deps.AuthService, logger: deps.Logger}
}

func (s *AuthServer) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	if strings.TrimSpace(req.GetUsername()) == "" || strings.TrimSpace(req.GetPassword()) == "" || strings.TrimSpace(req.GetAppCode()) == "" {
		return nil, status.Error(codes.InvalidArgument, "username/password/app_code are required")
	}

	resp, err := s.avc.Login(ctx, &authservice.LoginCommand{
		Username: req.GetUsername(),
		Password: req.GetPassword(),
		AppCode:  req.GetAppCode(),
		DeviceID: req.GetDeviceId(),
	})
	if err != nil {
		return nil, grpcerrs.ToGRPCError(err)
	}

	return &pb.LoginResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		UserId:       resp.UserID,
		Username:     resp.Username,
		SsoToken:     resp.SSOToken,
	}, nil
}

func (s *AuthServer) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	if strings.TrimSpace(req.GetToken()) == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}

	resp, err := s.avc.RefreshToken(ctx, &authservice.RefreshTokenCommand{Token: req.GetToken()})
	if err != nil {
		return nil, grpcerrs.ToGRPCError(err)
	}

	return &pb.RefreshTokenResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	}, nil
}

func (s *AuthServer) ExchangeSSO(ctx context.Context, req *pb.ExchangeSSORequest) (*pb.ExchangeSSOResponse, error) {
	if strings.TrimSpace(req.GetSsoToken()) == "" || strings.TrimSpace(req.GetAppCode()) == "" || strings.TrimSpace(req.GetDeviceId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "sso_token/app_code/device_id are required")
	}

	resp, err := s.avc.ExchangeSSO(ctx, &authservice.ExchangeSSOCommand{
		SSOToken: req.GetSsoToken(),
		AppCode:  req.GetAppCode(),
		DeviceID: req.GetDeviceId(),
	})
	if err != nil {
		return nil, grpcerrs.ToGRPCError(err)
	}

	return &pb.ExchangeSSOResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		UserId:       resp.UserID,
		Username:     resp.Username,
	}, nil
}

func (s *AuthServer) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	if strings.TrimSpace(req.GetAppCode()) == "" || strings.TrimSpace(req.GetDeviceId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "app_code/device_id are required")
	}

	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	err = s.avc.Logout(ctx, &authservice.LogoutCommand{
		UserID:   userID,
		AppCode:  req.GetAppCode(),
		DeviceID: req.GetDeviceId(),
	})
	if err != nil {
		return nil, grpcerrs.ToGRPCError(err)
	}

	return &pb.LogoutResponse{}, nil
}

func (s *AuthServer) VerifyToken(ctx context.Context, req *pb.VerifyTokenRequest) (*pb.VerifyTokenResponse, error) {
	if strings.TrimSpace(req.GetToken()) == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}

	resp, err := s.avc.VerifyToken(ctx, &authservice.VerifyTokenCommand{Token: req.GetToken()})
	if err != nil {
		return nil, grpcerrs.ToGRPCError(err)
	}

	return &pb.VerifyTokenResponse{
		UserId:   resp.UserID,
		Username: resp.Username,
	}, nil
}

func (s *AuthServer) SendPhoneCode(ctx context.Context, req *pb.SendPhoneCodeRequest) (*pb.SendPhoneCodeResponse, error) {
	if strings.TrimSpace(req.GetPhone()) == "" || strings.TrimSpace(req.GetScene()) == "" {
		return nil, status.Error(codes.InvalidArgument, "phone/scene are required")
	}

	resp, err := s.avc.SendPhoneCode(ctx, &authservice.SendPhoneCodeCommand{
		Phone: req.GetPhone(),
		Scene: req.GetScene(),
	})
	if err != nil {
		return nil, grpcerrs.ToGRPCError(err)
	}

	return &pb.SendPhoneCodeResponse{
		Action:          resp.Action,
		CooldownSeconds: int32(resp.CooldownSeconds),
		Message:         resp.Message,
		DebugCode:       resp.DebugCode,
	}, nil
}

func (s *AuthServer) PhoneAuthEntry(ctx context.Context, req *pb.PhoneAuthEntryRequest) (*pb.PhoneAuthEntryResponse, error) {
	if strings.TrimSpace(req.GetPhone()) == "" || strings.TrimSpace(req.GetVerificationCode()) == "" || strings.TrimSpace(req.GetAppCode()) == "" || strings.TrimSpace(req.GetDeviceId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "phone/verification_code/app_code/device_id are required")
	}

	resp, err := s.avc.PhoneAuthEntry(ctx, &authservice.PhoneAuthEntryCommand{
		Phone:            req.GetPhone(),
		VerificationCode: req.GetVerificationCode(),
		AppCode:          req.GetAppCode(),
		DeviceID:         req.GetDeviceId(),
	})
	if err != nil {
		return nil, grpcerrs.ToGRPCError(err)
	}

	return &pb.PhoneAuthEntryResponse{
		Action:          resp.Action,
		AccessToken:     resp.AccessToken,
		RefreshToken:    resp.RefreshToken,
		UserId:          resp.UserID,
		Username:        resp.Username,
		Email:           resp.Email,
		Phone:           resp.Phone,
		ShouldBindEmail: resp.ShouldBindEmail,
		Message:         resp.Message,
		SsoToken:        resp.SSOToken,
	}, nil
}

func (s *AuthServer) PhonePasswordLogin(ctx context.Context, req *pb.PhonePasswordLoginRequest) (*pb.PhonePasswordLoginResponse, error) {
	if strings.TrimSpace(req.GetPhone()) == "" || strings.TrimSpace(req.GetPassword()) == "" || strings.TrimSpace(req.GetAppCode()) == "" || strings.TrimSpace(req.GetDeviceId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "phone/password/app_code/device_id are required")
	}

	resp, err := s.avc.PhonePasswordLogin(ctx, &authservice.PhonePasswordLoginCommand{
		Phone:    req.GetPhone(),
		Password: req.GetPassword(),
		AppCode:  req.GetAppCode(),
		DeviceID: req.GetDeviceId(),
	})
	if err != nil {
		return nil, grpcerrs.ToGRPCError(err)
	}

	return &pb.PhonePasswordLoginResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		UserId:       resp.UserID,
		Username:     resp.Username,
		Phone:        resp.Phone,
		Message:      resp.Message,
		SsoToken:     resp.SSOToken,
	}, nil
}
