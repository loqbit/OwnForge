package grpcclient

import (
	"google.golang.org/grpc"

	userpb "github.com/ownforge/ownforge/pkg/proto/user"
)

func NewUserClient(userPlatformAddr string) (userpb.UserServiceClient, error) {
	conn, err := grpc.NewClient(userPlatformAddr, DefaultDialOptions(userPlatformAddr)...)
	if err != nil {
		return nil, err
	}

	return userpb.NewUserServiceClient(conn), nil
}
