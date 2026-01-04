package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	pb "github.com/himalayo/clusterfuck/api/friends/proto"
	"google.golang.org/grpc"
)

var (
	grpc_port = flag.Int("grpc_port", 50063, "The gRPC server port")
)

type server struct {
	pb.UnimplementedFriendsServer
	data *Database
}

func (f *Friend) ToProto() *pb.Friend {
	if f == nil {
		return nil
	}
	gender := pb.Gender_M
	if f.Gender == "F" {
		gender = pb.Gender_F
	}
	return &pb.Friend{
		Id:         int32(f.Id),
		Username:   f.Username,
		Gender:     gender,
		Look:       f.Look,
		Motto:      f.Motto,
		Relation:   int32(f.Relation),
		CategoryId: int32(f.CategoryId),
		UserId:     int32(f.UserId),
	}
}

func friendResultToProto(f *Friend, err error) (*pb.Friend, error) {
	return f.ToProto(), err
}

func friendListResultToProto(fs []*Friend, err error) (*pb.FriendList, error) {
	friends := make([]*pb.Friend, len(fs))
	for i, friend := range fs {
		friends[i] = friend.ToProto()
	}
	return &pb.FriendList{
		Elements: friends,
	}, err
}

func (s *server) GetFriendById(ctx context.Context, in *pb.GetFriendByIdRequest) (*pb.Friend, error) {
	return friendResultToProto(s.data.GetFriendById(ctx, int(in.GetFriendId())))
}

func (s *server) GetFriendList(ctx context.Context, in *pb.GetFriendListRequest) (*pb.FriendList, error) {
	return friendListResultToProto(s.data.GetFriendsForUser(ctx, in.GetSession()))
}

func StartServer(data *Database) {
	_, portString, _ := strings.Cut(os.Getenv("FRIENDS_HOST"), ":")
	port, err := strconv.Atoi(portString)
	if err != nil {
		port = *grpc_port
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Could not start gRPC socket: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterFriendsServer(s, &server{data: data})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("gRPC error: %v", err)
	}
}
