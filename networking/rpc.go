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

	listener "github.com/himalayo/clusterfuck/api/networking/listener"
	pb "github.com/himalayo/clusterfuck/api/networking/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	grpc_port = flag.Int("grpc_port", 50051, "The gRPC server port")
	Listeners = make(map[string]listener.Listener)
)

type grpcServer struct {
	pb.UnimplementedNetworkingServer
}

func (s *grpcServer) SendPacket(_ context.Context, p *pb.Packet) (*pb.SuccessMessage, error) {
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

func (s *grpcServer) RegisterIncomingListener(_ context.Context, in *pb.IncomingInstance) (*pb.SuccessMessage, error) {
	lis := listener.NewListener()
	addr := in.Address
	if addr == "" {
		return &pb.SuccessMessage{Successful: false}, nil
	}
	headers := in.GetHeaders()
	if headers == nil {
		return &pb.SuccessMessage{Successful: false}, nil
	}

	for i := range headers {
		registerHandler(int16(headers[i]), func(c *Client, data []byte) {
			if c.sso == "" {
				return
			}
			go func() {
				lis.Send(c.sso, data)
			}()
		})
	}

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return &pb.SuccessMessage{Successful: false}, nil
	}
	go lis.ListenWithConnection(conn)

	return &pb.SuccessMessage{Successful: true}, nil
}

func StartRpc() {
	_, portString, _ := strings.Cut(os.Getenv("NETWORKING_HOST"), ":")
	port, err := strconv.Atoi(portString)
	if err != nil {
		port = *grpc_port
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Failed to start gRPC socket: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterNetworkingServer(s, &grpcServer{})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("gRPC error: %v", err)
	}

}
