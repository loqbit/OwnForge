package grpcclient

import (
	"google.golang.org/grpc"

	notepb "github.com/ownforge/ownforge/pkg/proto/note"
)

func NewNoteClient(noteServiceAddr string) (notepb.NoteServiceClient, error) {
	conn, err := grpc.NewClient(noteServiceAddr, DefaultDialOptions(noteServiceAddr)...)
	if err != nil {
		return nil, err
	}

	return notepb.NewNoteServiceClient(conn), nil
}
