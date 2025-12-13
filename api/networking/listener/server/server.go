package api

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"

	api "github.com/himalayo/clusterfuck/api/networking"
	pb "github.com/himalayo/clusterfuck/api/networking/proto"
	"google.golang.org/grpc"
)

func getPacketHeader(data []byte) int16 {
	header := int16(binary.BigEndian.Uint16(data[4:6]))
	return header
}

type IncomingServer struct {
	pb.UnimplementedIncomingListenerServer
	handlers map[int16][]func(string, []byte)
}

func NewIncomingServer() *IncomingServer {
	return &IncomingServer{
		handlers: make(map[int16][]func(string, []byte)),
	}
}

func (s *IncomingServer) RequestConnection(addr string, n *api.NetworkingClient, app string) {
	headers := make([]int, 0, len(s.handlers))
	for header := range s.handlers {
		headers = append(headers, int(header))
	}
	log.Printf("Requesting incoming connection as: %s for headers: %v", addr, headers)
	n.ConnectHandler(addr, headers, app)
}

func (s *IncomingServer) ReceivePacket(_ context.Context, p *pb.Packet) (*pb.SuccessMessage, error) {
	header := getPacketHeader(p.Packet)
	handlers, ok := s.handlers[header]
	if !ok {
		return &pb.SuccessMessage{Successful: false}, nil
	}

	for i := range handlers {
		go handlers[i](p.ClientId, p.Packet)
	}

	return &pb.SuccessMessage{Successful: true}, nil
}

func (s *IncomingServer) RegisterHandler(header int16, handler func(string, []byte)) {
	s.handlers[header] = append(s.handlers[header], handler)
}

func (s *IncomingServer) Serve(port int) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Failed to start gRPC socket: %v", err)
	}
	sv := grpc.NewServer()
	pb.RegisterIncomingListenerServer(sv, s)
	if err := sv.Serve(lis); err != nil {
		log.Fatalf("gRPC error: %v", err)
	}
}
