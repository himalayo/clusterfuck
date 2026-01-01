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

	pb "github.com/himalayo/clusterfuck/api/player/proto"
	"google.golang.org/grpc"
)

var (
	serverPort = flag.Int("grpc_port", 50053, "gRPC server port")
)

func (u *UserData) toProto() *pb.UserData {
	return &pb.UserData{
		Id:         int32(u.Id),
		Username:   u.Username,
		Look:       u.Look,
		AuthTicket: u.AuthTicket,
		Motto:      u.Motto,
		HomeRoom:   int32(u.HomeRoom),
		Rank:       int32(u.Rank),
	}
}

type server struct {
	pb.UnimplementedPlayerServer
	events *EventListener
}

func (s *server) LoginPlayer(_ context.Context, sso *pb.Ticket) (*pb.LoginStatus, error) {
	return &pb.LoginStatus{Success: s.events.Login(sso)}, nil
}

func (s *server) GetUserData(_ context.Context, sso *pb.Ticket) (*pb.UserData, error) {
	return Data.loadUserData(sso.GetSso()).toProto(), nil
}

func (s *server) GetUserHCData(ctx context.Context, ticket *pb.Ticket) (*pb.UserHCData, error) {
	u, err := Data.GetUserHCData(ctx, ticket.Sso)
	if err != nil {
		return nil, err
	}
	return &pb.UserHCData{
		LastHcPayday:   int32(u.LastHCPayday),
		HcGiftsClaimed: int32(u.HCGiftsClaimed),
	}, nil
}

func StartIncoming(addr string) {
	hostname, portString, _ := strings.Cut(addr, ":")
	port, err := strconv.Atoi(portString)
	if err != nil {
		port = *serverPort
	}
	go Incoming.Serve(port + 500)
	go Incoming.RequestConnection(fmt.Sprintf("%s:%d", hostname, port+500), Net, "player")
}

func StartServer(events *EventListener) {
	addr := os.Getenv("PLAYER_HOST")
	_, portString, _ := strings.Cut(addr, ":")
	port, err := strconv.Atoi(portString)
	if err != nil {
		port = *serverPort
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterPlayerServer(s, &server{events: events})
	go StartIncoming(addr)
	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
