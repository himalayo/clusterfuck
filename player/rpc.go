package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	pb "github.com/himalayo/clusterfuck/api/player/proto"
	"google.golang.org/grpc"
)

var (
	serverPort = flag.Int("grpc_port", 50053, "gRPC server port")
)

type server struct {
	pb.UnimplementedPlayerServer
	events *EventListener
}

func (s *server) LoginPlayer(_ context.Context, sso *pb.Ticket) (*pb.LoginStatus, error) {
	return &pb.LoginStatus{Success: s.events.Login(sso)}, nil
}

func StartServer(events *EventListener) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *serverPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterPlayerServer(s, &server{events: events})
	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
