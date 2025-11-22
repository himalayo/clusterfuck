package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
	pb "github.com/himalayo/clusterfuck/achievements/proto"
)

var (
	serverPort = flag.Int("grpc_port", 50058, "gRPC server port")
)

type server struct {
	pb.UnimplementedAchievementsServer
	data *Database
}

func (s *server) GetAchievementByName(_ context.Context, in *pb.AchievementName) (*pb.Achievement, error) {
	return s.data.GetAchievementByName(in.Name)
}

func (s *server) GetAchievement(_ context.Context, in *pb.AchievementId) (*pb.Achievement, error) {
	return s.data.GetAchievement(int(in.Id))
}

func (s *server) InventoryAchievementsComposer(_ context.Context, _ *pb.Empty) (*pb.Packet, error) {
	achievements, err := s.data.GetInventoryAchievements()
	if err != nil {
		return nil, err
	}
	return &pb.Packet{Data: InventoryAchievementsComposer(achievements)}, nil
}


func StartServer(data *Database) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *serverPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterAchievementsServer(s, &server{data: data})
	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
