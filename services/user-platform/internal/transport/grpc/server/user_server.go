package server

import (
	"context"
	"strings"

	grpcerrs "github.com/ownforge/ownforge/services/user-platform/internal/transport/grpc/codec/errs"
	grpcinterceptor "github.com/ownforge/ownforge/services/user-platform/internal/transport/grpc/interceptor"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/luckysxx/common/proto/user"
	accountservice "github.com/ownforge/ownforge/services/user-platform/internal/service/account"

	"go.uber.org/zap"
)

// UserServer 是用户服务的 gRPC 接口实现。
type UserServer struct {
	pb.UnimplementedUserServiceServer
	svc        accountservice.UserService
	profileSvc accountservice.ProfileService
	logger     *zap.Logger
}

// UserServerDependencies 描述用户 gRPC Server 所需的依赖集合。
type UserServerDependencies struct {
	UserService    accountservice.UserService
	ProfileService accountservice.ProfileService
	Logger         *zap.Logger
}

// NewUserServer 创建一个用户服务 gRPC Server。
func NewUserServer(deps UserServerDependencies) *UserServer {
	return &UserServer{svc: deps.UserService, profileSvc: deps.ProfileService, logger: deps.Logger}
}

// Register 处理用户注册的 gRPC 请求。
func (s *UserServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	if strings.TrimSpace(req.GetPhone()) == "" || strings.TrimSpace(req.GetEmail()) == "" || strings.TrimSpace(req.GetUsername()) == "" || strings.TrimSpace(req.GetPassword()) == "" {
		return nil, status.Error(codes.InvalidArgument, "phone/email/username/password are required")
	}

	resp, err := s.svc.Register(ctx, &accountservice.RegisterCommand{
		Phone:    req.GetPhone(),
		Email:    req.GetEmail(),
		Username: req.GetUsername(),
		Password: req.GetPassword(),
	})
	if err != nil {
		return nil, grpcerrs.ToGRPCError(err)
	}

	return &pb.RegisterResponse{
		UserId:   resp.UserID,
		Username: resp.Username,
		Phone:    resp.Phone,
	}, nil
}

// ChangePassword 处理修改密码的 gRPC 请求。
func (s *UserServer) ChangePassword(ctx context.Context, req *pb.ChangePasswordRequest) (*pb.ChangePasswordResponse, error) {
	userID, err := grpcinterceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.GetOldPassword()) == "" || strings.TrimSpace(req.GetNewPassword()) == "" {
		return nil, status.Error(codes.InvalidArgument, "old_password/new_password are required")
	}

	resp, err := s.svc.ChangePassword(ctx, &accountservice.ChangePasswordCommand{
		UserID:      userID,
		OldPassword: req.GetOldPassword(),
		NewPassword: req.GetNewPassword(),
	})
	if err != nil {
		return nil, grpcerrs.ToGRPCError(err)
	}

	return &pb.ChangePasswordResponse{
		UserId:  resp.UserID,
		Message: resp.Message,
	}, nil
}

// LogoutAllSessions 处理退出全部设备的 gRPC 请求。
func (s *UserServer) LogoutAllSessions(ctx context.Context, _ *pb.LogoutAllSessionsRequest) (*pb.LogoutAllSessionsResponse, error) {
	userID, err := grpcinterceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := s.svc.LogoutAllSessions(ctx, &accountservice.LogoutAllSessionsCommand{
		UserID: userID,
	})
	if err != nil {
		return nil, grpcerrs.ToGRPCError(err)
	}

	return &pb.LogoutAllSessionsResponse{
		UserId:  resp.UserID,
		Message: resp.Message,
	}, nil
}

// BindEmail 处理绑定邮箱的 gRPC 请求。
func (s *UserServer) BindEmail(ctx context.Context, req *pb.BindEmailRequest) (*pb.BindEmailResponse, error) {
	userID, err := grpcinterceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.GetEmail()) == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	resp, err := s.svc.BindEmail(ctx, &accountservice.BindEmailCommand{
		UserID: userID,
		Email:  req.GetEmail(),
	})
	if err != nil {
		return nil, grpcerrs.ToGRPCError(err)
	}

	return &pb.BindEmailResponse{
		UserId:  resp.UserID,
		Email:   resp.Email,
		Message: resp.Message,
	}, nil
}

// SetPassword 处理设置密码的 gRPC 请求。
func (s *UserServer) SetPassword(ctx context.Context, req *pb.SetPasswordRequest) (*pb.SetPasswordResponse, error) {
	userID, err := grpcinterceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.GetNewPassword()) == "" {
		return nil, status.Error(codes.InvalidArgument, "new_password is required")
	}

	resp, err := s.svc.SetPassword(ctx, &accountservice.SetPasswordCommand{
		UserID:      userID,
		NewPassword: req.GetNewPassword(),
	})
	if err != nil {
		return nil, grpcerrs.ToGRPCError(err)
	}

	return &pb.SetPasswordResponse{
		UserId:  resp.UserID,
		Message: resp.Message,
	}, nil
}

// GetProfile 处理获取用户资料的 gRPC 请求。
func (s *UserServer) GetProfile(ctx context.Context, req *pb.GetProfileRequest) (*pb.ProfileResponse, error) {
	if req.GetUserId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	resp, err := s.profileSvc.GetProfile(ctx, &accountservice.GetProfileQuery{
		UserID: req.GetUserId(),
	})
	if err != nil {
		return nil, grpcerrs.ToGRPCError(err)
	}

	return &pb.ProfileResponse{
		UserId:    resp.UserID,
		Nickname:  resp.Nickname,
		AvatarUrl: resp.AvatarURL,
		Bio:       resp.Bio,
		Birthday:  resp.Birthday,
		UpdatedAt: resp.UpdatedAt,
	}, nil
}

// UpdateProfile 处理更新用户资料的 gRPC 请求。
func (s *UserServer) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.ProfileResponse, error) {
	if req.GetUserId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	resp, err := s.profileSvc.UpdateProfile(ctx, &accountservice.UpdateProfileCommand{
		UserID:    req.GetUserId(),
		Nickname:  req.GetNickname(),
		AvatarURL: req.GetAvatarUrl(),
		Bio:       req.GetBio(),
		Birthday:  req.GetBirthday(),
	})
	if err != nil {
		return nil, grpcerrs.ToGRPCError(err)
	}

	return &pb.ProfileResponse{
		UserId:    resp.UserID,
		Nickname:  resp.Nickname,
		AvatarUrl: resp.AvatarURL,
		Bio:       resp.Bio,
		Birthday:  resp.Birthday,
		UpdatedAt: resp.UpdatedAt,
	}, nil
}
