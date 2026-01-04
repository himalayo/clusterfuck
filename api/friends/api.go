package api

import (
	"context"
	"log"

	pb "github.com/himalayo/clusterfuck/api/friends/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type FriendsClient struct {
	client pb.FriendsClient
}

func NewClient() *FriendsClient {
	return &FriendsClient{}
}

func (n *FriendsClient) GetItemById(ctx context.Context, id int) (*pb.Friend, error) {
	return n.client.GetFriendById(ctx, &pb.GetFriendByIdRequest{
		FriendId: int32(id),
	})
}

func (n *FriendsClient) GetFriends(ctx context.Context, sso string) (*pb.FriendList, error) {
	return n.client.GetFriendList(ctx, &pb.GetFriendListRequest{
		Session: sso,
	})
}

func (n *FriendsClient) Listen(addr string) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("could not connect to networking module: %v", err)
	}
	n.client = pb.NewFriendsClient(conn)
}
