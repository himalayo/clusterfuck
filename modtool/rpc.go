package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	pb "github.com/himalayo/clusterfuck/api/modtool/proto"
	"google.golang.org/grpc"
)

var (
	serverPort = flag.Int("grpc_port", 50056, "gRPC server port")
)

type server struct {
	pb.UnimplementedModtoolServer
	data *Database
}

func (s *server) GetCfhTopic(_ context.Context, in *pb.TopicId) (*pb.CfhTopic, error) {
	return s.data.GetCfhTopic(int(in.Id))
}

func (s *server) CfhTopicsMessageComposer(_ context.Context, _ *pb.Empty) (*pb.Packet, error) {
	categories, err := s.data.GetPartialCategories()
	if err != nil {
		return nil, err
	}
	packet := CfhTopicsComposer(categories)
	return &pb.Packet{Data: packet}, nil
}

func StartServer(data *Database) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *serverPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterModtoolServer(s, &server{data: data})
	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
