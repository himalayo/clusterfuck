package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"context"

	"google.golang.org/grpc"
	pb "github.com/himalayo/clusterfuck/networking/proto"
)

var (
	grpc_port = flag.Int("grpc_port", 50051, "The gRPC server port")
)

type grpcServer struct {
	pb.UnimplementedNetworkingServer
}

func (s *grpcServer) SendPacket(_ context.Context, p *pb.Packet) (*pb.SuccessMessage, error)  {
	client := Man.sessions[p.ClientId]
	log.Printf("SendPacket: %s", p)
	if client == nil {
		return &pb.SuccessMessage{Successful: false}, nil
	}
	client.send <- p.Packet
	return &pb.SuccessMessage{Successful: true}, nil
}


func (s *grpcServer) SendPackets(_ context.Context, p *pb.Packets) (*pb.SuccessMessage, error) {
	client := Man.sessions[p.ClientId]
	if client == nil {
		return &pb.SuccessMessage{Successful: false}, nil
	}
	for _, packet := range p.Packets {
		client.send <- packet
	}
	return &pb.SuccessMessage{Successful: true}, nil
}

func StartRpc() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *grpc_port))
	if err != nil {
		log.Fatalf("Failed to start gRPC socket: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterNetworkingServer(s, &grpcServer{})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("gRPC error: %v", err)
	}

}
