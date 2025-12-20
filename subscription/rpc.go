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

	pb "github.com/himalayo/clusterfuck/api/subscription/proto"
	"google.golang.org/grpc"
)

var (
	serverPort = flag.Int("grpc_port", 50052, "gRPC server port")
)

type server struct {
	pb.UnimplementedSubscriptionServer
	data *Database
}

func toSubscriptions(subs []SubscriptionData) *pb.Subscriptions {
	var out []*pb.SubscriptionInstance
	for _, sub := range subs {
		out = append(out, sub.toSubscriptionInstance())
	}
	return &pb.Subscriptions{Subscriptions: out}
}

func (s *server) GetSubscriptionsForUser(_ context.Context, user *pb.User) (*pb.Subscriptions, error) {
	s.data.GetSubscriptions(int(user.Id))
	result := <-s.data.Subscriptions
	return toSubscriptions(result), nil
}

func (s *server) UserHasSubscription(_ context.Context, req *pb.SubscriptionRequest) (*pb.HasSubscriptionResponse, error) {
	s.data.hasClub(req)
	return &pb.HasSubscriptionResponse{HasSubscription: <-s.data.HasSubscription}, nil
}

func (s *server) SetActive(_ context.Context, req *pb.ActivationRequest) (*pb.SubscriptionInstance, error) {
	s.data.activationRequest(req)
	sub := <-s.data.Active
	return sub.toSubscriptionInstance(), nil
}

func (s *server) AddDuration(_ context.Context, req *pb.DurationRequest) (*pb.SubscriptionInstance, error) {
	s.data.durationRequest(req)
	sub := <-s.data.DurationAdded
	return sub.toSubscriptionInstance(), nil
}

func (s *server) UserClubComposer(ctx context.Context, req *pb.SubscriptionRequest) (*pb.Packet, error) {
	subscriptions, err := s.data.GetUserSubscriptionsByType(ctx, int(req.UserId), req.SubscriptionType)
	if err != nil {
		return nil, err
	}
	return &pb.Packet{Data: UserClubComposer(ctx, subscriptions, 1)}, nil
}

func StartServer(data *Database) {
	_, portString, _ := strings.Cut(os.Getenv("SUBSCRIPTION_HOST"), ":")
	port, err := strconv.Atoi(portString)
	if err != nil {
		port = *serverPort
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterSubscriptionServer(s, &server{data: data})
	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
