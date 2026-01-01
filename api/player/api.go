package api

import (
	"context"
	"log"
	"strconv"
	"time"

	events "github.com/himalayo/clusterfuck/api/events"
	pb "github.com/himalayo/clusterfuck/api/player/proto"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type PlayerClient struct {
	client       pb.PlayerClient
	loginRequest chan string
	Login        chan string
	redis_sub    *events.RedisSubscriber
}

func NewClient(group string, redis_cfg *redis.Options) (*PlayerClient, error) {
	redis_sub, err := events.NewRedisSubscriber(redis_cfg, "player-events", group)
	if err != nil {
		return nil, err
	}
	return &PlayerClient{Login: make(chan string), loginRequest: make(chan string), redis_sub: redis_sub}, nil
}

func (n *PlayerClient) sendLoginRequest(ticket *pb.Ticket) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ok, err := n.client.LoginPlayer(ctx, ticket)
	if err != nil {
		log.Fatalf("could not login player: %v", err)
	}
	return ok.Success
}

func (n *PlayerClient) sendUserDataRequest(ticket *pb.Ticket) *pb.UserData {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ok, err := n.client.GetUserData(ctx, ticket)
	if err != nil {
		return nil
	}
	return ok
}

func (n *PlayerClient) LoginRequest(sso string) {
	n.loginRequest <- sso
}

func (n *PlayerClient) GetUserData(sso string) *pb.UserData {
	return n.sendUserDataRequest(&pb.Ticket{Sso: sso})
}

func (n *PlayerClient) GetUserHCData(ctx context.Context, sso string) (*pb.UserHCData, error) {
	return n.client.GetUserHCData(ctx, &pb.Ticket{
		Sso: sso,
	})
}

func getRedisMapInt(values map[string]interface{}, key string) int64 {

	out, err := strconv.ParseInt(values[key].(string), 10, 32)
	if err != nil {
		return 0
	}
	return out
}

func GenericEventToLoginEvent(evt events.Event) *pb.LoginEvent {
	eventType := evt.GetType()
	if eventType != "LOGIN" {
		return nil
	}
	values := evt.GetValues()
	return &pb.LoginEvent{
		Id:   evt.GetId(),
		Type: eventType,
		UserData: &pb.UserData{
			Id:         int32(getRedisMapInt(values, "user_id")),
			AuthTicket: values["auth_ticket"].(string),
			Username:   values["username"].(string),
			Look:       values["look"].(string),
			Motto:      values["motto"].(string),
			HomeRoom:   int32(getRedisMapInt(values, "home_room")),
			Rank:       int32(getRedisMapInt(values, "rank")),
		},
	}
}

func (n *PlayerClient) RegisterLoginHandler(handler func(context.Context, *pb.LoginEvent)) {
	n.redis_sub.Subscribe("LOGIN", func(ctx context.Context, evt events.Event) {
		handler(ctx, GenericEventToLoginEvent(evt))
	})
}

func (n *PlayerClient) Listen(addr string) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("could not connect to networking module: %v", err)
	}
	defer conn.Close()
	n.client = pb.NewPlayerClient(conn)
	log.Printf("PlayerClient.Listen: Listening for data with address: %s", addr)

	go n.redis_sub.Listen(context.Background())
	for sso := range n.loginRequest {
		response := n.sendLoginRequest(&pb.Ticket{Sso: sso})
		if response {
			n.Login <- sso
		}
	}
}
