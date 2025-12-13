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
	"sync"

	listener "github.com/himalayo/clusterfuck/api/networking/listener"
	pb "github.com/himalayo/clusterfuck/api/networking/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type application struct {
	addresses []string
	headers   []int32
	mu        sync.Mutex
}

var (
	grpc_port            = flag.Int("grpc_port", 50051, "The gRPC server port")
	Listeners            = make(map[string]*listener.Listener)
	Applications         = make(map[string]*application)
	ApplicationsHeaders  = make(map[int16][]*application)
	AddressToApplication = make(map[string]*application)
)

type grpcServer struct {
	pb.UnimplementedNetworkingServer
}

func (s *grpcServer) SendPacket(_ context.Context, p *pb.Packet) (*pb.SuccessMessage, error) {
	client := Man.sessions[p.ClientId]
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

	Listeners[addr] = lis

	if in.Application == "" {
		for i := range headers {
			RegisterHandler(int16(headers[i]), func(c *Client, data []byte, packet []byte) {
				if c.sso == "" {
					return
				}
				go func() {
					lis.Send(c.sso, packet)
				}()
			})
		}
	} else {
		_, ok := Applications[in.Application]
		if !ok {
			app := &application{
				addresses: []string{addr},
				headers:   headers,
			}
			Applications[in.Application] = app
			AddressToApplication[addr] = app
			for i := range headers {
				handler_id := RegisterHandler(int16(headers[i]), func(c *Client, data []byte, packet []byte) {
					if c.sso == "" {
						return
					}
					log.Printf("%d: Sending to application: %s", headers[i], in.Application)
					go func() {
						lis.Send(c.sso, packet)
					}()
				})
				lis.HandlerIds = append(lis.HandlerIds, handler_id)
				ApplicationsHeaders[int16(headers[i])] = append(ApplicationsHeaders[int16(headers[i])], app)
			}
		} else {
			return &pb.SuccessMessage{Successful: true}, nil
		}
		ResolverManager.AddAddress(addr, in.Application)
		addr = fmt.Sprintf("clusterfuck:///%s", in.Application)
		conn, err := grpc.NewClient(
			addr,
			grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return &pb.SuccessMessage{Successful: false}, nil
		}
		go lis.ListenWithConnection(conn)
		log.Printf("Successfully established connection with: %s (application: %s) for headers: %v", addr, in.Application, headers)

		return &pb.SuccessMessage{Successful: true}, nil
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return &pb.SuccessMessage{Successful: false}, nil
	}
	go lis.ListenWithConnection(conn)

	log.Printf("Successfully established connection with: %s for headers: %v", addr, headers)

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
