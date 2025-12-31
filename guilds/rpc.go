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

	pb "github.com/himalayo/clusterfuck/api/guilds/proto"
	"google.golang.org/grpc"
)

var (
	grpc_port = flag.Int("grpc_port", 50060, "The gRPC server port")
)

type server struct {
	pb.UnimplementedGuildsServer
	data *Database
}

func partsResultToProto(parts []GuildPart, err error) (*pb.GuildParts, error) {
	if err != nil {
		return nil, err
	}
	protoParts := make([]*pb.GuildPart, len(parts))
	for i, part := range parts {
		protoParts[i] = part.ToProto()
	}
	return &pb.GuildParts{Parts: protoParts}, nil
}

func (s *server) GetBases(ctx context.Context, _ *pb.Empty) (*pb.GuildParts, error) {
	return partsResultToProto(data.LoadGuildPartsByType(ctx, "base"))
}

func (s *server) GetSymbols(ctx context.Context, _ *pb.Empty) (*pb.GuildParts, error) {
	return partsResultToProto(data.LoadGuildPartsByType(ctx, "symbol"))
}

func (s *server) GetBaseColors(ctx context.Context, _ *pb.Empty) (*pb.GuildParts, error) {
	return partsResultToProto(data.LoadGuildPartsByType(ctx, "base_color"))
}

func (s *server) GetSymbolColors(ctx context.Context, _ *pb.Empty) (*pb.GuildParts, error) {
	return partsResultToProto(data.LoadGuildPartsByType(ctx, "symbol_color"))
}

func (s *server) GetBackgroundColors(ctx context.Context, _ *pb.Empty) (*pb.GuildParts, error) {
	return partsResultToProto(data.LoadGuildPartsByType(ctx, "background_color"))
}

func StartServer(data *Database) {
	_, portString, _ := strings.Cut(os.Getenv("GUILDS_HOST"), ":")
	port, err := strconv.Atoi(portString)
	if err != nil {
		port = *grpc_port
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Could not start gRPC socket: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterGuildsServer(s, &server{data: data})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("gRPC error: %v", err)
	}
}
