package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	"database/sql"

	pb "github.com/himalayo/clusterfuck/api/configuration/proto"
	"google.golang.org/grpc"
)

var (
	grpc_port = flag.Int("grpc_port", 50057, "The gRPC server port")
)

type server struct {
	pb.UnimplementedConfigurationServer
	data *Database
}

func unpackBoolResult(def *pb.ConfigurationResult) *pb.BooleanConfiguration {
	if def == nil {
		return nil
	}
	return def.GetBooleanConfig()
}

func unpackIntResult(def *pb.ConfigurationResult) *pb.IntConfiguration {
	if def == nil {
		return nil
	}
	return def.GetIntConfig()
}

func unpackStringResult(def *pb.ConfigurationResult) *pb.StringConfiguration {
	if def == nil {
		return nil
	}
	return def.GetStringConfig()
}

func unpackDoubleResult(def *pb.ConfigurationResult) *pb.DoubleConfiguration {
	if def == nil {
		return nil
	}
	return def.GetDoubleConfig()
}

func (s *server) GetBoolean(_ context.Context, in *pb.ConfigurationRequest) (*pb.BooleanConfiguration, error) {
	key := in.GetKey()
	def := unpackBoolResult(in.GetDefaultResult())
	config, err := s.data.GetBoolean(key)
	if err != nil {
		if err == sql.ErrNoRows {
			return def, nil
		}
		return nil, err
	}
	return &pb.BooleanConfiguration{Config: config}, nil
}

func (s *server) GetInt(_ context.Context, in *pb.ConfigurationRequest) (*pb.IntConfiguration, error) {
	key := in.GetKey()
	def := unpackIntResult(in.GetDefaultResult())
	config, err := s.data.GetInt(key)
	if err != nil {
		if err == sql.ErrNoRows {
			return def, nil
		}
		return nil, err
	}
	return &pb.IntConfiguration{Config: int32(config)}, nil
}

func (s *server) GetDouble(_ context.Context, in *pb.ConfigurationRequest) (*pb.DoubleConfiguration, error) {
	key := in.GetKey()
	def := unpackDoubleResult(in.GetDefaultResult())
	config, err := s.data.GetDouble(key)
	if err != nil {
		if err == sql.ErrNoRows {
			return def, nil
		}
		return nil, err
	}
	return &pb.DoubleConfiguration{Config: config}, nil
}

func (s *server) GetString(_ context.Context, in *pb.ConfigurationRequest) (*pb.StringConfiguration, error) {
	key := in.GetKey()
	def := unpackStringResult(in.GetDefaultResult())
	config, err := s.data.GetString(key)
	if err != nil {
		if err == sql.ErrNoRows {
			return def, nil
		}
		return nil, err
	}
	return &pb.StringConfiguration{Config: config}, nil
}

func StartServer(data *Database) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *grpc_port))
	if err != nil {
		log.Fatalf("Could not start gRPC socket: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterConfigurationServer(s, &server{data: data})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("gRPC error: %v", err)
	}
}
