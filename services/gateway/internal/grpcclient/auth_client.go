package grpcclient

import (
	"google.golang.org/grpc"

	authpb "github.com/loqbit/ownforge/pkg/proto/auth"
)

func NewAuthClient(userPlatformAddr string) (authpb.AuthServiceClient, error) {
	conn, err := grpc.NewClient(userPlatformAddr, DefaultDialOptions(userPlatformAddr)...)
	if err != nil {
		return nil, err
	}

	return authpb.NewAuthServiceClient(conn), nil
}
