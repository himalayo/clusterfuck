package main

import (
	"log"
	"flag"
	"context"
	"net"
	"fmt"
	"google.golang.org/grpc"

	pb "github.com/himalayo/clusterfuck/permission/proto"
)

var (
	serverPort = flag.Int("grpc_port", 50054, "gRPC server port")
)

type server struct {
	pb.UnimplementedPermissionsServer
	data *Database
}

func toRankList(ranks []Rank) *pb.RankList {
	elements := make([]*pb.Rank,0)
	for _, rank := range ranks {
		elements = append(elements, rank.toProto())
	}
	return &pb.RankList{Elements: elements}
}

func (s *server) GetAllRanks(_ context.Context, _ *pb.Empty) (*pb.RankList, error) {
	result, err := s.data.GetAllRanks()
	if err != nil {
		return nil, err
	}
	out := toRankList(result)
	return out, nil
}

func (s *server) GetRank(_ context.Context, in *pb.RankId) (*pb.Rank, error) {
	result, err := s.data.GetRank(int(in.Id))
	if err != nil {
		return nil, err
	}
	return result.toProto(), nil
}

func (s *server) GetRankByName(_ context.Context, in *pb.RankName) (*pb.Rank, error) {
	result, err := s.data.GetRankByName(in.Name)
	if err != nil {
		return nil, err
	}
	return result.toProto(), nil
}

func (s *server) CheckRankExists(_ context.Context, in *pb.RankId) (*pb.RankExists, error) {
	result, err := s.data.GetRank(int(in.Id))
	if err != nil {
		return &pb.RankExists{Exists: false}, err
	}
	return &pb.RankExists{Exists: result == nil}, err
}

func (s *server) GetPermission(_ context.Context, in *pb.RankPermission) (*pb.Permission, error) {
	result, err := s.data.GetPermission(int(in.RankId), in.PermissionKey)
	if err != nil {
		return nil, err
	}
	return &pb.Permission{Key: in.PermissionKey, Setting: pb.PermissionSetting(result)}, nil
}

func (s *server) GetRankLevel(_ context.Context, in *pb.RankId) (*pb.Level, error) {
	result, err := s.data.GetRankLevel(int(in.Id))
	if err != nil {
		return nil, err
	}
	return &pb.Level{Level: int32(result)}, nil
}

func StartServer(data *Database) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *serverPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterPermissionsServer(s, &server{data: data})
	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
